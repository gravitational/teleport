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

mod cursor;
mod regions;
mod image;

use crate::cursor::CursorState;
use crate::regions::UpdatedRegions;
use fast_image_resize::{FilterType, ResizeAlg, ResizeOptions, Resizer};
use ironrdp_core::WriteBuf;
use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_session::fast_path::UpdateKind;
use ironrdp_session::{
    fast_path::{Processor, ProcessorBuilder},
    image::DecodedImage,
};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::{io::Error, ptr};

pub struct RdpDecoder {
    image: DecodedImage,
    fast_path_processor: Processor,
    cursor_state: CursorState,
    updated_regions: UpdatedRegions,
    resizer: Resizer,
    thumbnail_scratch: Vec<u8>,
}

impl RdpDecoder {
    const PIXEL_FORMAT: PixelFormat = PixelFormat::RgbA32;

    pub fn new(width: u16, height: u16, io_channel_id: u16, user_channel_id: u16) -> Self {
        Self {
            image: DecodedImage::new(RdpDecoder::PIXEL_FORMAT, width, height),
            fast_path_processor: ProcessorBuilder {
                // Enable pointer updates so we can get the state of the cursor for when we create
                // cropped & zoomed in thumbnails in the session recording metadata generation.
                enable_server_pointer: true,
                io_channel_id,
                user_channel_id,
                // These options only matter in a real RDP session when we have
                // to send responses back to the server. We can safely leave them
                // at defaults when decoding session recordings.
                pointer_software_rendering: false,
                bulk_decompressor: None,
                // share_id is important for live RDP sessions
                // (see https://github.com/Devolutions/IronRDP/pull/1147)
                // but doesn't need to be set for our decoder.
                share_id: 0,
            }
            .build(),
            cursor_state: Default::default(),
            updated_regions: Default::default(),
            resizer: Resizer::new(),
            thumbnail_scratch: Vec::new(),
        }
    }

    pub fn resize(&mut self, width: u16, height: u16) {
        self.image = DecodedImage::new(RdpDecoder::PIXEL_FORMAT, width, height);
        self.cursor_state = Default::default();
        self.updated_regions.reset();
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
        // to use to create responses to send to the server.
        // We're only interested in updating the internal frame buffer,
        // so we can ignore the result.
        if let Ok(updates) =
            self.fast_path_processor
                .process(&mut self.image, tdp_fast_path_frame, &mut output)
        {
            for update in updates {
                match update {
                    UpdateKind::Region(rect) => {
                        self.updated_regions.push(rect);
                    }
                    UpdateKind::PointerBitmap(pointer) => {
                        self.cursor_state.set_visible(true);
                        self.cursor_state.set_bitmap(&pointer);
                    }
                    UpdateKind::PointerDefault => {
                        self.cursor_state.set_visible(true);
                        self.cursor_state.clear_bitmap();
                    }
                    UpdateKind::PointerHidden => self.cursor_state.set_visible(false),
                    UpdateKind::PointerPosition { x, y } => {
                        self.cursor_state.move_cursor(x, y);
                    }
                    _ => {}
                }
            }
        }
    }

    pub fn thumbnail(&mut self, width: u16, height: u16, dst: &mut [u8]) -> Result<(), Error> {
        let canvas_bytes = (width as usize) * (height as usize) * 4;
        if dst.len() < canvas_bytes {
            return Err(Error::other("destination buffer too small"));
        }

        let (fw, fh) = self.fitted_dimensions(width, height);
        if fw == 0 || fh == 0 {
            return Ok(());
        }

        let opts = ResizeOptions::new()
            .resize_alg(ResizeAlg::Nearest)
            .use_alpha(false);

        if fw == width && fh == height {
            return self.resize_image_into(fw, fh, dst, &opts);
        }

        let fitted_bytes = (fw as usize) * (fh as usize) * 4;
        if self.thumbnail_scratch.len() < fitted_bytes {
            self.thumbnail_scratch.resize(fitted_bytes, 0);
        }

        let Self {
            image,
            resizer,
            thumbnail_scratch,
            ..
        } = self;

        image::resize_into(
            image,
            resizer,
            fw,
            fh,
            &mut thumbnail_scratch[..fitted_bytes],
            &opts,
        )?;

        let row_bytes = (fw as usize) * 4;
        let canvas_stride = (width as usize) * 4;
        let offset_x = ((width - fw) as usize) / 2;
        let offset_y = ((height - fh) as usize) / 2;

        for row in 0..(fh as usize) {
            let src_off = row * row_bytes;
            let dst_off = (offset_y + row) * canvas_stride + offset_x * 4;
            dst[dst_off..dst_off + row_bytes]
                .copy_from_slice(&thumbnail_scratch[src_off..src_off + row_bytes]);
        }

        Ok(())
    }

