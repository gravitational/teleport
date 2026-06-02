// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

use crate::CGOErrCode;
use ironrdp_core::{encode_vec, Encode, WriteCursor};
use ironrdp_pdu::fast_path::{
    EncryptionFlags, FastPathHeader, FastPathUpdate, FastPathUpdatePdu, Fragmentation, UpdateCode,
};
use ironrdp_pdu::geometry::ExclusiveRectangle;
use ironrdp_pdu::pointer::{
    ColorPointerAttribute, Point16, PointerAttribute, PointerUpdateData,
};
use ironrdp_pdu::surface_commands::{ExtendedBitmapDataPdu, SurfaceBitsPdu, SurfaceCommand};
use std::fmt::Display;
use std::{ptr, slice};

// EncodingResult is structure used to pass encoded frames from Rust to Go
#[repr(C)]
pub struct EncodingResult {
    pub length: usize,
    pub pdus: *mut Pdu,
    pub error_msg: *mut u8,
    pub error_code: CGOErrCode,
}

// Pdu represents single encoded FastPath PDU
#[repr(C)]
pub struct Pdu {
    pub length: usize,
    pub data: *mut u8,
}

#[no_mangle]
pub unsafe extern "C" fn encode_qoiz(
    data: *mut u8,
    x: u16,
    y: u16,
    width: u16,
    height: u16,
) -> EncodingResult {
    let w = width as usize;
    let h = height as usize;
    let input = slice::from_raw_parts(data, w * h * 4);

    match inner_encode_qoiz(input, x, y, width, height) {
        Ok(data) => {
            let frames: Vec<_> = data
                .iter()
                .map(|x| Pdu {
                    length: x.len(),
                    data: Box::into_raw(x.clone().into_boxed_slice()) as _,
                })
                .collect();
            EncodingResult {
                length: frames.len(),
                pdus: Box::into_raw(frames.into_boxed_slice()) as _,
                error_code: CGOErrCode::ErrCodeSuccess,
                error_msg: ptr::null_mut(),
            }
        }
        Err(e) => {
            let msg = format!("{}", e);
            let b = msg.into_bytes().into_boxed_slice();
            EncodingResult {
                length: b.len(),
                pdus: ptr::null_mut(),
                error_msg: Box::into_raw(b) as _,
                error_code: CGOErrCode::ErrCodeFailure,
            }
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn free_encoding_result(frames: EncodingResult) {
    if frames.error_code == CGOErrCode::ErrCodeFailure {
        drop(Box::from_raw(ptr::slice_from_raw_parts_mut(
            frames.error_msg,
            frames.length,
        )));
    } else {
        let frames = Box::from_raw(ptr::slice_from_raw_parts_mut(frames.pdus, frames.length));
        for frame in frames {
            drop(Box::from_raw(ptr::slice_from_raw_parts_mut(
                frame.data,
                frame.length,
            )));
        }
    }
}

fn inner_encode_qoiz(
    input: &[u8],
    x: u16,
    y: u16,
    width: u16,
    height: u16,
) -> Result<Vec<Vec<u8>>, EncodeError> {
    let mut data = qoi::encode_to_vec(input, width as u32, height as u32)?;
    // our frames always have alpha set to 0xFF so we set channels number to 3 as it is
    // required by ironrdp decoding routine
    data[12] = 3;
    let data = zstd::encode_all(data.as_slice(), 0)?;
    let update =
        FastPathUpdate::SurfaceCommands(vec![SurfaceCommand::SetSurfaceBits(SurfaceBitsPdu {
            destination: ExclusiveRectangle {
                left: x,
                top: y,
                right: x + width,
                bottom: y + height,
            },
            extended_bitmap_data: ExtendedBitmapDataPdu {
                bpp: 0,
                codec_id: 0x0B, // qoiz
                width,
                height,
                header: None,
                data: &data,
            },
        })]);
    let data = encode_vec(&update)?;
    let mut pdus: Vec<_> = data
        .chunks(32760)
        .map(|chunk| {
            let pdu = FastPathUpdatePdu {
                fragmentation: Fragmentation::Next,
                update_code: UpdateCode::SurfaceCommands,
                compression_flags: None,
                compression_type: None,
                data: chunk,
            };
            let header = FastPathHeader::new(EncryptionFlags::empty(), pdu.size());
            (header, pdu)
        })
        .collect();
    if pdus.len() == 1 {
        pdus[0].1.fragmentation = Fragmentation::Single;
    } else {
        pdus[0].1.fragmentation = Fragmentation::First;
        pdus.last_mut().unwrap().1.fragmentation = Fragmentation::Last;
    }
    pdus.iter()
        .map(|(h, p)| {
            let mut buf = vec![0; h.size() + p.size()];
            let mut cursor = WriteCursor::new(&mut buf);
            h.encode(&mut cursor)?;
            p.encode(&mut cursor)?;
            Ok(buf)
        })
        .collect()
}

/// encode_pointer_bitmap builds a single FastPath NewPointer PDU containing a
/// 32bpp ARGB cursor sprite. `data` is `width * height * 4` bytes in BGRA byte
/// order, top-to-bottom (the byte layout of a little-endian ARGB u32). The PDU
/// reverses scanlines because RDP cursor bitmaps are stored bottom-up.
///
/// # Safety
///
/// `data` must point to a buffer of at least `width * height * 4` bytes.
#[no_mangle]
pub unsafe extern "C" fn encode_pointer_bitmap(
    data: *const u8,
    width: u16,
    height: u16,
    hotspot_x: u16,
    hotspot_y: u16,
) -> EncodingResult {
    let w = width as usize;
    let h = height as usize;
    let input = slice::from_raw_parts(data, w * h * 4);

    match inner_encode_pointer_bitmap(input, width, height, hotspot_x, hotspot_y) {
        Ok(buf) => single_pdu_result(buf),
        Err(e) => encoding_error(e),
    }
}

/// encode_pointer_default builds a single FastPath DefaultPointer PDU, telling
/// the client to fall back to its platform-default cursor. Used when a sprite is
/// too large to encode as a NewPointer PDU.
#[no_mangle]
pub unsafe extern "C" fn encode_pointer_default() -> EncodingResult {
    match inner_encode_fast_path_update(FastPathUpdate::Pointer(PointerUpdateData::SetDefault)) {
        Ok(buf) => single_pdu_result(buf),
        Err(e) => encoding_error(e),
    }
}

fn inner_encode_pointer_bitmap(
    bgra_top_down: &[u8],
    width: u16,
    height: u16,
    hotspot_x: u16,
    hotspot_y: u16,
) -> Result<Vec<u8>, EncodeError> {
    let w = width as usize;
    let h = height as usize;
    let stride = w * 4;

    // RDP wants bottom-up scanlines; XFixes (and our framebuffer convention) is
    // top-down. Reverse the rows once into a contiguous buffer.
    let mut xor_mask = Vec::with_capacity(stride * h);
    for row in (0..h).rev() {
        let start = row * stride;
        xor_mask.extend_from_slice(&bgra_top_down[start..start + stride]);
    }

    // 32bpp cursors with an alpha channel don't need a real and-mask, but RDP
    // still requires one with scanlines padded to a 16-bit boundary. Allocate
    // a zero buffer of the right size.
    let and_scanline = ((w + 15) / 16) * 2;
    let and_mask = vec![0u8; and_scanline * h];

    // A sprite whose encoded FastPath PDU would exceed the ~32KB FastPath length
    // limit can't be sent as a single NewPointer PDU (encode below returns an
    // error) and callers fall back to a default pointer. Real cursors are well
    // under that (XCURSOR_SIZE tops out at 72px).
    let attribute = PointerAttribute {
        xor_bpp: 32,
        color_pointer: ColorPointerAttribute {
            cache_index: 0,
            hot_spot: Point16 {
                x: hotspot_x,
                y: hotspot_y,
            },
            width,
            height,
            xor_mask: &xor_mask,
            and_mask: &and_mask,
        },
    };
    inner_encode_fast_path_update(FastPathUpdate::Pointer(PointerUpdateData::New(attribute)))
}

fn inner_encode_fast_path_update(update: FastPathUpdate<'_>) -> Result<Vec<u8>, EncodeError> {
    let update_code = UpdateCode::from(&update);
    let data = encode_vec(&update)?;
    let pdu = FastPathUpdatePdu {
        fragmentation: Fragmentation::Single,
        update_code,
        compression_flags: None,
        compression_type: None,
        data: &data,
    };
    let header = FastPathHeader::new(EncryptionFlags::empty(), pdu.size());
    let mut buf = vec![0; header.size() + pdu.size()];
    let mut cursor = WriteCursor::new(&mut buf);
    header.encode(&mut cursor)?;
    pdu.encode(&mut cursor)?;
    Ok(buf)
}

fn single_pdu_result(buf: Vec<u8>) -> EncodingResult {
    let frames = vec![Pdu {
        length: buf.len(),
        data: Box::into_raw(buf.into_boxed_slice()) as _,
    }];
    EncodingResult {
        length: frames.len(),
        pdus: Box::into_raw(frames.into_boxed_slice()) as _,
        error_code: CGOErrCode::ErrCodeSuccess,
        error_msg: ptr::null_mut(),
    }
}

fn encoding_error(e: EncodeError) -> EncodingResult {
    let msg = format!("{}", e);
    let b = msg.into_bytes().into_boxed_slice();
    EncodingResult {
        length: b.len(),
        pdus: ptr::null_mut(),
        error_msg: Box::into_raw(b) as _,
        error_code: CGOErrCode::ErrCodeFailure,
    }
}

#[derive(Debug)]
enum EncodeError {
    Zstd(std::io::Error),
    Qoi(qoi::Error),
    IronRdp(ironrdp_core::EncodeError),
}

impl From<std::io::Error> for EncodeError {
    fn from(e: std::io::Error) -> EncodeError {
        EncodeError::Zstd(e)
    }
}

impl From<qoi::Error> for EncodeError {
    fn from(e: qoi::Error) -> EncodeError {
        EncodeError::Qoi(e)
    }
}

impl From<ironrdp_core::EncodeError> for EncodeError {
    fn from(e: ironrdp_core::EncodeError) -> EncodeError {
        EncodeError::IronRdp(e)
    }
}

impl Display for EncodeError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            EncodeError::Zstd(e) => e.fmt(f),
            EncodeError::Qoi(e) => e.fmt(f),
            EncodeError::IronRdp(e) => e.fmt(f),
        }
    }
}

