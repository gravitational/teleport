// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

use ironrdp_pdu::{other_err, PduResult};
use std::convert::TryFrom;
use std::ffi::{CStr, CString, NulError};
use std::os::raw::c_char;
use std::slice;
use utf16string::{WString, LE};

/// According to [MS-RDPEFS] 1.1 Glossary:
/// Unless otherwise specified, all Unicode strings follow the UTF-16LE
/// encoding scheme with no Byte Order Mark (BOM).
///
/// This helper function takes a string slice and converts it to a
/// UTF-16LE encoded Vec<u8>, which is useful in cases where we want
/// to handle some data in the code as a &str (or String), and later
/// convert it to RDP's preferred format and send it over the wire.
pub fn to_unicode(s: &str, with_null_term: bool) -> Vec<u8> {
    let mut buf = WString::<LE>::from(s).as_bytes().to_vec();
    if with_null_term {
        let mut null_terminator: Vec<u8> = vec![0, 0];
        buf.append(&mut null_terminator);
    }
    buf
}

#[allow(clippy::bind_instead_of_map)]
pub fn from_unicode(s: Vec<u8>) -> PduResult<String> {
    let mut with_null_terminator = WString::from_utf16le(s)
        .or_else(|_| Err(other_err!("from_unicode", "invalid Unicode")))?
        .to_utf8();
    with_null_terminator.pop();
    let without_null_terminator = with_null_terminator;
    Ok(without_null_terminator)
}

/// Converts a &str into a null-terminated UTF-8 encoded Vec<u8>
pub fn to_utf8(s: &str) -> Vec<u8> {
    format!("{s}\x00").into_bytes()
}

pub fn from_utf8(s: Vec<u8>) -> PduResult<String> {
    let mut with_null_terminator =
        String::from_utf8(s).or_else(|_| Err(other_err!("from_utf8", "invalid Unicode")))?;
    with_null_terminator.pop();
    let without_null_terminator = with_null_terminator;
    Ok(without_null_terminator)
}

/// Takes a Rust string slice and calculates it's unicode size in bytes.
pub fn unicode_size(s: &str, with_null_term: bool) -> u32 {
    u32::try_from(to_unicode(s, with_null_term).len()).unwrap()
}

pub fn vec_u8_debug(v: &[u8]) -> String {
    format!("&[u8] of length {}", v.len())
}

/// to_c_string can be used to return string values over the Go boundary.
/// To avoid memory leaks, the Go function must call free_go_string once
/// it's done with the memory.
///
/// See https://doc.rust-lang.org/std/ffi/struct.CString.html#method.into_raw
pub fn to_c_string(s: &str) -> Result<*const c_char, NulError> {
    let c_string = CString::new(s)?;
    Ok(c_string.into_raw())
}

/// See the docstring for to_c_string.
///
/// # Safety
///
/// s must be a pointer originally created by to_c_string
#[no_mangle]
pub unsafe extern "C" fn free_c_string(s: *mut c_char) {
    // retake pointer to free memory
    let _ = CString::from_raw(s);
}

/// # Safety
///
/// s must be a C-style null terminated string.
/// s is cloned here, and the caller is responsible for
/// ensuring its memory is freed.
pub unsafe fn from_c_string(s: *const c_char) -> String {
    // # Safety
    //
    // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
    // In other words, all pointer data that needs to persist after this function returns MUST
    // be copied into Rust-owned memory.
    CStr::from_ptr(s).to_string_lossy().into_owned()
}

/// Creates a Vec from a Go (C) array without a copy.
///
/// # Safety
///
/// See https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html
pub unsafe fn from_go_array<T: Clone>(data: *const T, len: u32) -> Vec<T> {
    // # Safety
    //
    // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
    // In other words, all pointer data that needs to persist after this function returns MUST
    // be copied into Rust-owned memory.
    slice::from_raw_parts(data, len as usize).to_vec()
}

/// encodes png from the uncompressed bitmap data
///
/// # Arguments
///
/// * `dest` - buffer that will contain the png data
/// * `width` - width of the png
/// * `height` - height of the png
/// * `data` - buffer that contains uncompressed bitmap data
pub fn encode_png(
    dest: &mut Vec<u8>,
    width: u16,
    height: u16,
    data: Vec<u8>,
) -> Result<(), png::EncodingError> {
    let mut encoder = png::Encoder::new(dest, width as u32, height as u32);
    encoder.set_compression(png::Compression::Fast);
    encoder.set_color(png::ColorType::Rgba);

    let mut writer = encoder.write_header()?;
    writer.write_image_data(&data)?;
    writer.finish()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn to_and_from() {
        let hello_vec = to_unicode("hello", true);
        assert_eq!(
            hello_vec,
            vec![104, 0, 101, 0, 108, 0, 108, 0, 111, 0, 0, 0]
        );

        let hello_string = from_unicode(hello_vec).unwrap();
        assert_eq!(hello_string, "hello");
    }

    #[test]
    fn from_unicode_empty_vector() {
        assert_eq!(from_unicode(vec![]).unwrap(), "");
    }
}
