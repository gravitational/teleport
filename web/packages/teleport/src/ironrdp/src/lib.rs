#[macro_use]
extern crate log;
extern crate bytes;
extern crate console_log;
extern crate js_sys;
extern crate wasm_bindgen;

use bytes::Bytes;
use js_sys::{ArrayBuffer, Uint8Array};
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub fn init_ironrdp(log_level: &str) {
    if let Ok(level) = log_level.parse::<log::Level>() {
        console_log::init_with_level(level).unwrap();
    }
}

#[wasm_bindgen]
pub fn process_buffer(buffer: ArrayBuffer) {
    let byte_array = Uint8Array::new(&buffer);
    let bytes = Bytes::from(byte_array.to_vec());
    debug!("bytes: {:?}", bytes);
}
