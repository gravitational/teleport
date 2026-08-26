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

mod backend;
mod context;
mod piv;

pub(crate) use backend::ScardBackend;
use iso7816::Status;

pub(crate) const SCARD_DEVICE_ID: u32 = 1;
/// Maximum size of transmit request/response short data, in bytes.
const TRANSMIT_DATA_LIMIT: usize = 1024;

/// Represents a response returned by a smart card after processing an APDU command.
#[derive(Debug)]
struct Response {
    /// Optional response APDU.
    data: Option<Vec<u8>>,
    /// An APDU status.
    status: Status,
}

impl Response {
    /// Creates a new [`Response`] with the given status and no response APDU.
    fn new(status: Status) -> Self {
        Self { data: None, status }
    }

    /// Creates a new [`Response`] with the given status and response APDU.
    fn with_data(status: Status, data: Vec<u8>) -> Self {
        Self {
            data: Some(data),
            status,
        }
    }

    /// Encodes the [`Response`] into a byte vector.
    fn encode(&self) -> Vec<u8> {
        let mut buf = Vec::new();
        if let Some(data) = &self.data {
            buf.extend_from_slice(data);
        }
        let status: [u8; 2] = self.status.into();
        buf.extend_from_slice(&status);
        buf
    }
}