#[cfg(test)]
mod tests {
    use crate::linux_desktop_encoder::{
        inner_encode_fast_path_update, inner_encode_pointer_bitmap, inner_encode_qoiz,
    };
    use ironrdp_core::WriteBuf;
    use ironrdp_pdu::fast_path::FastPathUpdate;
    use ironrdp_pdu::pointer::PointerUpdateData;
    use ironrdp_session::fast_path::{ProcessorBuilder, UpdateKind};
    use ironrdp_session::image::DecodedImage;
    use rand::random_iter;

    #[test]
    pub fn encode_qoiz_single() {
        let input = &[0xFF, 0xFF, 0xFF, 0xFF];
        let encoded = inner_encode_qoiz(input, 0, 0, 1, 1).unwrap();
        assert_eq!(encoded.len(), 1);
        assert_eq!(
            encoded[0],
            vec![
                0u8, 59, 4, 54, 0, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 11, 1, 0, 1, 0, 32, 0, 0,
                0, 40, 181, 47, 253, 0, 88, 185, 0, 0, 113, 111, 105, 102, 0, 0, 0, 1, 0, 0, 0, 1,
                3, 0, 85, 0, 0, 0, 0, 0, 0, 0, 1
            ]
        );
        test_encoding(input, 1, 1);
    }

