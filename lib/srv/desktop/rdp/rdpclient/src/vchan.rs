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
use crate::Payload;
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use rdp::model::error::*;

bitflags! {
    pub struct ChannelPDUFlags: u32 {
        const CHANNEL_FLAG_FIRST = 0x00000001;
        const CHANNEL_FLAG_LAST = 0x00000002;
        const CHANNEL_FLAG_ONLY = Self::CHANNEL_FLAG_FIRST.bits | Self::CHANNEL_FLAG_LAST.bits;
    }
}

/// Channel PDU header precedes all static virtual channel traffic
/// transmitted between an RDP client and server.
///
/// It is specified in section 2.2.6.1.1 of MS-RDPBCGR.
#[derive(Debug)]
pub struct ChannelPDUHeader {
    length: u32,
    flags: ChannelPDUFlags,
}

impl ChannelPDUHeader {
    pub fn new(length: u32) -> Self {
        Self {
            length,
            flags: ChannelPDUFlags::CHANNEL_FLAG_ONLY,
        }
    }
    pub fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            length: payload.read_u32::<LittleEndian>()?,
            flags: ChannelPDUFlags::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("invalid flags in ChannelPDUHeader"))?,
        })
    }
    pub fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.flags.bits())?;
        Ok(w)
    }
}
