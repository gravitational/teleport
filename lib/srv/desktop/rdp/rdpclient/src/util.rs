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

use ironrdp_pdu::pdu_other_err;
use ironrdp_pdu::PduResult;
use std::ffi::CStr;
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
        .or_else(|_| Err(pdu_other_err!("invalid Unicode")))?
        .to_utf8();
    with_null_terminator.pop();
    let without_null_terminator = with_null_terminator;
    Ok(without_null_terminator)
}

pub fn from_utf8(s: Vec<u8>) -> PduResult<String> {
    let mut with_null_terminator =
        String::from_utf8(s).map_err(|_| pdu_other_err!("invalid Unicode"))?;
    with_null_terminator.pop();
    let without_null_terminator = with_null_terminator;
    Ok(without_null_terminator)
}

pub fn vec_u8_debug(v: &[u8]) -> String {
    format!("&[u8] of length {}", v.len())
}

pub fn str_debug(s: &str) -> String {
    format!("&str of length {}", s.len())
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
