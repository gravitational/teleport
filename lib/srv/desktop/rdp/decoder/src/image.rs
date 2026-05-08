// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

use crate::cursor::CursorState;
use crate::RdpDecoder;
use fast_image_resize::images::{Image, ImageRef};
use fast_image_resize::{ImageBufferError, PixelType, ResizeOptions, Resizer};
use ironrdp_session::image::DecodedImage;
use std::io::Error;

impl RdpDecoder {
    pub fn resize_image_into(
        &mut self,
        out_w: u16,
        out_h: u16,
        dst: &mut [u8],
        opts: &ResizeOptions,
        cursor: Option<(u16, u16)>,
    ) -> Result<(), Error> {
        resize_into(
            &self.image,
            &self.cursor_state,
            &mut self.resizer,
            &mut self.composite_scratch,
            out_w,
            out_h,
            dst,
            opts,
            cursor,
        )
    }
}

pub(crate) fn resize_into(
    image: &DecodedImage,
    cursor_state: &CursorState,
    resizer: &mut Resizer,
    composite_scratch: &mut Vec<u8>,
    out_w: u16,
    out_h: u16,
    dst: &mut [u8],
    opts: &ResizeOptions,
    cursor: Option<(u16, u16)>,
) -> Result<(), Error> {
    let needed = (out_w as usize) * (out_h as usize) * 4;
    if dst.len() < needed {
        return Err(Error::other("destination buffer too small"));
    }

    let src = image_ref(image, cursor_state, composite_scratch, cursor).map_err(Error::other)?;

    let mut dst_image = Image::from_slice_u8(
        u32::from(out_w),
        u32::from(out_h),
        &mut dst[..needed],
        PixelType::U8x4,
    )
    .map_err(Error::other)?;

    resizer
        .resize(&src, &mut dst_image, opts)
        .map_err(Error::other)?;

    Ok(())
}

fn image_ref<'a>(
    image: &'a DecodedImage,
    cursor_state: &CursorState,
    composite_scratch: &'a mut Vec<u8>,
    cursor: Option<(u16, u16)>,
) -> Result<ImageRef<'a>, ImageBufferError> {
    let src_w = image.width();
    let src_h = image.height();

    let src_buf = if let Some((cx, cy)) = cursor {
        let cursor_bitmap = cursor_state.bitmap_or_default();

        let bytes = (src_w as usize) * (src_h as usize) * 4;
        if composite_scratch.len() < bytes {
            composite_scratch.resize(bytes, 0);
        }
        composite_scratch[..bytes].copy_from_slice(image.data());

        composite_over(
            &mut composite_scratch[..bytes],
            src_w,
            src_h,
            &*cursor_bitmap.data,
            cursor_bitmap.width,
            cursor_bitmap.height,
            i32::from(cx) - i32::from(cursor_bitmap.hotspot_x),
            i32::from(cy) - i32::from(cursor_bitmap.hotspot_y),
        );

        &composite_scratch[..bytes]
    } else {
        image.data()
    };

    ImageRef::new(u32::from(src_w), u32::from(src_h), src_buf, PixelType::U8x4)
}

