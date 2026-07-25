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

use crate::RdpDecoder;
use fast_image_resize::images::{Image, ImageRef};
use fast_image_resize::{FilterType, PixelType, ResizeAlg, ResizeOptions};
use std::io::Error;

impl RdpDecoder {
    /// Writes a CatmullRom-resized copy of the source crop region into `dst`, scaled to
    /// `out_w` x `out_h`.
    /// When `with_cursor` is true and the tracked cursor is visible, it is composited onto the
    /// cropped pixels before the resize, so the cursor scales with the screen.
    ///
    /// The crop must lie within the current frame bounds.
    pub fn resize_crop_into(
        &mut self,
        (crop_x, crop_y): (u16, u16),
        (crop_w, crop_h): (u16, u16),
        (out_w, out_h): (u16, u16),
        dst: &mut [u8],
        with_cursor: bool,
    ) -> Result<(), Error> {
        let bpp = self.image.pixel_format().bytes_per_pixel();
        let pixel_type = match bpp {
            4 => PixelType::U8x4,
            _ => return Err(Error::other(format!("unsupported bytes per pixel: {bpp}"))),
        };

        let src_w = self.image.width();
        let src_h = self.image.height();
        if u32::from(crop_x) + u32::from(crop_w) > u32::from(src_w)
            || u32::from(crop_y) + u32::from(crop_h) > u32::from(src_h)
        {
            return Err(Error::other("crop extends past frame"));
        }

        let needed = (out_w as usize) * (out_h as usize) * usize::from(bpp);
        if dst.len() < needed {
            return Err(Error::other("destination buffer too small"));
        }

        let opts = ResizeOptions::new()
            .resize_alg(ResizeAlg::Convolution(FilterType::CatmullRom))
            .use_alpha(false);

        let composite_cursor = with_cursor && self.cursor_state.is_visible();

        let (src_buffer, src_pixel_w, src_pixel_h, opts) = if composite_cursor {
            let (cx, cy) = self.cursor_state.position();
            let cursor_bitmap = self.cursor_state.bitmap();

            let bpp_usize = usize::from(bpp);
            let src_stride = (src_w as usize) * bpp_usize;
            let crop_row_bytes = (crop_w as usize) * bpp_usize;
            let scratch_bytes = crop_row_bytes * (crop_h as usize);
            if self.composite_scratch.len() < scratch_bytes {
                self.composite_scratch.resize(scratch_bytes, 0);
            }

            let image_data = self.image.data();
            let crop_x_off = (crop_x as usize) * bpp_usize;
            for y in 0..(crop_h as usize) {
                let src_off = ((crop_y as usize) + y) * src_stride + crop_x_off;
                let dst_off = y * crop_row_bytes;
                self.composite_scratch[dst_off..dst_off + crop_row_bytes]
                    .copy_from_slice(&image_data[src_off..src_off + crop_row_bytes]);
            }

            let (cursor_hotspot_x, cursor_hotspot_y) = cursor_bitmap.hotspot();

            composite_over(
                &mut self.composite_scratch[..scratch_bytes],
                (crop_w, crop_h),
                cursor_bitmap.data(),
                cursor_bitmap.dimensions(),
                (
                    i32::from(cx) - i32::from(cursor_hotspot_x) - i32::from(crop_x),
                    i32::from(cy) - i32::from(cursor_hotspot_y) - i32::from(crop_y),
                ),
            );

            (
                &self.composite_scratch[..scratch_bytes],
                crop_w,
                crop_h,
                opts,
            )
        } else {
            (
                self.image.data(),
                src_w,
                src_h,
                opts.crop(
                    f64::from(crop_x),
                    f64::from(crop_y),
                    f64::from(crop_w),
                    f64::from(crop_h),
                ),
            )
        };

        let src = ImageRef::new(
            u32::from(src_pixel_w),
            u32::from(src_pixel_h),
            src_buffer,
            pixel_type,
        )
        .map_err(Error::other)?;

        let mut dst_image = Image::from_slice_u8(
            u32::from(out_w),
            u32::from(out_h),
            &mut dst[..needed],
            pixel_type,
        )
        .map_err(Error::other)?;

        self.resizer
            .resize(&src, &mut dst_image, &opts)
            .map_err(Error::other)?;

        Ok(())
    }
}

// Source-over blend of `bmp_pix` onto `dst` at (`draw_x`, `draw_y`).
// RGBA8-only: the per-pixel math assumes 4 channels with alpha at byte index 3.
fn composite_over(
    dst: &mut [u8],
    (dst_w, dst_h): (u16, u16),
    bmp_pix: &[u8],
    (bmp_w, bmp_h): (u16, u16),
    (draw_x, draw_y): (i32, i32),
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

        composite_over(&mut dst, (2, 2), &bmp, (2, 2), (0, 0));

        assert_eq!(dst, make_dst(2, 2, 200));
    }

    #[test]
    fn composite_over_opaque_source_overwrites_dst() {
        let mut dst = make_dst(2, 2, 200);
        let bmp = make_bmp(2, 2, [10, 20, 30, 255]);

        composite_over(&mut dst, (2, 2), &bmp, (2, 2), (0, 0));

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

        composite_over(&mut dst, (1, 1), &bmp, (1, 1), (0, 0));

        assert_eq!(pixel(&dst, 4, 0, 0), [149, 149, 149, 227]);
    }

    #[test]
    fn composite_over_skips_when_fully_off_screen() {
        let original = make_dst(2, 2, 200);
        let bmp = make_bmp(2, 2, [10, 20, 30, 255]);

        for (dx, dy) in [(-2, 0), (2, 0), (0, -2), (0, 2), (-5, -5), (10, 10)] {
            let mut dst = original.clone();
            composite_over(&mut dst, (2, 2), &bmp, (2, 2), (dx, dy));
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

        composite_over(&mut dst, (4, 1), &bmp, (4, 1), (-2, 0));

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

        composite_over(&mut dst, (1, 4), &bmp, (1, 4), (0, -2));

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

        composite_over(&mut dst, (4, 1), &bmp, (4, 1), (2, 0));

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

        composite_over(&mut dst, (1, 4), &bmp, (1, 4), (0, 2));

        assert_eq!(pixel(&dst, 4, 0, 0), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 4, 0, 1), [0, 0, 0, 0]);
        assert_eq!(pixel(&dst, 4, 0, 2), [1, 1, 1, 255]);
        assert_eq!(pixel(&dst, 4, 0, 3), [2, 2, 2, 255]);
    }
}
