// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

// default trait not supported in wasm
#![allow(clippy::new_without_default)]

use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_pdu::cursor::ReadCursor;
use ironrdp_pdu::decode_cursor;
use ironrdp_pdu::fast_path::UpdateCode::{Bitmap, SurfaceCommands};
use ironrdp_pdu::fast_path::{FastPathHeader, FastPathUpdatePdu};
use ironrdp_pdu::geometry::{InclusiveRectangle, Rectangle};
use ironrdp_pdu::write_buf::WriteBuf;
use ironrdp_session::fast_path::UpdateKind;
use ironrdp_session::image::DecodedImage;
use ironrdp_session::ActiveStageOutput;
use ironrdp_session::{
    fast_path::Processor as IronRdpFastPathProcessor,
    fast_path::ProcessorBuilder as IronRdpFastPathProcessorBuilder,
};
use js_sys::Uint8Array;
use log::{debug, warn};
use wasm_bindgen::{prelude::*, Clamped};
use web_sys::ImageData;

#[wasm_bindgen]
pub fn init_wasm_log(log_level: &str) {
    use tracing::Level;
    use tracing_subscriber::filter::LevelFilter;
    use tracing_subscriber::fmt::time::UtcTime;
    use tracing_subscriber::prelude::*;
    use tracing_web::MakeConsoleWriter;

    // When the `console_error_panic_hook` feature is enabled, we can call the
    // `set_panic_hook` function at least once during initialization, and then
    // we will get better error messages if our code ever panics.
    //
    // For more details see
    // https://github.com/rustwasm/console_error_panic_hook#readme
    console_error_panic_hook::set_once();

    if let Ok(level) = log_level.parse::<Level>() {
        let fmt_layer = tracing_subscriber::fmt::layer()
            .with_ansi(false)
            .with_timer(UtcTime::rfc_3339()) // std::time is not available in browsers
            .with_writer(MakeConsoleWriter);

        let level_filter = LevelFilter::from_level(level);

        tracing_subscriber::registry()
            .with(fmt_layer)
            .with(level_filter)
            .init();

        debug!("WASM log is ready");
    }
}

#[wasm_bindgen]
pub struct BitmapFrame {
    top: u16,
    left: u16,
    image_data: ImageData,
}

#[wasm_bindgen]
impl BitmapFrame {
    #[wasm_bindgen(getter)]
    pub fn top(&self) -> u16 {
        self.top
    }

    #[wasm_bindgen(getter)]
    pub fn left(&self) -> u16 {
        self.left
    }

    #[wasm_bindgen(getter)]
    pub fn image_data(&self) -> ImageData {
        self.image_data.clone() // TODO(isaiah): can we remove this clone?
    }
}

fn create_image_data_from_image_and_region(
    image_data: &[u8],
    image_location: InclusiveRectangle,
) -> Result<ImageData, JsValue> {
    ImageData::new_with_u8_clamped_array_and_sh(
        Clamped(image_data),
        image_location.width().into(),
        image_location.height().into(),
    )
}

#[wasm_bindgen]
pub struct FastPathProcessor {
    fast_path_processor: IronRdpFastPathProcessor,
    image: DecodedImage,
    remote_fx_check_required: bool,
}

#[wasm_bindgen]
impl FastPathProcessor {
    #[wasm_bindgen(constructor)]
    pub fn new(width: u16, height: u16, io_channel_id: u16, user_channel_id: u16) -> Self {
        Self {
            fast_path_processor: IronRdpFastPathProcessorBuilder {
                io_channel_id,
                user_channel_id,
                // These should be set to the same values as they're set to in the
                // `Config` object in lib/srv/desktop/rdp/rdpclient/src/client.rs.
                no_server_pointer: false,
                pointer_software_rendering: false,
            }
            .build(),
            image: DecodedImage::new(PixelFormat::RgbA32, width, height),
            remote_fx_check_required: true,
        }
    }

    pub fn resize(&mut self, width: u16, height: u16) -> Result<(), JsValue> {
        self.image = DecodedImage::new(PixelFormat::RgbA32, width, height);
        Ok(())
    }

