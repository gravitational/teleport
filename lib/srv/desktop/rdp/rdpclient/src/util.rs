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

use crate::errors::invalid_data_error;
use rdp::model::error::RdpResult;
use std::convert::TryFrom;
use std::ffi::{CStr, CString, NulError};
use std::io;
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
pub fn from_unicode(s: Vec<u8>) -> RdpResult<String> {
    let mut with_null_terminator = WString::from_utf16le(s)
        .or_else(|_| Err(invalid_data_error("invalid Unicode")))?
        .to_utf8();
    with_null_terminator.pop();
    let without_null_terminator = with_null_terminator;
    Ok(without_null_terminator)
}

/// Converts a &str into a null-terminated UTF-8 encoded Vec<u8>
pub fn to_utf8(s: &str) -> Vec<u8> {
    format!("{s}\x00").into_bytes()
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

// Taken from ironrdp_tls
pub fn extract_tls_server_public_key(cert: &[u8]) -> std::io::Result<Vec<u8>> {
    use x509_cert::der::Decode as _;

    let cert = x509_cert::Certificate::from_der(cert)
        .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))?;

    let server_public_key = cert
        .tbs_certificate
        .subject_public_key_info
        .subject_public_key
        .as_bytes()
        .ok_or_else(|| {
            std::io::Error::new(
                std::io::ErrorKind::Other,
                "subject public key BIT STRING is not aligned",
            )
        })?
        .to_owned();

    Ok(server_public_key)
}

/// Takes an fd and hangs until it's to be ready to read from.
#[cfg(unix)]
pub fn hang_until_read_ready(fd: i32) -> io::Result<()> {
    let fds = &mut libc::pollfd {
        fd,
        events: libc::POLLIN,
        revents: 0,
    };
    loop {
        let res = unsafe { libc::poll(fds, 1, -1) };

        // We only use a single fd and can't timeout, so
        // res will either be 1 for success or -1 for failure.
        if res != 1 {
            let os_err = std::io::Error::last_os_error();
            match os_err.raw_os_error() {
                Some(libc::EINTR) | Some(libc::EAGAIN) => continue,
                _ => return Err(os_err),
            }
        }

        // res == 1
        // POLLIN means that the fd is ready to be read from,
        // POLLHUP means that the other side of the pipe was closed,
        // but we still may have data to read.
        if fds.revents & (libc::POLLIN | libc::POLLHUP) != 0 {
            return Ok(()); // ready for a read
        } else if fds.revents & libc::POLLNVAL != 0 {
            return Err(io::Error::new(io::ErrorKind::InvalidInput, "invalid fd"));
        } else {
            return Err(io::Error::new(io::ErrorKind::Other, "error on fd"));
        }
    }
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
