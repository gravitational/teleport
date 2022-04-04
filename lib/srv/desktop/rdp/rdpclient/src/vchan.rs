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
use rdp::core::tpkt;
use rdp::model::error::*;
use rdp::try_let;
use std::io::{Cursor, Read};

/// Client is a general client for handling virtual channel payloads.
/// Its read method can read an RDP message sent in multiple chunks
/// (or a single chunk) over a virtual channel.
/// See https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/343e4888-4c48-4054-b0e3-4e0762d1993c
/// for more information about chunks.
pub struct Client {
    data: Vec<u8>,
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

impl Client {
    pub fn new() -> Self {
        Self { data: Vec::new() }
    }

    /// Callers can call read() to process RDP messages (PDUs) sent over a virtual channel.
    ///
    /// For chunked PDUs, the Client will piece the full PDU together in Client.data over multiple calls,
    /// and will only return an Ok(Some(Payload)) once a full message has been pieced together.
    ///
    /// The Payload will be the raw bytes of the PDU, starting at the channel specific header.
    /// For example, if handling a cliprdr PDU, Payload will be a full PDU starting with the
    /// CLIPRDR_HEADER structure that's is present in all clipboard PDUs.
    ///
    /// Returns Ok(None) on interim chunks.
    pub fn read(&mut self, raw_payload: tpkt::Payload) -> RdpResult<Option<Payload>> {
        let mut raw_payload = try_let!(tpkt::Payload::Raw, raw_payload)?;
        let channel_pdu_header = ChannelPDUHeader::decode(&mut raw_payload)?;

        raw_payload.read_to_end(&mut self.data)?;

        if channel_pdu_header
            .flags
            .contains(ChannelPDUFlags::CHANNEL_FLAG_LAST)
        {
            return Ok(Some(Cursor::new(self.data.split_off(0))));
        }

        Ok(None)
    }

    /// add_header_and_chunkify takes an encoded PDU ready to be sent over a virtual channel (payload),
    /// adds the appropriate (virtual) Channel PDU Header, and splits it into chunks if the payload exceeds
    /// the maximum size. The caller may optionally provide any any non-chunk-related Channel PDU Header
    /// flags that should be set. "Non-chunk-related" means any flags besides CHANNEL_FLAG_FIRST and CHANNEL_FLAG_LAST, which
    /// are handled by this function automatically.
    pub fn add_header_and_chunkify(
        &self,
        channel_flags: Option<ChannelPDUFlags>,
        payload: Vec<u8>,
    ) -> RdpResult<Vec<Vec<u8>>> {
        let mut inner = payload;
        let total_len = inner.len() as u32;

        let mut result = Vec::new();
        let mut first = true;
        while !inner.is_empty() {
            let i = std::cmp::min(inner.len(), CHANNEL_CHUNK_LEGNTH);
            let leftover = inner.split_off(i);

            let mut channel_flags = channel_flags.unwrap_or(ChannelPDUFlags::from_bits_truncate(0));

            if first {
                channel_flags.set(ChannelPDUFlags::CHANNEL_FLAG_FIRST, true);
                first = false;
            }
            if leftover.is_empty() {
                channel_flags.set(ChannelPDUFlags::CHANNEL_FLAG_LAST, true);
            }

            // the Channel PDU Header always specifies the *total length* of the PDU,
            // even if it has to be split into multpile chunks:
            // https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/a542bf19-1c86-4c80-ab3e-61449653abf6
            let mut outer = ChannelPDUHeader::new(total_len, channel_flags).encode()?;
            outer.extend(inner);
            result.push(outer);

            inner = leftover;
        }

        Ok(result)
    }
}

/// The default maximum chunk size for virtual channel data.
///
/// If an RDP server supports larger chunks, it will advertise
/// the larger chunk size in the `VCChunkSize` field of the
/// virtual channel capability set.
///
/// See also:
/// - https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/6c074267-1b32-4ceb-9496-2eb941a23e6b
/// - https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/a8593178-80c0-4b80-876c-cb77e62cecfc
pub const CHANNEL_CHUNK_LEGNTH: usize = 1600;

bitflags! {
    /// Channel control flags, as specified in section 2.2.6.1.1 of MS-RDPBCGR.
    ///
    /// See: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/f125c65e-6901-43c3-8071-d7d5aaee7ae4
    pub struct ChannelPDUFlags: u32 {
        const CHANNEL_FLAG_FIRST = 0x00000001;
        const CHANNEL_FLAG_LAST = 0x00000002;
        const CHANNEL_FLAG_SHOW_PROTOCOL = 0x00000010;
        const CHANNEL_FLAG_SUSPEND = 0x00000020;
        const CHANNEL_FLAG_RESUME = 0x00000040;
        const CHANNEL_FLAG_SHADOW_PERSISTENT = 0x00000080;
        const CHANNEL_PACKET_COMPRESSED = 0x00200000;
        const CHANNEL_PACKET_AT_FRONT = 0x00400000;
        const CHANNEL_PACKET_FLUSHED = 0x00800000;

        const CHANNEL_FLAG_ONLY = Self::CHANNEL_FLAG_FIRST.bits | Self::CHANNEL_FLAG_LAST.bits;
    }
}

/// Channel PDU header precedes all static virtual channel traffic
/// transmitted between an RDP client and server.
///
/// It is specified in section 2.2.6.1.1 of MS-RDPBCGR.
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/f125c65e-6901-43c3-8071-d7d5aaee7ae4
#[derive(Debug)]
pub struct ChannelPDUHeader {
    /// The total length of the uncompressed PDU data,
    /// excluding the length of this header.
    /// Note: the data can span multiple PDUs, in which
    /// case each PDU in the series contains the same
    /// length field.
    pub length: u32,
    pub flags: ChannelPDUFlags,
}

impl ChannelPDUHeader {
    pub fn new(length: u32, flags: ChannelPDUFlags) -> Self {
        Self { length, flags }
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
