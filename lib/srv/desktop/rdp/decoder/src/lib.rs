// Teleport
// Copyright (C) 2025  Gravitational, Inc.
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

//! This crate contains an RDP decoder which can produce images given
//! a series of RDP fast path PDUs. It exposes a small C API so that
//! Go (via cgo) can create a decoder and call its methods.

use ironrdp_core::WriteBuf;
use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_session::{
    fast_path::{Processor, ProcessorBuilder, UpdateKind},
    image::DecodedImage,
};

pub struct RdpDecoder {
    image: DecodedImage,
    fast_path_processor: Processor,
}

impl RdpDecoder {
    pub fn new(width: u16, height: u16) -> Self {
        Self {
            image: DecodedImage::new(PixelFormat::RgbA32, width, height),
            fast_path_processor: ProcessorBuilder {
                // These channel IDs only matter in a real RDP session when we have
                // to send responses back to the server. We can safely leave them
                // set to 0 when decoding session recordings.
                io_channel_id: 0,
                user_channel_id: 0,
                // TODO: what does this change?
                enable_server_pointer: false,
                pointer_software_rendering: false,
            }
            .build(),
        }
    }

    pub fn resize(&mut self, width: u16, height: u16) {
        self.image = DecodedImage::new(PixelFormat::RgbA32, width, height);
    }

    pub fn image_data(&self) -> &[u8] {
        self.image.data()
    }

    pub fn process(&mut self, tdp_fast_path_frame: &[u8]) {
        let mut output = WriteBuf::new();

        // TODO: need error handling here
        let updates = self
            .fast_path_processor
            .process(&mut self.image, tdp_fast_path_frame, &mut output)
            .unwrap_or_default();

        for update in updates {
            match update {
                UpdateKind::None => {}
                UpdateKind::Region(inclusive_rectangle) => {
                    if inclusive_rectangle.right > self.image.width()
                        || inclusive_rectangle.bottom >= self.image.height()
                    {
                        todo!("Region exceeds bounds, needs resize!")
                    }
                }
                // We don't care about the mouse pointer.
                UpdateKind::PointerDefault => {}
                UpdateKind::PointerHidden => {}
                UpdateKind::PointerPosition { x: _, y: _ } => {}
                UpdateKind::PointerBitmap(_) => {}
            }
        }
    }
}

/// Create a new decoder and return an owned pointer to it.
/// The caller is responsible for calling rdp_decoder_free
/// when the decoder is no longer needed.
#[no_mangle]
pub extern "C" fn rdp_decoder_new(width: u16, height: u16) -> *mut RdpDecoder {
    Box::into_raw(Box::new(RdpDecoder::new(width, height)))
}

/// Frees the memory associated with a decoder.
#[no_mangle]
pub extern "C" fn rdp_decoder_free(ptr: *mut RdpDecoder) {
    unsafe {
        let _ = Box::from_raw(ptr);
    }
}

/// Resizes the decoder's internal image buffer.
#[no_mangle]
pub extern "C" fn rdp_decoder_resize(ptr: *mut RdpDecoder, width: u16, height: u16) {
    unsafe {
        let decoder = &mut *ptr;
        decoder.resize(width, height);
    }
}

#[no_mangle]
pub extern "C" fn rdp_decoder_process(ptr: *mut RdpDecoder, data: *const u8, len: usize) {
    if ptr.is_null() || data.is_null() || len == 0 {
        return;
    }
    unsafe {
        let decoder = &mut *ptr;
        let slice = std::slice::from_raw_parts(data, len);
        decoder.process(slice);
    }
}

/// Returns a pointer to the decoder's internal image buffer and writes its length to `out_len`.
/// The pointer is valid as long as the decoder is alive and is not mutated by other calls
/// (e.g. `process`, `resize` may reallocate). Caller should copy the data if it needs to keep it.
#[no_mangle]
pub extern "C" fn rdp_decoder_image_data(ptr: *mut RdpDecoder, out_len: *mut usize) -> *const u8 {
    if ptr.is_null() {
        if !out_len.is_null() {
            unsafe { *out_len = 0 };
        }
        return std::ptr::null();
    }
    unsafe {
        let decoder = &*ptr;
        let data = decoder.image_data();
        if !out_len.is_null() {
            *out_len = data.len();
        }
        data.as_ptr()
    }
}