    #[test]
    pub fn encode_qoiz_random() {
        let mut vec: Vec<_> = random_iter().take(4 * 500 * 500).collect();
        for chunk in vec.chunks_exact_mut(4) {
            chunk[3] = 0xFF;
        }
        test_encoding(&vec, 500, 500);
    }

    fn test_encoding(vec: &[u8], width: u16, height: u16) {
        let encoded = inner_encode_qoiz(vec, 0, 0, width, height).unwrap();
        let mut processor = ProcessorBuilder {
            // These options only matter in a real RDP session when we have
            // to send responses back to the server. We can safely leave them
            // at defaults when decoding session recordings.
            io_channel_id: 0,
            user_channel_id: 0,
            enable_server_pointer: false,
            pointer_software_rendering: false,
        }
        .build();
        let mut image = DecodedImage::new(
            ironrdp_graphics::image_processing::PixelFormat::RgbX32,
            width,
            height,
        );
        let mut buf = WriteBuf::new();
        for frame in encoded {
            processor.process(&mut image, &frame, &mut buf).unwrap();
        }
        let res = vec
            .iter()
            .zip(image.data())
            .enumerate()
            .find(|(_, (a, b))| a != b);
        assert_eq!(res, None)
    }

    #[test]
    pub fn encode_pointer_bitmap_round_trip() {
        // 4x2 cursor: row 0 is solid blue (top), row 1 is solid red
        // (bottom). The encoder takes BGRA top-down on the wire; the decoder
        // outputs RGBA top-down for direct rendering on a canvas. Both row
        // order and colour-channel order should round-trip correctly.
        let blue_bgra = [0xFFu8, 0x00, 0x00, 0xFF];
        let red_bgra = [0x00u8, 0x00, 0xFF, 0xFF];
        let mut bgra = Vec::with_capacity(4 * 2 * 4);
        for _ in 0..4 {
            bgra.extend_from_slice(&blue_bgra);
        }
        for _ in 0..4 {
            bgra.extend_from_slice(&red_bgra);
        }

        let encoded = inner_encode_pointer_bitmap(&bgra, 4, 2, 1, 1).unwrap();
        let mut processor = ProcessorBuilder {
            io_channel_id: 0,
            user_channel_id: 0,
            enable_server_pointer: true,
            pointer_software_rendering: false,
        }
        .build();
        let mut image = DecodedImage::new(
            ironrdp_graphics::image_processing::PixelFormat::RgbA32,
            16,
            16,
        );
        let mut buf = WriteBuf::new();
        let updates = processor.process(&mut image, &encoded, &mut buf).unwrap();

        let pointer = updates
            .into_iter()
            .find_map(|u| match u {
                UpdateKind::PointerBitmap(p) => Some(p),
                _ => None,
            })
            .expect("decoder should emit PointerBitmap");
        assert_eq!(pointer.width, 4);
        assert_eq!(pointer.height, 2);
        assert_eq!(pointer.hotspot_x, 1);
        assert_eq!(pointer.hotspot_y, 1);

        let blue_rgba = [0x00u8, 0x00, 0xFF, 0xFF];
        let red_rgba = [0xFFu8, 0x00, 0x00, 0xFF];
        let mut expected = Vec::with_capacity(4 * 2 * 4);
        for _ in 0..4 {
            expected.extend_from_slice(&blue_rgba);
        }
        for _ in 0..4 {
            expected.extend_from_slice(&red_rgba);
        }
        assert_eq!(pointer.bitmap_data, expected);
    }

