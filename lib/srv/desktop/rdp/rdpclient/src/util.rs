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

use crate::errors::invalid_data_error;
use rdp::model::error::RdpResult;
use std::convert::TryFrom;
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
