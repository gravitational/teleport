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

//! Simple RDP decoder that processes Fast Path PDUs

use ironrdp_core::decode_cursor;
use ironrdp_core::ReadCursor;
use ironrdp_core::WriteBuf;
use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_pdu::fast_path::UpdateCode::{Bitmap, SurfaceCommands};
use ironrdp_pdu::fast_path::{FastPathHeader, FastPathUpdatePdu};
use ironrdp_pdu::geometry::{InclusiveRectangle, Rectangle};
use ironrdp_session::fast_path::UpdateKind;
use ironrdp_session::image::DecodedImage;
use ironrdp_session::{
	fast_path::Processor as IronRdpFastPathProcessor,
	fast_path::ProcessorBuilder as IronRdpFastPathProcessorBuilder,
};
use log::warn;

pub struct RdpDecoder {
    pub width: u16,
    pub height: u16,
    fast_path_processor: IronRdpFastPathProcessor,
    image: DecodedImage,
    remote_fx_check_required: bool,
}

#[derive(Debug, Clone)]
pub struct FrameUpdate {
    pub x: u16,
    pub y: u16,
    pub width: u16,
    pub height: u16,
    pub data: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct PointerUpdate {
    pub width: u16,
    pub height: u16,
    pub hotspot_x: u16,
    pub hotspot_y: u16,
    pub bitmap_data: Vec<u8>,
}

#[derive(Debug)]
pub enum ProcessorOutput {
    GraphicsUpdate(FrameUpdate),
    ResponseFrame(Vec<u8>),
    PointerBitmap(PointerUpdate),
    PointerDefault,
    PointerHidden,
    PointerPosition { x: u16, y: u16 },
}

pub struct ProcessResult {
    pub outputs: Vec<ProcessorOutput>,
}

impl RdpDecoder {
    pub fn new(width: u16, height: u16, io_channel_id: u16, user_channel_id: u16) -> Self {
        let actual_width = if width == 0 { 1920 } else { width };
        let actual_height = if height == 0 { 1080 } else { height };

        Self {
            width: actual_width,
            height: actual_height,
            fast_path_processor: IronRdpFastPathProcessorBuilder {
                io_channel_id,
                user_channel_id,
                enable_server_pointer: true,
                pointer_software_rendering: false,
            }
            .build(),
            image: DecodedImage::new(PixelFormat::RgbA32, actual_width, actual_height),
            remote_fx_check_required: true,
        }
    }

    pub fn resize(&mut self, width: u16, height: u16) -> Result<(), String> {
        self.width = width;
        self.height = height;
        self.image = DecodedImage::new(PixelFormat::RgbA32, width, height);
        Ok(())
    }

    pub fn process(&mut self, tdp_fast_path_frame: &[u8]) -> Result<ProcessResult, String> {
        self.check_remote_fx(tdp_fast_path_frame)?;

        let mut output = WriteBuf::new();
        let client_updates = self
            .fast_path_processor
            .process(&mut self.image, tdp_fast_path_frame, &mut output)
            .unwrap();

        let rdp_responses = output.into_inner();

        let mut outputs = Vec::new();

        if !rdp_responses.is_empty() {
            outputs.push(ProcessorOutput::ResponseFrame(rdp_responses));
        }

        for update in client_updates {
            match update {
                UpdateKind::None => {}
                UpdateKind::Region(region) => {
                    let needs_resize = region.right >= self.width || region.bottom >= self.height;
                    if needs_resize {
                        let new_width = (region.right + 1).max(self.width);
                        let new_height = (region.bottom + 1).max(self.height);

                        if let Err(e) = self.resize(new_width, new_height) {
                            warn!(
                                "Failed to resize decoder to {}x{}: {}",
                                new_width, new_height, e
                            );
                        }
                    }

                    let (image_location, image_data) = extract_partial_image(&self.image, region);

                    let frame_update = FrameUpdate {
                        x: image_location.left,
                        y: image_location.top,
                        width: image_location.width(),
                        height: image_location.height(),
                        data: image_data,
                    };
                    outputs.push(ProcessorOutput::GraphicsUpdate(frame_update));
                }
                UpdateKind::PointerDefault => {
                    outputs.push(ProcessorOutput::PointerDefault);
                }
                UpdateKind::PointerHidden => {
                    outputs.push(ProcessorOutput::PointerHidden);
                }
                UpdateKind::PointerPosition { x, y } => {
                    outputs.push(ProcessorOutput::PointerPosition { x, y });
                }
                UpdateKind::PointerBitmap(pointer) => {
                    let pointer_update = PointerUpdate {
                        width: pointer.width,
                        height: pointer.height,
                        hotspot_x: pointer.hotspot_x,
                        hotspot_y: pointer.hotspot_y,
                        bitmap_data: pointer.bitmap_data.clone(),
                    };
                    outputs.push(ProcessorOutput::PointerBitmap(pointer_update));
                }
            }
        }

        Ok(ProcessResult { outputs })
    }