    #[test]
    pub fn oversized_pointer_bitmap_is_rejected() {
        // A 128x128 32bpp sprite encodes to ~67KB, past the ~32KB FastPath PDU
        // length limit, so encoding must fail rather than emit a corrupt PDU.
        // handleCursorChange relies on this to fall back to a default pointer.
        let bgra = vec![0xFFu8; 128 * 128 * 4];
        assert!(inner_encode_pointer_bitmap(&bgra, 128, 128, 0, 0).is_err());
    }

    #[test]
    pub fn encode_pointer_default_round_trip() {
        let encoded =
            inner_encode_fast_path_update(FastPathUpdate::Pointer(PointerUpdateData::SetDefault))
                .unwrap();
        let mut processor = ProcessorBuilder {
            io_channel_id: 0,
            user_channel_id: 0,
            enable_server_pointer: true,
            pointer_software_rendering: false,
        }
        .build();
        let mut image = DecodedImage::new(
            ironrdp_graphics::image_processing::PixelFormat::RgbA32,
            16,
            16,
        );
        let mut buf = WriteBuf::new();
        let updates = processor.process(&mut image, &encoded, &mut buf).unwrap();
        assert!(updates
            .iter()
            .any(|u| matches!(u, UpdateKind::PointerDefault)));
    }
}
