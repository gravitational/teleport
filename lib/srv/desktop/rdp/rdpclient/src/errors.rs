// Copyright 2021 Gravitational, Inc
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

use rdp::model::error::*;

// Helper function to return an unmarshaling error.
pub fn invalid_data_error(msg: &str) -> Error {
    Error::RdpError(RdpError::new(RdpErrorKind::InvalidData, msg))
}

pub fn not_implemented_error(msg: &str) -> Error {
    Error::RdpError(RdpError::new(RdpErrorKind::NotImplemented, msg))
}

// NTSTATUS_OK is a Windows NTStatus value that means "success".
pub const NTSTATUS_OK: u32 = 0;
// SPECIAL_NO_RESPONSE is our custom (not defined by Windows) NTStatus value that means "don't send
// a response to this message". Used in scard.rs.
pub const SPECIAL_NO_RESPONSE: u32 = 0xffffffff;