    /// check_remote_fx check if each fast path frame is RemoteFX frame, if we find bitmap frame
    /// (i.e. RemoteFX is not enabled on the server) we return error with helpful message
    fn check_remote_fx(&mut self, tdp_fast_path_frame: &[u8]) -> Result<(), String> {
        if !self.remote_fx_check_required {
            return Ok(());
        }

        // we have to, at least partially, parse frame to check update code,
        // code here is copied from fast_path::Processor::process
        let mut input = ReadCursor::new(tdp_fast_path_frame);
        decode_cursor::<FastPathHeader>(&mut input).map_err(|e| format!("{:?}", e))?;
        let update_pdu =
            decode_cursor::<FastPathUpdatePdu<'_>>(&mut input).map_err(|e| format!("{:?}", e))?;

        match update_pdu.update_code {
            SurfaceCommands => {
                self.remote_fx_check_required = false;
                Ok(())
            }
            Bitmap => Err(concat!(
                "Teleport requires the RemoteFX codec for Windows desktop sessions, ",
                "but it is not currently enabled. For detailed instructions, see:\n",
                "https://goteleport.com/docs/enroll-resources/desktop-access/active-directory/#enable-remotefx"
            ).to_string()),
            _ => Ok(()),
        }
    }
}

/// Taken from https://github.com/Devolutions/IronRDP/blob/35839459aa58c5c42cd686b39b63a7944285c0de/crates/ironrdp-web/src/image.rs#L6
pub fn extract_partial_image(
    image: &DecodedImage,
    region: InclusiveRectangle,
) -> (InclusiveRectangle, Vec<u8>) {
    // PERF: needs actual benchmark to find a better heuristic
    if region.height() > 64 || region.width() > 512 {
        extract_whole_rows(image, region)
    } else {
        extract_smallest_rectangle(image, region)
    }
}

/// Faster for low-height and smaller images
///
/// https://github.com/Devolutions/IronRDP/blob/35839459aa58c5c42cd686b39b63a7944285c0de/crates/ironrdp-web/src/image.rs#L16
fn extract_smallest_rectangle(
    image: &DecodedImage,
    region: InclusiveRectangle,
) -> (InclusiveRectangle, Vec<u8>) {
    let pixel_size = usize::from(image.pixel_format().bytes_per_pixel());

    let image_width = usize::from(image.width());
    let image_stride = image_width * pixel_size;

    let region_top = usize::from(region.top);
    let region_left = usize::from(region.left);
    let region_width = usize::from(region.width());
    let region_height = usize::from(region.height());
    let region_stride = region_width * pixel_size;

    let dst_buf_size = region_width * region_height * pixel_size;
    let mut dst = vec![0; dst_buf_size];

    let src = image.data();

    for row in 0..region_height {
        let src_begin = image_stride * (region_top + row) + region_left * pixel_size;
        let src_end = src_begin + region_stride;
        let src_slice = &src[src_begin..src_end];

        let target_begin = region_stride * row;
        let target_end = target_begin + region_stride;
        let target_slice = &mut dst[target_begin..target_end];

        target_slice.copy_from_slice(src_slice);
    }

    (region, dst)
}

/// Faster for high-height and bigger images
///
/// https://github.com/Devolutions/IronRDP/blob/35839459aa58c5c42cd686b39b63a7944285c0de/crates/ironrdp-web/src/image.rs#L49
fn extract_whole_rows(
    image: &DecodedImage,
    region: InclusiveRectangle,
) -> (InclusiveRectangle, Vec<u8>) {
    let pixel_size = usize::from(image.pixel_format().bytes_per_pixel());

    let image_width = usize::from(image.width());
    let image_stride = image_width * pixel_size;

    let region_top = usize::from(region.top);
    let region_bottom = usize::from(region.bottom);

    let src = image.data();

    let src_begin = region_top * image_stride;
    let src_end = (region_bottom + 1) * image_stride;

    let dst = src[src_begin..src_end].to_vec();

    let wider_region = InclusiveRectangle {
        left: 0,
        top: region.top,
        right: image.width() - 1,
        bottom: region.bottom,
    };

    (wider_region, dst)
}
