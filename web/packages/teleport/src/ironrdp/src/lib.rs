#![allow(clippy::new_without_default)] // default trait not supported in wasm

#[macro_use]
extern crate log;
extern crate byteorder;
extern crate bytes;
extern crate console_log;
extern crate ironrdp;
extern crate js_sys;
extern crate wasm_bindgen;
extern crate web_sys;

use ironrdp::graphics::image_processing::PixelFormat;
use ironrdp::pdu::geometry::Rectangle;
use ironrdp::session::image::DecodedImage;
use ironrdp::session::{
    fast_path::Processor as IronRdpFastPathProcessor,
    fast_path::ProcessorBuilder as IronRdpFastPathProcessorBuilder, ActiveStageOutput,
};
use js_sys::Uint8Array;
use std::convert::TryFrom;
use wasm_bindgen::{prelude::*, Clamped};
use web_sys::ImageData;

#[wasm_bindgen]
pub fn init_wasm_log(log_level: &str) {
    error!("log_level: {}", log_level);
    if let Ok(level) = log_level.parse::<log::Level>() {
        error!("level: {}", level);
        console_log::init_with_level(level).unwrap();
    } else {
        error!("invalid log level: {}", log_level);
    }
}

#[wasm_bindgen]
pub struct FastPathFrame {
    rpc_id: u32,
    data: Uint8Array,
}

#[wasm_bindgen]
impl FastPathFrame {
    #[wasm_bindgen(constructor)]
    pub fn new(rpc_id: u32, data: Uint8Array) -> Self {
        Self { rpc_id, data }
    }
}

struct RustFastPathFrame {
    rpc_id: u32,
    data: Vec<u8>,
}

impl From<FastPathFrame> for RustFastPathFrame {
    fn from(js_frame: FastPathFrame) -> Self {
        Self {
            rpc_id: js_frame.rpc_id,
            data: js_frame.data.to_vec(), // TODO(isaiah): is it possible to avoid copy?
        }
    }
}

#[wasm_bindgen]
struct BitmapFrame {
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
        self.image_data.clone() // todo(isaiah): bad

        // You can pass the `&[u8]` from Rust to JavaScript without copying it by using the `wasm_bindgen::memory` function to directly access the WebAssembly linear memory. Here's how you can achieve this:

        // 1. Get a pointer to the data and its length.
        // 2. Create a `Uint8Array` that directly refers to the WebAssembly linear memory.
        // 3. Use the `subarray` method to create a new view that refers to the desired data without copying it.

        // ```rust
        // #[wasm_bindgen(getter)]
        // pub fn image_data(&self) -> JsValue {
        //     let data = self.image_data.data();
        //     let data_ptr = data.as_ptr() as u32;
        //     let data_len = data.len() as u32;

        //     let memory_buffer = wasm_bindgen::memory()
        //         .dyn_into::<WebAssembly::Memory>()
        //         .unwrap()
        //         .buffer();

        //     let data_array = js_sys::Uint8Array::new(&memory_buffer).subarray(data_ptr, data_ptr + data_len);

        //     let obj = js_sys::Object::new();
        //     js_sys::Reflect::set(&obj, &"data".into(), &data_array).unwrap();
        //     js_sys::Reflect::set(&obj, &"width".into(), &JsValue::from(self.image_data.width())).unwrap();
        //     js_sys::Reflect::set(&obj, &"height".into(), &JsValue::from(self.image_data.height())).unwrap();

        //     obj.into()
        // }
        // ```

        // This implementation should pass the data from Rust to JavaScript without copying it. Note that the returned `Uint8Array` is a view over the WebAssembly linear memory, so you need to make sure that the data is not modified on the Rust side while it's being used in JavaScript. Also, keep in mind that the lifetime of the `Uint8Array` is tied to the lifetime of the `ImageData` object in Rust. If the `ImageData` object is dropped, the underlying data may be deallocated, and the `Uint8Array` in JavaScript may become invalid.
    }
}