    pub fn fitted_dimensions(&self, max_width: u16, max_height: u16) -> (u16, u16) {
        let src_w = self.image.width();
        let src_h = self.image.height();

        if src_w <= max_width && src_h <= max_height {
            return (src_w, src_h);
        }

        let scale_w = f64::from(max_width) / f64::from(src_w);
        let scale_h = f64::from(max_height) / f64::from(src_h);
        let scale = scale_w.min(scale_h);

        let out_w = ((f64::from(src_w) * scale).round() as u16).clamp(1, max_width);
        let out_h = ((f64::from(src_h) * scale).round() as u16).clamp(1, max_height);

        (out_w, out_h)
    }

    /// FNV-1a 64-bit digest of pixels sampled on a fixed grid from the current
    /// frame buffer. `sample_count` controls per-axis sample density; the
    /// effective step is `max(1, dim / sample_count)`. Matches Go `hash/fnv`
    /// `New64a()` byte-for-byte for the same inputs so dedup comparisons stay
    /// consistent across the cgo boundary.
    pub fn sample_hash(&self, sample_count: u16) -> u64 {
        sample_hash(
            self.image.data(),
            self.image.width(),
            self.image.height(),
            sample_count,
        )
    }
}

const FNV_OFFSET_BASIS: u64 = 0xcbf2_9ce4_8422_2325;
const FNV_PRIME: u64 = 0x0000_0100_0000_01b3;

fn sample_hash(data: &[u8], width: u16, height: u16, sample_count: u16) -> u64 {
    const BYTES_PER_PIXEL: usize = 4;

    let w = width as usize;
    let h = height as usize;
    if w == 0 || h == 0 || sample_count == 0 {
        return 0;
    }

    let step_x = (w / usize::from(sample_count)).max(1);
    let step_y = (h / usize::from(sample_count)).max(1);
    let stride = w * BYTES_PER_PIXEL;
    let row_step = stride * step_y;
    let col_step = BYTES_PER_PIXEL * step_x;
    let col_end = w * BYTES_PER_PIXEL;
    let pix_len = data.len();

    let mut hash: u64 = FNV_OFFSET_BASIS;
    let mut row_off: usize = 0;
    let mut y = 0usize;
    while y < h {
        let mut off = row_off;
        while off < row_off + col_end && off + BYTES_PER_PIXEL <= pix_len {
            for b in &data[off..off + BYTES_PER_PIXEL] {
                hash ^= u64::from(*b);
                hash = hash.wrapping_mul(FNV_PRIME);
            }
            off += col_step;
        }
        row_off += row_step;
        y += step_y;
    }
    hash
}

/// Create a new decoder and return an owned pointer to it.
/// The caller is responsible for calling rdp_decoder_free
/// when the decoder is no longer needed.
#[no_mangle]
pub extern "C" fn rdp_decoder_new(
    width: u16,
    height: u16,
    io_channel_id: u16,
    user_channel_id: u16,
) -> *mut RdpDecoder {
    catch_unwind(AssertUnwindSafe(move || {
        Box::into_raw(Box::new(RdpDecoder::new(
            width,
            height,
            io_channel_id,
            user_channel_id,
        )))
    }))
    .unwrap_or(ptr::null_mut())
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
    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
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
    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
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
    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
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

    catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        let data = decoder.image_data();

        *out_width = decoder.image.width();
        *out_height = decoder.image.height();
        data.as_ptr()
    }))
    .unwrap_or(ptr::null())
}

