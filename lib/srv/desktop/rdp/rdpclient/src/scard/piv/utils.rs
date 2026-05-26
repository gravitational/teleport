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

use iso7816::Command;
use std::fmt::Write as _;

pub(super) fn hex_data<const S: usize>(cmd: &Command<S>) -> String {
    to_hex(cmd.data())
}

pub(super) fn to_hex(bytes: &[u8]) -> String {
    let mut s = String::new();
    for b in bytes {
        // https://rust-lang.github.io/rust-clippy/master/index.html#format_push_string
        let _ = write!(s, "{b:02X}");
    }
    s
}

#[allow(clippy::cast_possible_truncation)]
pub(super) fn len_to_vec(len: usize) -> Vec<u8> {
    if len < 0x7f {
        vec![len as u8]
    } else {
        let mut ret: Vec<u8> = len
            .to_be_bytes()
            .iter()
            .skip_while(|&x| *x == 0)
            .cloned()
            .collect();
        ret.insert(0, 0x80 | ret.len() as u8);
        ret
    }
}