fn create_image_data_from_image_and_region(
    image_data: &[u8],
    image_location: Rectangle,
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
}

#[wasm_bindgen]
impl FastPathProcessor {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Self {
        Self {
            fast_path_processor: IronRdpFastPathProcessorBuilder {
                io_channel_id: 0,   // todo(isaiah)
                user_channel_id: 0, // todo(isaiah)
            }
            .build(),
            image: DecodedImage::new(
                PixelFormat::RgbA32,
                1728, //todo(isaiah): hardcoded
                932,  //todo(isaiah): hardcoded
            ),
        }
    }

    // tdp_fast_path_frame = | message type (29) | rpc_id uint32 | data_length uint32 | data []byte |
    pub fn process(
        &mut self,
        tdp_fast_path_frame: FastPathFrame,
        callback_context: &JsValue,
        callback: &js_sys::Function,
    ) -> Result<(), JsValue> {
        let mut output = Vec::new();
        let tdp_fast_path_frame: RustFastPathFrame = tdp_fast_path_frame.into();

        let graphics_update_region = self
            .fast_path_processor
            .process(&mut self.image, &tdp_fast_path_frame.data, &mut output)
            .map_err(|e| JsValue::from_str(&format!("{:?}", e)))?;

        let mut fast_path_outputs = Vec::new();

        if !output.is_empty() {
            fast_path_outputs.push(ActiveStageOutput::ResponseFrame(output));
        }

        if let Some(update_region) = graphics_update_region {
            fast_path_outputs.push(ActiveStageOutput::GraphicsUpdate(update_region));
        }

        for out in fast_path_outputs {
            match out {
                ActiveStageOutput::GraphicsUpdate(updated_region) => {
                    let (image_location, image_data) =
                        extract_partial_image(&self.image, updated_region);
                    self.apply_image_to_canvas(
                        image_data,
                        image_location,
                        callback_context,
                        callback,
                    )?;
                }
                ActiveStageOutput::ResponseFrame(_) => todo!(), // TODO(isaiah): must be sent back to server over TDP
                ActiveStageOutput::Terminate => {
                    return Err(JsValue::from_str("Terminate should never be returned"));
                }
            }
        }

        Ok(())
    }

    fn apply_image_to_canvas(
        &self,
        image_data: Vec<u8>,
        image_location: Rectangle,
        callback_context: &JsValue,
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
        debug!("bitmap_frame: {:?}", bitmap_frame);

        let _ret = callback.call1(callback_context, bitmap_frame)?;
        Ok(())
    }
}

pub fn extract_partial_image(image: &DecodedImage, region: Rectangle) -> (Rectangle, Vec<u8>) {
    // PERF: needs actual benchmark to find a better heuristic
    if region.height() > 64 || region.width() > 512 {
        extract_whole_rows(image, region)
    } else {
        extract_smallest_rectangle(image, region)
    }
}

// Faster for low-height and smaller images
fn extract_smallest_rectangle(image: &DecodedImage, region: Rectangle) -> (Rectangle, Vec<u8>) {
    let pixel_size = usize::from(image.pixel_format().bytes_per_pixel());

    let image_width = usize::try_from(image.width()).unwrap();
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

// Faster for high-height and bigger images
fn extract_whole_rows(image: &DecodedImage, region: Rectangle) -> (Rectangle, Vec<u8>) {
    let pixel_size = usize::from(image.pixel_format().bytes_per_pixel());

    let image_width = usize::try_from(image.width()).unwrap();
    let image_stride = image_width * pixel_size;

    let region_top = usize::from(region.top);
    let region_bottom = usize::from(region.bottom);

    let src = image.data();

    let src_begin = region_top * image_stride;
    let src_end = (region_bottom + 1) * image_stride;

    let dst = src[src_begin..src_end].to_vec();

    let wider_region = Rectangle {
        left: 0,
        top: region.top,
        right: image.width() - 1,
        bottom: region.bottom,
    };

    (wider_region, dst)
}