/// Writes a CatmullRom-resized copy of the source crop region into `out_buf`,
/// scaled to exactly `out_width` x `out_height`. The crop must be within the
/// current frame bounds; out-of-bounds crops are rejected.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - `out_buf` must point to `out_buf_len` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_resize_crop(
    ptr: *mut RdpDecoder,
    crop_x: u16,
    crop_y: u16,
    crop_w: u16,
    crop_h: u16,
    out_width: u16,
    out_height: u16,
    out_buf: *mut u8,
    out_buf_len: usize,
) {
    if ptr.is_null()
        || out_buf.is_null()
        || out_width == 0
        || out_height == 0
        || crop_w == 0
        || crop_h == 0
    {
        return;
    }

    let needed = (out_width as usize) * (out_height as usize) * 4;
    if out_buf_len < needed {
        return;
    }

    let opts = ResizeOptions::new()
        .resize_alg(ResizeAlg::Convolution(FilterType::CatmullRom))
        .use_alpha(false)
        .crop(
            f64::from(crop_x),
            f64::from(crop_y),
            f64::from(crop_w),
            f64::from(crop_h),
        );

    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &mut *ptr;
        // Reject crops that extend past the frame.
        let src_w = decoder.image.width();
        let src_h = decoder.image.height();
        if u32::from(crop_x) + u32::from(crop_w) > u32::from(src_w)
            || u32::from(crop_y) + u32::from(crop_h) > u32::from(src_h)
        {
            return;
        }

        let dst = std::slice::from_raw_parts_mut(out_buf, needed);
        let _ = decoder.resize_image_into(out_width, out_height, dst, &opts);
    }));
}

/// Returns the current frame dimensions via out-params. Sets both to 0 on
/// failure (null pointer, panic).
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - `out_width` and `out_height` must be valid, non-null pointers.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_dimensions(
    ptr: *mut RdpDecoder,
    out_width: *mut u16,
    out_height: *mut u16,
) {
    if out_width.is_null() || out_height.is_null() {
        return;
    }
    unsafe {
        *out_width = 0;
        *out_height = 0;
    }

    if ptr.is_null() {
        return;
    }

    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        *out_width = decoder.width();
        *out_height = decoder.height();
    }));
}

/// Returns an FNV-1a 64-bit digest of pixels sampled on a fixed grid from the
/// current frame buffer. `sample_count` controls the per-axis sample density.
/// Returns 0 on null pointer or empty frame; the cursor is not composited so
/// callers can match a non-cursor Go-side sampler exactly.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_sample_hash(ptr: *mut RdpDecoder, sample_count: u16) -> u64 {
    if ptr.is_null() {
        return 0;
    }

    catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        decoder.sample_hash(sample_count)
    }))
    .unwrap_or(0)
}

/// Writes a fast nearest-neighbor thumbnail of the current frame, fitted
/// while preserving aspect ratio and centered with transparent padding,
/// into a `width` x `height` RGBA buffer. The caller must
/// zero-initialize `out_buf`.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - `out_buf` must point to `out_buf_len` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_thumbnail(
    ptr: *mut RdpDecoder,
    width: u16,
    height: u16,
    out_buf: *mut u8,
    out_buf_len: usize,
) {
    if ptr.is_null() || out_buf.is_null() || width == 0 || height == 0 {
        return;
    }

    let needed = (width as usize) * (height as usize) * 4;
    if out_buf_len < needed {
        return;
    }

    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &mut *ptr;
        let dst = std::slice::from_raw_parts_mut(out_buf, needed);
        let _ = decoder.thumbnail(width, height, dst);
    }));
}

/// Returns the current cursor position and visibility state.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - All out-params must be valid, non-null pointers.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_cursor_state(
    ptr: *mut RdpDecoder,
    out_visible: *mut u8,
    out_x: *mut u16,
    out_y: *mut u16,
) {
    if ptr.is_null() || out_visible.is_null() || out_x.is_null() || out_y.is_null() {
        return;
    }

    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        let (x, y) = decoder.cursor_state.position();

        *out_visible = decoder.cursor_state.is_visible() as u8;
        *out_x = x;
        *out_y = y;
    }));
}