    /// `tdp_fast_path_frame: Uint8Array`
    ///
    /// `cb_context: tdp.Client`
    ///
    /// `draw_cb: (bitmapFrame: BitmapFrame) => void`
    ///
    /// `respond_cb: (responseFrame: ArrayBuffer) => void`
    ///
    /// If `data` is `false` we hide the cursor (but remember its value), if `data` is `true` we restore
    /// the last cursor value; otherwise we set the cursor to a bitmap from `ImageData`.
    /// `update_pointer_cb: (data: ImageData | boolean, hotspot_x: number, hotspot_y: number) => void`
    pub fn process(
        &mut self,
        tdp_fast_path_frame: &[u8],
        cb_context: &JsValue,
        draw_cb: &js_sys::Function,
        respond_cb: &js_sys::Function,
        update_pointer_cb: &js_sys::Function,
    ) -> Result<(), JsValue> {
        self.check_remote_fx(tdp_fast_path_frame)?;

        let (rdp_responses, client_updates) = {
            let mut output = WriteBuf::new();

            let processor_updates = self
                .fast_path_processor
                .process(&mut self.image, tdp_fast_path_frame, &mut output)
                .map_err(|e| JsValue::from_str(&format!("{:?}", e)))?;

            (output.into_inner(), processor_updates)
        };

        let outputs = {
            let mut outputs = Vec::new();

            if !rdp_responses.is_empty() {
                outputs.push(ActiveStageOutput::ResponseFrame(rdp_responses));
            }

            for update in client_updates {
                match update {
                    UpdateKind::None => {}
                    UpdateKind::Region(region) => {
                        outputs.push(ActiveStageOutput::GraphicsUpdate(region));
                    }
                    UpdateKind::PointerDefault => {
                        outputs.push(ActiveStageOutput::PointerDefault);
                    }
                    UpdateKind::PointerHidden => {
                        outputs.push(ActiveStageOutput::PointerHidden);
                    }
                    UpdateKind::PointerPosition { .. } => {
                        warn!("Pointer position updates are not supported");
                        continue;
                    }
                    UpdateKind::PointerBitmap(pointer) => {
                        outputs.push(ActiveStageOutput::PointerBitmap(pointer))
                    }
                }
            }

            outputs
        };

        for output in outputs {
            match output {
                ActiveStageOutput::GraphicsUpdate(updated_region) => {
                    // Apply the updated region to the canvas.
                    let (image_location, image_data) =
                        extract_partial_image(&self.image, updated_region);
                    self.apply_image_to_canvas(image_data, image_location, cb_context, draw_cb)?;
                }
                ActiveStageOutput::ResponseFrame(frame) => {
                    // Send the response frame back to the server.
                    let frame = Uint8Array::from(frame.as_slice()); // todo(isaiah): this is a copy
                    let _ = respond_cb.call1(cb_context, &frame.buffer())?;
                }
                ActiveStageOutput::Terminate(_) => {
                    return Err(JsValue::from_str("Terminate should never be returned"));
                }
                ActiveStageOutput::PointerBitmap(pointer) => {
                    let data = &pointer.bitmap_data;
                    let image_data = create_image_data_from_image_and_region(
                        data,
                        InclusiveRectangle {
                            left: 0,
                            top: 0,
                            right: pointer.width - 1,
                            bottom: pointer.height - 1,
                        },
                    )?;
                    update_pointer_cb.call3(
                        cb_context,
                        &JsValue::from(image_data),
                        &JsValue::from(pointer.hotspot_x),
                        &JsValue::from(pointer.hotspot_y),
                    )?;
                }
                ActiveStageOutput::PointerDefault => {
                    update_pointer_cb.call1(cb_context, &JsValue::from(true))?;
                }
                ActiveStageOutput::PointerHidden => {
                    update_pointer_cb.call1(cb_context, &JsValue::from(false))?;
                }
                _ => {
                    debug!("Unhandled ActiveStageOutput: {:?}", output);
                }
            }
        }

        Ok(())
    }

    /// check_remote_fx check if each fast path frame is RemoteFX frame, if we find bitmap frame
    /// (i.e. RemoteFX is not enabled on the server) we return error with helpful message
    fn check_remote_fx(&mut self, tdp_fast_path_frame: &[u8]) -> Result<(), JsValue> {
        if !self.remote_fx_check_required {
            return Ok(());
        }

        // we have to, at least partially, parse frame to check update code,
        // code here is copied from fast_path::Processor::process
        let mut input = ReadCursor::new(tdp_fast_path_frame);
        decode_cursor::<FastPathHeader>(&mut input)
            .map_err(|e| JsValue::from_str(&format!("{:?}", e)))?;
        let update_pdu = decode_cursor::<FastPathUpdatePdu<'_>>(&mut input)
            .map_err(|e| JsValue::from_str(&format!("{:?}", e)))?;

        match update_pdu.update_code {
            SurfaceCommands => {
                self.remote_fx_check_required = false;
                Ok(())
            }
            Bitmap => Err(JsValue::from_str(concat!(
                "Teleport requires the RemoteFX codec for Windows desktop sessions, ",
                "but it is not currently enabled. For detailed instructions, see:\n",
                "https://goteleport.com/docs/ver/15.x/desktop-access/active-directory-manual/#enable-remotefx"
            ))),
            _ => Ok(()),
        }
    }

    fn apply_image_to_canvas(
        &self,
        image_data: Vec<u8>,
        image_location: InclusiveRectangle,
        cb_context: &JsValue,
        callback: &js_sys::Function,
    ) -> Result<(), JsValue> {
        let top = image_location.top;
        let left = image_location.left;

        let image_data = create_image_data_from_image_and_region(&image_data, image_location)?;
        let bitmap_frame = BitmapFrame {
            top,
            left,
            image_data,
        };

        let bitmap_frame = &JsValue::from(bitmap_frame);

        // TODO(isaiah): return this?
        let _ret = callback.call1(cb_context, bitmap_frame)?;
        Ok(())
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
