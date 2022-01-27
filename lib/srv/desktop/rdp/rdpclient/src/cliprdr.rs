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
use rdp::core::{mcs, tpkt};
use rdp::model::error::*;
use std::io::{Read, Write};

const CHANNEL_NAME: &str = "cliprdr";

/// Client implements a client for the clipboard virtual channel
/// (CLIPRDR) extension, as defined in:
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/fb9b7e0b-6db4-41c2-b83c-f889c1ee7688
pub struct Client {}

impl Client {
    pub fn new() -> Self {
        Client {}
    }

    pub fn read<S: Read + Write>(
        &mut self,
        payload: tpkt::Payload,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        Ok(())
    }
}

bitflags! {
    struct ClipboardHeaderFlags: u16 {
        /// Indicates that the assocated request was processed successfully.
        const CB_RESPONSE_OK = 0x0001;

        /// Indicates that the associated request was not procesed successfully.
        const CB_RESPONSE_FAIL = 0x0002;

        /// Used by the short format name variant to indicate that the format
        /// names are in ASCII 8.
        const CB_ASCII_NAMES = 0x0004;
    }
}

/// This header (CLIPRDR_HEADER) is present in all clipboard PDUs.
#[derive(Debug)]
struct ClipboardPDUHeader {
    /// Specifies the type of clipboard PDU that follows the dataLen field.
    msg_type: ClipboardPDUType,
    msg_flags: ClipboardHeaderFlags,
    /// Specifies the size, in bytes, of the data which follows this header.
    data_len: u32,
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum ClipboardPDUType {
    CB_MONITOR_READY = 0x0001,
    CB_FORMAT_LIST = 0x0002,
    CB_FORMAT_LIST_RESPONSE = 0x0003,
    CB_FORMAT_DATA_REQUEST = 0x0004,
    CB_FORMAT_DATA_RESPONSE = 0x0005,
    CB_TEMP_DIRECTORY = 0x0006,
    CB_CLIP_CAPS = 0x0007,
    CB_FILECONTENTS_REQUEST = 0x0008,
    CB_FILECONTENTS_RESPONSE = 0x0009,
    CB_LOCK_CLIPDATA = 0x000A,
    CB_UNLOCK_CLIPDATA = 0x000B,
}

/// An optional PDU (CLIPRDR_CAPS) used to exchange capability information.
/// If this PDU is not sent, it is assumed that the endpoint which did not
/// send capabilities is using the default values for each field.
#[derive(Debug)]
struct ClipboardCapabilitiesPDU {
    // The protocol is written in such a way that there can be
    // a variable number of capability sets in this PDU. However,
    // the spec only defines one type of capability set (general),
    // so we'll just use an Option.
    general: Option<GeneralClipboardCapabilitySet>,
}

const CB_CAPS_VERSION_2: u32 = 0x0002;

impl ClipboardCapabilitiesPDU {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        // there's either 0 or 1 capability sets included here
        w.write_u16::<LittleEndian>(self.general.as_ref().map_or(0, |_| 1))?;
        w.write_u16::<LittleEndian>(0)?; // pad

        if let Some(set) = &self.general {
            w.write_u16::<LittleEndian>(ClipboardCapabilitySetType::General as u16)?;
            w.write_u16::<LittleEndian>(12)?; // length
            w.write_u32::<LittleEndian>(CB_CAPS_VERSION_2)?;
            w.write_u32::<LittleEndian>(set.flags.bits)?;
        }

        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let count = payload.read_u16::<LittleEndian>()?;
        payload.read_u16::<LittleEndian>()?; // pad

        return match count {
            0 => Ok(Self { general: None }),
            1 => Ok(Self {
                general: Some(GeneralClipboardCapabilitySet::decode(payload)?),
            }),
            _ => Err(invalid_data_error("expected 0 or 1 capabilities")),
        };
    }
}

impl GeneralClipboardCapabilitySet {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let set_type = payload.read_u16::<LittleEndian>()?;
        if set_type != ClipboardCapabilitySetType::General as u16 {
            return Err(invalid_data_error("expected general capability set"));
        }

        let length = payload.read_u16::<LittleEndian>()?;
        if length != 12u16 {
            return Err(invalid_data_error(
                "expected 12 bytes for the general capability set",
            ));
        }

        Ok(Self {
            version: payload.read_u32::<LittleEndian>()?,
            flags: ClipboardGeneralCapabilityFlags::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("invalid flags in general capability set"))?,
        })
    }
}

enum ClipboardCapabilitySetType {
    General = 0x0001,
}

/// The general capability set (CLIPRDR_GENERAL_CAPABILITY) is used to
/// advertise general clipboard settings.
#[derive(Debug)]
struct GeneralClipboardCapabilitySet {
    /// Specifies the RDP Clipboard Virtual Extension version number.
    /// Used for informational purposes only, and MUST NOT be used to
    /// make protocol capability decisions.
    version: u32,
    flags: ClipboardGeneralCapabilityFlags,
}

bitflags! {
    struct ClipboardGeneralCapabilityFlags: u32 {
        /// Indicates that long format names will be used in the format list PDU.
        /// If this flag is not set, then the short format names MUST be used.
        const CB_USE_LONG_FORMAT_NAMES = 0x0002;

        /// File copy and paste using stream-based operations are supported.
        const CB_STREAM_FILECLIP_ENABLED = 0x0004;

        /// Indicates that any description of files to copy and paste MUST NOT
        /// include the source path of the files.
        const CB_FILECLIP_NO_FILE_PATHS = 0x0008;

        /// Indicates that locking and unlocking of file stream data
        /// on the clipboard is supported.
        const CB_CAN_LOCK_CLIPDATA = 0x0010;

        /// Indicates support for transferring files greater than 4GB.
        const CB_HUGE_FILE_SUPPORT_ENABLED = 0x0020;
    }
}
