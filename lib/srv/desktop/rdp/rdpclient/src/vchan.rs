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

use crate::{errors::invalid_data_error, Message, Messages};
use crate::{Encode, Payload};
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
    // size_limit is the maximum size (in bytes) of an RDP
    // message we will receive from the RDP server, minus
    // any ChannelPDUHeaders.
    size_limit: usize,
    data: Vec<u8>,
    drop_current_message: bool,
}

impl Client {
    /// Client will drop all messages with length greater than the specified capacity.
    pub fn new(capacity: usize) -> Self {
        Self {
            size_limit: capacity,
            data: Vec::new(),
            drop_current_message: false,
        }
    }

    /// Callers can call read() to process RDP messages (PDUs) sent over a virtual channel.
    ///
    /// For chunked PDUs, the Client will piece the full PDU together in Client.data over multiple calls,
    /// and will only return an Ok(Some(Payload)) once a full message has been pieced together, presuming
    /// the total message does not exceed capacity. Messages that do exceed capacity will be
    /// dropped.
    ///
    /// The Payload will be the raw bytes of the PDU, starting at the channel specific header.
    /// For example, if handling a cliprdr PDU, Payload will be a full PDU starting with the
    /// CLIPRDR_HEADER structure that's is present in all clipboard PDUs.
    ///
    /// Returns Ok(None) on interim chunks.
    pub fn read(&mut self, raw_payload: tpkt::Payload) -> RdpResult<Option<Payload>> {
        let mut raw_payload = try_let!(tpkt::Payload::Raw, raw_payload)?;
        let channel_pdu_header = ChannelPDUHeader::decode(&mut raw_payload)?;
        debug!("got RDP: {:?}", channel_pdu_header);

        let this_chunk_size = raw_payload.get_ref().len() - raw_payload.position() as usize;
        let cumulative_message_size = self.data.len();

        if this_chunk_size + cumulative_message_size <= self.size_limit {
            raw_payload.read_to_end(&mut self.data)?;
        } else {
            self.drop_current_message = true;
        }

        if channel_pdu_header
            .flags
            .contains(ChannelPDUFlags::CHANNEL_FLAG_LAST)
        {
            if !self.drop_current_message {
                return Ok(Some(Cursor::new(self.data.split_off(0))));
            }
            warn!("RDP client received a message that exceeded the maximum allowed message size ({:?} bytes), message was dropped", self.size_limit);
            self.drop_current_message = false; // reset for the next message
            self.data.clear(); // clear the pending data
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
        payload: Message,
    ) -> RdpResult<Messages> {
        let mut inner = payload;
        let total_len = inner.len() as u32;

        let mut result = Vec::new();
        let mut first = true;
        while !inner.is_empty() {
            let i = std::cmp::min(inner.len(), CHANNEL_CHUNK_LEGNTH);
            let leftover = inner.split_off(i);

            let mut channel_flags =
                channel_flags.unwrap_or_else(|| ChannelPDUFlags::from_bits_truncate(0));

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
    #[derive(Debug, PartialEq, Copy, Clone)]
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

        const CHANNEL_FLAG_ONLY = Self::CHANNEL_FLAG_FIRST.bits() | Self::CHANNEL_FLAG_LAST.bits();
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
}

impl Encode for ChannelPDUHeader {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.flags.bits())?;
        Ok(w)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn drops_messages_exceeding_capacity() {
        let mut c = Client::new(3);

        // Create a first message that takes up the total capacity of the Client
        let mut first_message = ChannelPDUHeader {
            length: 4,
            flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST,
        }
        .encode()
        .unwrap();
        first_message.extend([1, 2, 3]);
        let first_message = tpkt::Payload::Raw(Cursor::new(first_message));

        let res = c.read(first_message).unwrap();
        assert_eq!(res, None); // wasn't last message
        assert!(!c.drop_current_message); // we haven't gone over capacity yet
        assert_eq!(c.data, vec![1, 2, 3]);

        // Create a second message that will overflow the capacity
        let mut second_message = ChannelPDUHeader {
            length: 4,
            flags: ChannelPDUFlags::CHANNEL_FLAG_SHADOW_PERSISTENT,
        }
        .encode()
        .unwrap();
        second_message.extend([4, 5, 6]);
        let second_message = tpkt::Payload::Raw(Cursor::new(second_message));

        let res = c.read(second_message).unwrap();
        assert_eq!(res, None); // wasn't last message
        assert!(c.drop_current_message); // we're now over capacity
        assert_eq!(c.data, vec![1, 2, 3]); // make sure we didn't add anything over capacity

        // Create a would-be third and final message
        let mut third_message = ChannelPDUHeader {
            length: 4,
            flags: ChannelPDUFlags::CHANNEL_FLAG_LAST,
        }
        .encode()
        .unwrap();
        third_message.extend([7, 8, 9]);
        let third_message = tpkt::Payload::Raw(Cursor::new(third_message));
        let res = c.read(third_message).unwrap();

        assert_eq!(res, None); // was the last message, but it was dropped
        assert!(!c.drop_current_message); // the drop_this_message flag was reset
        assert_eq!(c.data, vec![]); // make sure the internal data cache was reset

        // Confirm that the Client still functions as expected for <= capacity messages
        let mut good_message = ChannelPDUHeader {
            length: 4,
            flags: ChannelPDUFlags::CHANNEL_FLAG_ONLY,
        }
        .encode()
        .unwrap();
        good_message.extend([10, 11, 12]);
        let good_message = tpkt::Payload::Raw(Cursor::new(good_message));
        let res = c.read(good_message).unwrap();

        assert_eq!(res, Option::Some(Cursor::new(vec![10, 11, 12]))); // we got the payload
        assert!(!c.drop_current_message); // the drop_this_message flag was never set
        assert_eq!(c.data, vec![]); // the internal data cache was reset
    }
}
