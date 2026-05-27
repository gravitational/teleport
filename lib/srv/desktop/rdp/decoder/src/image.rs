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
use fast_image_resize::{PixelType, ResizeOptions, Resizer};
use ironrdp_session::image::DecodedImage;
use std::io::Error;

impl RdpDecoder {
    pub fn resize_image_into(
        &mut self,
        out_w: u16,
        out_h: u16,
        dst: &mut [u8],
        opts: &ResizeOptions,
    ) -> Result<(), Error> {
        resize_into(&self.image, &mut self.resizer, out_w, out_h, dst, opts)
    }
}

pub(crate) fn resize_into(
    image: &DecodedImage,
    resizer: &mut Resizer,
    out_w: u16,
    out_h: u16,
    dst: &mut [u8],
    opts: &ResizeOptions,
) -> Result<(), Error> {
    let bpp = image.pixel_format().bytes_per_pixel();
    let pixel_type = match bpp {
        4 => PixelType::U8x4,
        _ => return Err(Error::other(format!("unsupported bytes per pixel: {bpp}"))),
    };

    let needed = (out_w as usize) * (out_h as usize) * usize::from(bpp);
    if dst.len() < needed {
        return Err(Error::other("destination buffer too small"));
    }

    let src = ImageRef::new(
        u32::from(image.width()),
        u32::from(image.height()),
        image.data(),
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

    resizer
        .resize(&src, &mut dst_image, opts)
        .map_err(Error::other)?;

    Ok(())
}
