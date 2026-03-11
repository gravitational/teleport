use crate::CGOErrCode;
use ironrdp_core::{encode_vec, Encode, WriteCursor};
use ironrdp_pdu::fast_path::{
    EncryptionFlags, FastPathHeader, FastPathUpdate, FastPathUpdatePdu, Fragmentation, UpdateCode,
};
use ironrdp_pdu::geometry::ExclusiveRectangle;
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
        drop(Box::from_raw(slice::from_raw_parts_mut(
            frames.error_msg,
            frames.length,
        )));
    } else {
        let frames = Box::from_raw(slice::from_raw_parts_mut(frames.pdus, frames.length));
        for frame in frames {
            drop(Box::from_raw(slice::from_raw_parts_mut(
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
    use crate::encoder::inner_encode_qoiz;
    use ironrdp_core::WriteBuf;
    use ironrdp_session::fast_path::ProcessorBuilder;
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
}