/// Returns a pointer to the cursor bitmap data and its dimensions via out-params.
/// Returns null if no cursor bitmap is available. The returned pointer is valid
/// as long as the decoder is alive and no new PointerBitmap update is processed.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - All out-params must be valid, non-null pointers.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_cursor_bitmap(
    ptr: *mut RdpDecoder,
    out_width: *mut u16,
    out_height: *mut u16,
    out_hotspot_x: *mut u16,
    out_hotspot_y: *mut u16,
) -> *const u8 {
    if ptr.is_null()
        || out_width.is_null()
        || out_height.is_null()
        || out_hotspot_x.is_null()
        || out_hotspot_y.is_null()
    {
        return ptr::null();
    }

    catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        let Some(bmp) = decoder.cursor_state.bitmap() else {
            return ptr::null();
        };
        bmp.write_metadata(out_width, out_height, out_hotspot_x, out_hotspot_y);
        bmp.data_ptr()
    }))
    .unwrap_or(ptr::null())
}

/// Copies update regions into the caller-provided buffer as (left, top, right, bottom)
/// u16 tuples. Returns the number of regions written. Coordinates use inclusive
/// right/bottom matching the RDP InclusiveRectangle convention.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
/// - `out_buf` must point to at least `max_count * 4` writable u16 values.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_updated_regions(
    ptr: *mut RdpDecoder,
    out_buf: *mut u16,
    max_count: u32,
) -> u32 {
    if ptr.is_null() || out_buf.is_null() || max_count == 0 {
        return 0;
    }

    catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        let out = std::slice::from_raw_parts_mut(out_buf as *mut [u16; 4], max_count as usize);

        let n = decoder.updated_regions.len().min(max_count as usize) as u32;
        for (slot, coords) in out.iter_mut().zip(decoder.updated_regions.iter()) {
            *slot = coords;
        }

        n
    }))
    .unwrap_or(0)
}

/// Clears the accumulated update regions.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_reset_updated_regions(ptr: *mut RdpDecoder) {
    if ptr.is_null() {
        return;
    }

    let _ = catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &mut *ptr;
        decoder.updated_regions.reset();
    }));
}

