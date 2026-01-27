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

use std::panic::{catch_unwind, AssertUnwindSafe};
use std::ptr;

use ironrdp_core::WriteBuf;
use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_session::{
    fast_path::{Processor, ProcessorBuilder},
    image::DecodedImage,
};

pub struct RdpDecoder {
    image: DecodedImage,
    fast_path_processor: Processor,
}

impl RdpDecoder {
    const PIXEL_FORMAT: PixelFormat = PixelFormat::RgbA32;

    pub fn new(width: u16, height: u16) -> Self {
        Self {
            image: DecodedImage::new(RdpDecoder::PIXEL_FORMAT, width, height),
            fast_path_processor: ProcessorBuilder {
                // These options only matter in a real RDP session when we have
                // to send responses back to the server. We can safely leave them
                // at defaults when decoding session recordings.
                io_channel_id: 0,
                user_channel_id: 0,
                enable_server_pointer: false,
                pointer_software_rendering: false,
            }
            .build(),
        }
    }

    pub fn resize(&mut self, width: u16, height: u16) {
        self.image = DecodedImage::new(RdpDecoder::PIXEL_FORMAT, width, height);
    }

    pub fn image_data(&self) -> &[u8] {
        self.image.data()
    }

    pub fn width(&self) -> u16 {
        self.image.width()
    }

    pub fn height(&self) -> u16 {
        self.image.height()
    }

    pub fn process(&mut self, tdp_fast_path_frame: &[u8]) {
        let mut output = WriteBuf::new();

        // In a live RDP connection, this would return data that we need
        // to use to create reponses to send to the server.
        // We're only interested in updating the internal frame buffer,
        // so we can ignore the result.
        let _ = self
            .fast_path_processor
            .process(&mut self.image, tdp_fast_path_frame, &mut output);
    }
}

/// Create a new decoder and return an owned pointer to it.
/// The caller is responsible for calling rdp_decoder_free
/// when the decoder is no longer needed.
#[no_mangle]
pub extern "C" fn rdp_decoder_new(width: u16, height: u16) -> *mut RdpDecoder {
    match catch_unwind_and_drop_panic_payload(AssertUnwindSafe(move || {
        Box::into_raw(Box::new(RdpDecoder::new(width, height)))
    })) {
        Ok(ptr) => ptr,
        Err(_) => ptr::null_mut(),
    }
}

/// Frees the memory associated with a decoder.
///
/// # Safety
///
/// `ptr` must be a pointer allocated by `rdp_decoder_new`.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_free(ptr: *mut RdpDecoder) {
    if ptr.is_null() {
        return;
    }
    let _ = catch_unwind_and_drop_panic_payload(AssertUnwindSafe(move || unsafe {
        let _ = Box::from_raw(ptr);
    }));
}

/// Resizes the decoder's internal image buffer.
///
/// # Safety
///
/// - `ptr` must be a valid, non-null pointer previously returned by `rdp_decoder_new`
///
/// Note: Resizing replaces the decoder's internal image buffer; any previously obtained
/// pointers into the internal buffer (for example via `rdp_decoder_image_data`) may become
/// invalid after this call.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_resize(ptr: *mut RdpDecoder, width: u16, height: u16) {
    if ptr.is_null() {
        return;
    }
    let _ = catch_unwind_and_drop_panic_payload(AssertUnwindSafe(move || unsafe {
        let decoder = &mut *ptr;
        decoder.resize(width, height);
    }));
}

/// Processes an RDP fast path frame and updates the internal state of the frame buffer.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new` and
/// - `data` must point to `len` contiguous bytes that are readable by this function.
///   Passing a null `data` pointer with `len > 0` is invalid. If `len == 0` the function
///   returns immediately and no bytes are read.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_process(ptr: *mut RdpDecoder, data: *const u8, len: usize) {
    if ptr.is_null() || data.is_null() || len == 0 {
        return;
    }
    let _ = catch_unwind_and_drop_panic_payload(AssertUnwindSafe(move || unsafe {
        let decoder = &mut *ptr;
        let slice = std::slice::from_raw_parts(data, len);
        decoder.process(slice);
    }));
}

/// Returns a pointer to the decoder's internal image buffer. The `out_width` and `out_height`
/// outparams receive the size of the image in pixels.
///
///
/// # Safety
///
/// The returned pointer is valid as long as the decoder is alive and is not mutated by other calls
/// (e.g. `process`, `resize`). Caller should copy the data if it needs to keep it.
///
/// The returned pointer references out_width * out_height * 4 bytes.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_image_data(
    ptr: *mut RdpDecoder,
    out_width: *mut u16,
    out_height: *mut u16,
) -> *const u8 {
    const { assert!(matches!(RdpDecoder::PIXEL_FORMAT, PixelFormat::RgbA32)) }

    if ptr.is_null() || out_width.is_null() || out_height.is_null() {
        return ptr::null();
    }

    match catch_unwind_and_drop_panic_payload(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        let data = decoder.image_data();

        *out_width = decoder.image.width();
        *out_height = decoder.image.height();
        data.as_ptr()
    })) {
        Ok(p) => p,
        Err(_) => ptr::null(),
    }
}

fn catch_unwind_and_drop_panic_payload<F: FnOnce() -> R, R>(f: F) -> Result<R, ()> {
    catch_unwind(AssertUnwindSafe(f)).map_err(|e| {
        // If dropping the original panic payload causes another panic,
        // abort the process.
        catch_unwind(AssertUnwindSafe(move || std::mem::drop(e)))
            .unwrap_or_else(|_e| std::process::abort())
    })
}