// Source-over blend of `bmp_pix` onto `dst` at (`draw_x`, `draw_y`).
// Matches the alpha convention of Go's `image/draw.Over` used previously.
fn composite_over(
    dst: &mut [u8],
    dst_w: u16,
    dst_h: u16,
    bmp_pix: &[u8],
    bmp_w: u16,
    bmp_h: u16,
    draw_x: i32,
    draw_y: i32,
) {
    let dw = i32::from(dst_w);
    let dh = i32::from(dst_h);
    let bw = i32::from(bmp_w);
    let bh = i32::from(bmp_h);

    let x0 = draw_x.max(0);
    let y0 = draw_y.max(0);
    let x1 = (draw_x + bw).min(dw);
    let y1 = (draw_y + bh).min(dh);
    if x0 >= x1 || y0 >= y1 {
        return;
    }

    let dst_stride = (dw as usize) * 4;
    let bmp_stride = (bw as usize) * 4;
    let row_bytes = ((x1 - x0) as usize) * 4;
    let dst_x_off = (x0 as usize) * 4;
    let bmp_x_off = ((x0 - draw_x) as usize) * 4;

    for y in y0..y1 {
        let dst_start = (y as usize) * dst_stride + dst_x_off;
        let bmp_start = ((y - draw_y) as usize) * bmp_stride + bmp_x_off;

        let dst_row = &mut dst[dst_start..dst_start + row_bytes];
        let bmp_row = &bmp_pix[bmp_start..bmp_start + row_bytes];

        for (dst_px, bmp_px) in dst_row.chunks_exact_mut(4).zip(bmp_row.chunks_exact(4)) {
            let sa = bmp_px[3];
            if sa == 0 {
                continue;
            }
            if sa == 255 {
                dst_px.copy_from_slice(bmp_px);
                continue;
            }

            let inv = 255 - u32::from(sa);
            for c in 0..4 {
                let s = u32::from(bmp_px[c]);
                let d = u32::from(dst_px[c]);
                // (x * 0x8081) >> 23 is a fast `x / 255` for x up to ~16 bits.
                let blended = s + ((d * inv * 0x8081) >> 23);

                dst_px[c] = blended.min(255) as u8;
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ironrdp_graphics::image_processing::PixelFormat;
    use ironrdp_graphics::pointer::DecodedPointer;

    fn make_dst(w: u16, h: u16, fill: u8) -> Vec<u8> {
        vec![fill; (w as usize) * (h as usize) * 4]
    }

    fn make_bmp(w: u16, h: u16, fill: [u8; 4]) -> Vec<u8> {
        let mut v = Vec::with_capacity((w as usize) * (h as usize) * 4);
        for _ in 0..(w as usize) * (h as usize) {
            v.extend_from_slice(&fill);
        }
        v
    }

    fn pixel(buf: &[u8], stride: usize, x: usize, y: usize) -> [u8; 4] {
        let off = y * stride + x * 4;
        [buf[off], buf[off + 1], buf[off + 2], buf[off + 3]]
    }

    #[test]
    fn composite_over_transparent_source_leaves_dst_unchanged() {
        let mut dst = make_dst(2, 2, 200);
        let bmp = make_bmp(2, 2, [10, 20, 30, 0]);

        composite_over(&mut dst, 2, 2, &bmp, 2, 2, 0, 0);

        assert_eq!(dst, make_dst(2, 2, 200));
    }

    #[test]
    fn composite_over_opaque_source_overwrites_dst() {
        let mut dst = make_dst(2, 2, 200);
        let bmp = make_bmp(2, 2, [10, 20, 30, 255]);

        composite_over(&mut dst, 2, 2, &bmp, 2, 2, 0, 0);

        for y in 0..2 {
            for x in 0..2 {
                assert_eq!(pixel(&dst, 8, x, y), [10, 20, 30, 255]);
            }
        }
    }

    #[test]
    fn composite_over_half_alpha_blends_source_over_dst() {
        // inv = 255 - 128 = 127; (200 * 127 * 0x8081) >> 23 == 99.
        // RGB: s=50, d=200 → 50 + 99 = 149.
        // A:   s=128, d=200 → 128 + 99 = 227.
        let mut dst = make_dst(1, 1, 200);
        let bmp = make_bmp(1, 1, [50, 50, 50, 128]);

        composite_over(&mut dst, 1, 1, &bmp, 1, 1, 0, 0);

        assert_eq!(pixel(&dst, 4, 0, 0), [149, 149, 149, 227]);
    }

    #[test]
    fn composite_over_skips_when_fully_off_screen() {
        let original = make_dst(2, 2, 200);
        let bmp = make_bmp(2, 2, [10, 20, 30, 255]);

        for (dx, dy) in [(-2, 0), (2, 0), (0, -2), (0, 2), (-5, -5), (10, 10)] {
            let mut dst = original.clone();
            composite_over(&mut dst, 2, 2, &bmp, 2, 2, dx, dy);
            assert_eq!(dst, original, "expected no change at draw=({dx},{dy})");
        }
    }

    #[test]
    fn composite_over_clips_negative_draw_x() {
        // 4x1 bmp at draw_x=-2, dst is 4x1: only bmp cols 2,3 land in dst cols 0,1.
        let mut dst = make_dst(4, 1, 0);
        let mut bmp = Vec::new();
        for v in [1u8, 2, 3, 4] {
            bmp.extend_from_slice(&[v, v, v, 255]);
        }

        composite_over(&mut dst, 4, 1, &bmp, 4, 1, -2, 0);

        assert_eq!(pixel(&dst, 16, 0, 0), [3, 3, 3, 255]);
        assert_eq!(pixel(&dst, 16, 1, 0), [4, 4, 4, 255]);
        assert_eq!(pixel(&dst, 16, 2, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 16, 3, 0), [0, 0, 0, 0]);
    }

    #[test]
    fn composite_over_clips_negative_draw_y() {
        // 1x4 bmp at draw_y=-2, dst is 1x4: only bmp rows 2,3 land in dst rows 0,1.
        let mut dst = make_dst(1, 4, 0);
        let mut bmp = Vec::new();
        for v in [1u8, 2, 3, 4] {
            bmp.extend_from_slice(&[v, v, v, 255]);
        }

        composite_over(&mut dst, 1, 4, &bmp, 1, 4, 0, -2);

        assert_eq!(pixel(&dst, 4, 0, 0), [3, 3, 3, 255]);
        assert_eq!(pixel(&dst, 4, 0, 1), [4, 4, 4, 255]);
        assert_eq!(pixel(&dst, 4, 0, 2), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 4, 0, 3), [0, 0, 0, 0]);
    }

    #[test]
    fn composite_over_clips_right_overflow() {
        // 4x1 bmp at draw_x=2, dst is 4x1: only bmp cols 0,1 land in dst cols 2,3.
        let mut dst = make_dst(4, 1, 0);
        let mut bmp = Vec::new();
        for v in [1u8, 2, 3, 4] {
            bmp.extend_from_slice(&[v, v, v, 255]);
        }

        composite_over(&mut dst, 4, 1, &bmp, 4, 1, 2, 0);

        assert_eq!(pixel(&dst, 16, 0, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 16, 1, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 16, 2, 0), [1, 1, 1, 255]);
        assert_eq!(pixel(&dst, 16, 3, 0), [2, 2, 2, 255]);
    }

    #[test]
    fn composite_over_clips_bottom_overflow() {
        // 1x4 bmp at draw_y=2, dst is 1x4: only bmp rows 0,1 land in dst rows 2,3.
        let mut dst = make_dst(1, 4, 0);
        let mut bmp = Vec::new();
        for v in [1u8, 2, 3, 4] {
            bmp.extend_from_slice(&[v, v, v, 255]);
        }

        composite_over(&mut dst, 1, 4, &bmp, 1, 4, 0, 2);

        assert_eq!(pixel(&dst, 4, 0, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 4, 0, 1), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 4, 0, 2), [1, 1, 1, 255]);
        assert_eq!(pixel(&dst, 4, 0, 3), [2, 2, 2, 255]);
    }

    fn pointer(width: u16, height: u16, hotspot_x: u16, hotspot_y: u16, fill: [u8; 4]) -> DecodedPointer {
        let mut bitmap_data = Vec::new();
        for _ in 0..(width as usize) * (height as usize) {
            bitmap_data.extend_from_slice(&fill);
        }
        DecodedPointer {
            width,
            height,
            hotspot_x,
            hotspot_y,
            bitmap_data,
        }
    }

    fn run_image_ref(
        src_w: u16,
        src_h: u16,
        cursor_state: &CursorState,
        cursor: Option<(u16, u16)>,
    ) -> (Vec<u8>, bool) {
        let image = DecodedImage::new(PixelFormat::RgbA32, src_w, src_h);
        let mut composite_scratch = Vec::new();
        let scratch_len_before = composite_scratch.len();
        let _ = image_ref(&image, cursor_state, &mut composite_scratch, cursor)
            .expect("image_ref must succeed");
        let scratch_grew = composite_scratch.len() > scratch_len_before;
        (composite_scratch, scratch_grew)
    }

    #[test]
    fn image_ref_without_cursor_does_not_touch_scratch() {
        let cursor_state = CursorState::default();
        let (scratch, grew) = run_image_ref(4, 4, &cursor_state, None);
        assert!(scratch.is_empty(), "scratch should remain unallocated");
        assert!(!grew);
    }

    #[test]
    fn image_ref_with_cursor_at_origin_writes_to_top_left() {
        let mut cursor_state = CursorState::default();
        cursor_state.set_bitmap(&pointer(2, 2, 0, 0, [9, 9, 9, 255]));

        let (scratch, _) = run_image_ref(4, 4, &cursor_state, Some((0, 0)));

        let stride = 4 * 4;
        assert_eq!(pixel(&scratch, stride, 0, 0), [9, 9, 9, 255]);
        assert_eq!(pixel(&scratch, stride, 1, 0), [9, 9, 9, 255]);
        assert_eq!(pixel(&scratch, stride, 0, 1), [9, 9, 9, 255]);
        assert_eq!(pixel(&scratch, stride, 1, 1), [9, 9, 9, 255]);
        assert_eq!(pixel(&scratch, stride, 2, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&scratch, stride, 0, 2), [0, 0, 0, 0]);
    }

    #[test]
    fn image_ref_applies_hotspot_offset() {
        // Hotspot (1,1) at cursor (2,2) → bitmap top-left lands at (1,1).
        let mut cursor_state = CursorState::default();
        cursor_state.set_bitmap(&pointer(2, 2, 1, 1, [7, 7, 7, 255]));

        let (scratch, _) = run_image_ref(4, 4, &cursor_state, Some((2, 2)));

        let stride = 4 * 4;
        assert_eq!(pixel(&scratch, stride, 1, 1), [7, 7, 7, 255]);
        assert_eq!(pixel(&scratch, stride, 2, 1), [7, 7, 7, 255]);
        assert_eq!(pixel(&scratch, stride, 1, 2), [7, 7, 7, 255]);
        assert_eq!(pixel(&scratch, stride, 2, 2), [7, 7, 7, 255]);
        assert_eq!(pixel(&scratch, stride, 0, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&scratch, stride, 3, 3), [0, 0, 0, 0]);
    }

    #[test]
    fn image_ref_with_cursor_off_screen_leaves_data_unchanged() {
        // Cursor coords are u16 (>=0), but a large hotspot can push the bitmap
        // origin fully off the top-left.
        let mut cursor_state = CursorState::default();
        cursor_state.set_bitmap(&pointer(2, 2, 10, 10, [5, 5, 5, 255]));

        let (scratch, _) = run_image_ref(4, 4, &cursor_state, Some((0, 0)));

        let bytes = 4 * 4 * 4;
        assert_eq!(&scratch[..bytes], &vec![0u8; bytes][..]);
    }

    #[test]
    fn image_ref_clips_cursor_extending_past_right_and_bottom() {
        let mut cursor_state = CursorState::default();
        cursor_state.set_bitmap(&pointer(3, 3, 0, 0, [4, 4, 4, 255]));

        // Place at (3,3) on a 4x4 image: only the top-left pixel of the cursor lands in (3,3).
        let (scratch, _) = run_image_ref(4, 4, &cursor_state, Some((3, 3)));

        let stride = 4 * 4;
        assert_eq!(pixel(&scratch, stride, 3, 3), [4, 4, 4, 255]);
        assert_eq!(pixel(&scratch, stride, 2, 3), [0, 0, 0, 0]);
        assert_eq!(pixel(&scratch, stride, 3, 2), [0, 0, 0, 0]);
    }
}
