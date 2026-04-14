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

//! C FFI layer for the verified access list logic.
//!
//! This module provides a C-compatible interface that can be called from Go via CGO.
//! Input data is passed as JSON strings for simplicity.

use std::ffi::CStr;
use std::os::raw::c_char;

use crate::{Requires, UserInfo};

/// FFI-compatible input combining user info and requirements.
#[derive(serde::Deserialize)]
struct FFIInput {
    user: UserInfo,
    requires: Requires,
}

/// Checks whether a user meets the given access list requirements.
///
/// # Arguments
/// * `input_json` - A null-terminated C string containing JSON with the following structure:
///   ```json
///   {
///     "user": { "roles": ["role1"], "traits": [{"key": "k", "values": ["v1"]}] },
///     "requires": { "roles": ["role1"], "traits": [{"key": "k", "values": ["v1"]}] }
///   }
///   ```
///
/// # Returns
/// * `1` if the user meets the requirements
/// * `0` if the user does not meet the requirements
/// * `-1` if the input JSON is invalid
#[no_mangle]
pub extern "C" fn verified_user_meets_requirements(input_json: *const c_char) -> i32 {
    if input_json.is_null() {
        return -1;
    }

    let c_str = unsafe { CStr::from_ptr(input_json) };
    let json_str = match c_str.to_str() {
        Ok(s) => s,
        Err(_) => return -1,
    };

    let input: FFIInput = match serde_json::from_str(json_str) {
        Ok(v) => v,
        Err(_) => return -1,
    };

    if crate::user_meets_requirements(&input.user, &input.requires) {
        1
    } else {
        0
    }
}