/// Returns the number of update regions accumulated since the last reset.
///
/// # Safety
///
/// - `ptr` must be a valid pointer previously returned by `rdp_decoder_new`.
#[no_mangle]
pub unsafe extern "C" fn rdp_decoder_updated_regions_count(ptr: *mut RdpDecoder) -> u32 {
    if ptr.is_null() {
        return 0;
    }

    catch_unwind(AssertUnwindSafe(move || unsafe {
        let decoder = &*ptr;
        decoder.updated_regions.len() as u32
    }))
    .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fitted_dimensions_cases() {
        let cases: &[(u16, u16, u16, u16, u16, u16, &str)] = &[
            (800, 600, 1024, 768, 800, 600, "no upscale: source fits"),
            (800, 600, 800, 600, 800, 600, "exact fit"),
            (1920, 1080, 960, 540, 960, 540, "landscape proportional downscale"),
            (1080, 1920, 540, 960, 540, 960, "portrait proportional downscale"),
            (1920, 1080, 800, 600, 800, 450, "landscape clamped by width"),
            (1080, 1920, 800, 600, 338, 600, "portrait clamped by height"),
            (1, 10000, 100, 100, 1, 100, "extreme aspect ratio scales to 1"),
            (10000, 1, 100, 100, 100, 1, "extreme aspect ratio scales to 1 (other axis)"),
            (3, 3, 2, 2, 2, 2, "round-down to fit"),
        ];

        for &(src_w, src_h, max_w, max_h, want_w, want_h, name) in cases {
            let decoder = RdpDecoder::new(src_w, src_h, 1003, 1007);
            let (got_w, got_h) = decoder.fitted_dimensions(max_w, max_h);
            assert_eq!(
                (got_w, got_h),
                (want_w, want_h),
                "{name}: src=({src_w},{src_h}) max=({max_w},{max_h})",
            );
        }
    }

    #[test]
    fn fitted_dimensions_minimum_is_one_pixel() {
        let decoder = RdpDecoder::new(1000, 1, 1003, 1007);
        let (w, h) = decoder.fitted_dimensions(10, 10);
        assert!(w >= 1 && h >= 1, "fitted dims must be at least 1x1, got ({w},{h})");
    }

    fn make_decoder(width: u16, height: u16) -> *mut RdpDecoder {
        let ptr = rdp_decoder_new(width, height, 1003, 1007);
        assert!(!ptr.is_null());
        ptr
    }

    #[test]
    fn resize_crop_rejects_crop_extending_past_width() {
        let ptr = make_decoder(100, 100);
        let mut buf = vec![0xAAu8; 10 * 10 * 4];

        unsafe {
            rdp_decoder_resize_crop(
                ptr, 90, 0, 20, 10, 10, 10, buf.as_mut_ptr(), buf.len(),
            );
        }

        // Crop extends past right edge → call returns without writing.
        assert!(buf.iter().all(|&b| b == 0xAA));

        unsafe { rdp_decoder_free(ptr) };
    }

    #[test]
    fn resize_crop_rejects_crop_extending_past_height() {
        let ptr = make_decoder(100, 100);
        let mut buf = vec![0xAAu8; 10 * 10 * 4];

        unsafe {
            rdp_decoder_resize_crop(
                ptr, 0, 95, 10, 10, 10, 10, buf.as_mut_ptr(), buf.len(),
            );
        }

        assert!(buf.iter().all(|&b| b == 0xAA));

        unsafe { rdp_decoder_free(ptr) };
    }

    #[test]
    fn resize_crop_rejects_zero_crop_dimensions() {
        let ptr = make_decoder(100, 100);
        let mut buf = vec![0xAAu8; 10 * 10 * 4];

        unsafe {
            // crop_w = 0
            rdp_decoder_resize_crop(ptr, 0, 0, 0, 10, 10, 10, buf.as_mut_ptr(), buf.len());
            assert!(buf.iter().all(|&b| b == 0xAA));

            // crop_h = 0
            rdp_decoder_resize_crop(ptr, 0, 0, 10, 0, 10, 10, buf.as_mut_ptr(), buf.len());
            assert!(buf.iter().all(|&b| b == 0xAA));

            rdp_decoder_free(ptr);
        }
    }

    #[test]
    fn resize_crop_accepts_in_bounds_crop() {
        let ptr = make_decoder(100, 100);
        let mut buf = vec![0xAAu8; 10 * 10 * 4];

        unsafe {
            // Exactly fills the right edge: 90 + 10 == 100.
            rdp_decoder_resize_crop(
                ptr, 90, 90, 10, 10, 10, 10, buf.as_mut_ptr(), buf.len(),
            );
        }

        // Decoder buffer is zero-initialized, so the resize writes zeros.
        assert!(buf.iter().any(|&b| b != 0xAA), "expected buffer to be written");

        unsafe { rdp_decoder_free(ptr) };
    }

    #[test]
    fn sample_hash_empty_inputs_return_zero() {
        assert_eq!(sample_hash(&[], 0, 0, 64), 0);
        assert_eq!(sample_hash(&[1, 2, 3, 4], 1, 1, 0), 0);
        assert_eq!(sample_hash(&[1, 2, 3, 4], 0, 1, 64), 0);
        assert_eq!(sample_hash(&[1, 2, 3, 4], 1, 0, 64), 0);
    }

    #[test]
    fn sample_hash_matches_fnv1a_reference() {
        let mut h = FNV_OFFSET_BASIS;
        for b in [1u8, 2, 3, 4] {
            h ^= u64::from(b);
            h = h.wrapping_mul(FNV_PRIME);
        }
        assert_eq!(sample_hash(&[1, 2, 3, 4], 1, 1, 64), h);
    }

    #[test]
    fn sample_hash_step_skips_pixels() {
        // 4x1 RGBA frame, sample_count=2 → step_x = 4/2 = 2, so only pixels
        // 0 and 2 are sampled.
        let mut data = Vec::with_capacity(16);
        for v in [1u8, 2, 3, 4] {
            data.extend_from_slice(&[v, v, v, 255]);
        }

        let mut h = FNV_OFFSET_BASIS;
        for v in [1u8, 1, 1, 255, 3, 3, 3, 255] {
            h ^= u64::from(v);
            h = h.wrapping_mul(FNV_PRIME);
        }

        assert_eq!(sample_hash(&data, 4, 1, 2), h);
    }

    #[test]
    fn sample_hash_changes_when_pixels_change() {
        let mut a = vec![0u8; 4];
        a[0] = 1;
        let mut b = vec![0u8; 4];
        b[0] = 2;
        assert_ne!(sample_hash(&a, 1, 1, 64), sample_hash(&b, 1, 1, 64));
    }
}
