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

pub fn try_error(msg: &str) -> Error {
    Error::TryError(msg.to_string())
}

pub fn rejected_by_server_error(msg: &str) -> Error {
    Error::RdpError(RdpError::new(RdpErrorKind::RejectedByServer, msg))
}
