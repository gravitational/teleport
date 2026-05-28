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
    /// `out_w` x `out_h`. The crop must lie within the current frame bounds.
    pub fn resize_crop_into(
        &mut self,
        (crop_x, crop_y): (u16, u16),
        (crop_w, crop_h): (u16, u16),
        (out_w, out_h): (u16, u16),
        dst: &mut [u8],
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
            .use_alpha(false)
            .crop(
                f64::from(crop_x),
                f64::from(crop_y),
                f64::from(crop_w),
                f64::from(crop_h),
            );

        let src = ImageRef::new(
            u32::from(src_w),
            u32::from(src_h),
            self.image.data(),
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
