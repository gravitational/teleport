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
use crate::{vchan, Payload};
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use num_traits::FromPrimitive;
use rdp::core::{mcs, tpkt};
use rdp::model::data::Message;
use rdp::model::error::*;
use rdp::try_let;
use std::io::{Read, Write};

pub const CHANNEL_NAME: &str = "cliprdr";

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
        let mut payload = try_let!(tpkt::Payload::Raw, payload)?;
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload)?;
        let header = ClipboardPDUHeader::decode(&mut payload)?;
        warn!("msgType {:?}", header.msg_type);

        let resp = match header.msg_type {
            ClipboardPDUType::CB_CLIP_CAPS => self.handle_server_caps(&mut payload)?,
            ClipboardPDUType::CB_MONITOR_READY => self.handle_monitor_ready(&mut payload)?,
            ClipboardPDUType::CB_FORMAT_LIST => {
                self.handle_format_list(&mut payload, header.data_len)?
            }
            ClipboardPDUType::CB_FORMAT_LIST_RESPONSE => {
                self.handle_format_list_response(header.msg_flags)?
            }
            _ => {
                error!(
                    "CLIPRDR message {:?} not implemented, ignoring",
                    header.msg_type
                );
                None
            }
        };

        if let Some(resp) = resp {
            let chan = &CHANNEL_NAME.to_string();
            for msg in resp {
                info!("sending clipboard message");
                mcs.write(chan, msg)?;
            }
        }
        Ok(())
    }

    fn handle_server_caps(&self, payload: &mut Payload) -> RdpResult<Option<Vec<Vec<u8>>>> {
        let caps = ClipboardCapabilitiesPDU::decode(payload)?;
        if let Some(general) = caps.general {
            // our capabilities are bare minimum, so we log the server
            // capabilities for debug purposes, but don't otherwise care
            // (the server will be forced into working with us)
            info!("RDP server clipboard capabilities: {:?}", general);
        }

        // we don't send our capabilities here, they get sent as a response
        // to the monitor ready PDU below
        Ok(None)
    }

    fn handle_monitor_ready(&self, _payload: &mut Payload) -> RdpResult<Option<Vec<Vec<u8>>>> {
        // There's nothing additional to decode here, the monitor ready PDU is just a header.
        // In response, we need to:
        // 1. Send our clipboard capabilities
        // 2. Mimic a "copy" operation by sending a format list PDU
        // This completes the initialization process.

        let mut result = Vec::with_capacity(2);

        result.push(encode_message(
            ClipboardPDUType::CB_CLIP_CAPS,
            ClipboardCapabilitiesPDU {
                general: Some(GeneralClipboardCapabilitySet {
                    version: CB_CAPS_VERSION_2,
                    flags: ClipboardGeneralCapabilityFlags::from_bits_truncate(0),
                }),
            }
            .encode()?,
        )?);

        result.push(encode_message(
            ClipboardPDUType::CB_FORMAT_LIST,
            FormatListPDU {
                format_names: vec![
                    ShortFormatName::id(ClipboardFormat::CF_TEXT as u32),
                    ShortFormatName::id(ClipboardFormat::CF_UNICODETEXT as u32),
                ],
            }
            .encode()?,
        )?);

        Ok(Some(result))
    }

    fn handle_format_list(
        &self,
        payload: &mut Payload,
        length: u32,
    ) -> RdpResult<Option<Vec<Vec<u8>>>> {
        info!("received format list, data was copied on the RDP server");
        let list = FormatListPDU::decode(payload, length)?;
        for name in list.format_names {
            // TODO(zmb3): name.format_name could be UTF-16, depending on whether
            // the header set the CB_ASCII_NAMES flag
            info!(
                "{}: {:?}",
                name.format_id,
                std::str::from_utf8(&name.format_name),
            );
        }

        Ok(Some(vec![encode_message(
            ClipboardPDUType::CB_FORMAT_LIST_RESPONSE,
            vec![],
        )?]))
    }

    fn handle_format_list_response(
        &self,
        flags: ClipboardHeaderFlags,
    ) -> RdpResult<Option<Vec<Vec<u8>>>> {
        if !flags.contains(ClipboardHeaderFlags::CB_RESPONSE_OK) {
            warn!("RDP server did not process our copy operation");
        } else {
            warn!("Got format list response from RDP server");
        }
        Ok(None)
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
#[derive(Debug, PartialEq, Eq)]
struct ClipboardPDUHeader {
    /// Specifies the type of clipboard PDU that follows the dataLen field.
    msg_type: ClipboardPDUType,
    msg_flags: ClipboardHeaderFlags,
    /// Specifies the size, in bytes, of the data which follows this header.
    data_len: u32,
}

impl ClipboardPDUHeader {
    fn new(msg_type: ClipboardPDUType, msg_flags: ClipboardHeaderFlags, data_len: u32) -> Self {
        ClipboardPDUHeader {
            msg_type,
            msg_flags,
            data_len,
        }
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let typ = payload.read_u16::<LittleEndian>()?;
        Ok(Self {
            msg_type: ClipboardPDUType::from_u16(typ)
                .ok_or_else(|| invalid_data_error(&format!("invalid message type {:#04x}", typ)))?,
            msg_flags: ClipboardHeaderFlags::from_bits(payload.read_u16::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("invalid flags in clipboard header"))?,
            data_len: payload.read_u32::<LittleEndian>()?,
        })
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.msg_type as u16)?;
        w.write_u16::<LittleEndian>(self.msg_flags.bits())?;
        w.write_u32::<LittleEndian>(self.data_len)?;
        Ok(w)
    }
}
#[derive(Clone, Copy, Debug, Eq, PartialEq, FromPrimitive, ToPrimitive)]
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
            return Err(invalid_data_error(
                format!("expected general capability set (1), got {}", set_type).as_str(),
            ));
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

/// Sent by either the client or server when its local system clipboard
/// is updated with new clipboard data.
#[derive(Debug)]
struct FormatListPDU {
    // the spec also defines long format names (2.2.3.1.2),
    // but we don't advertise support for them, and the spec
    // requires that both sides support long format names in
    // order for them to be used, so it's safe to assume
    // short names only
    format_names: Vec<ShortFormatName>,
}

impl FormatListPDU {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = Vec::with_capacity(self.format_names.len() * 36);
        for name in &self.format_names {
            w.write_u32::<LittleEndian>(name.format_id)?;
            w.write_all(&name.format_name)?;
        }

        Ok(w)
    }

    fn decode(payload: &mut Payload, mut length: u32) -> RdpResult<Self> {
        let mut format_names = vec![];
        while length > 0 {
            let format_id = payload.read_u32::<LittleEndian>()?;
            let mut format_name = [0u8; 32];
            payload.read_exact(&mut format_name)?;

            length -= 36;
            format_names.push(ShortFormatName {
                format_id,
                format_name,
            })
        }

        Ok(Self { format_names })
    }
}

/// Represents the CLIPRDR_SHORT_FORMAT_NAME structure.
#[derive(Debug)]
struct ShortFormatName {
    format_id: u32,
    format_name: [u8; 32],
}

impl ShortFormatName {
    fn id(id: u32) -> Self {
        Self {
            format_id: id,
            format_name: [0u8; 32],
        }
    }

    fn from_str(id: u32, name: &str) -> RdpResult<Self> {
        if name.len() > 32 {
            return Err(invalid_data_error(
                format!("{} is too long for short format name", name).as_str(),
            ));
        }
        let mut dest = [0u8; 32];
        dest[..name.len()].copy_from_slice(name.as_bytes());
        Ok(Self {
            format_id: id,
            format_name: dest,
        })
    }
}

/// All data copied to a system clipboard has to conform to a format
/// specification. These formats are identified by unique numeric IDs,
/// which are OS-specific.
///
/// See section 1.3.1.2.
///
/// Standard clipboard formats are listed here: https://docs.microsoft.com/en-us/windows/win32/dataxchg/standard-clipboard-formats
///
/// Applications can define their own clipboard formats as well.
#[allow(non_camel_case_types)]
enum ClipboardFormat {
    CF_TEXT = 1,         // CRLF line endings, null-terminated
    CF_BITMAP = 2,       // HBITMAP handle
    CF_METAFILEPICT = 3, // 1.3.1.1.3
    CF_SYLK = 4,         // Microsoft symbolik link format
    CF_DIF = 5,          // Software Arts' Data Interchange Format
    CF_TIFF = 6,         // tagged-image file format
    CF_OEMTEXT = 7,      // OEM charset, CRLF line endings, null-terminated
    CF_DIB = 8,          // BITMAPINFO
    CF_PALETTE = 9,      // 1.3.1.1.2
    CF_PENDATA = 10,     // Microsoft Windows for Pen Computing
    CF_RIFF = 11,        // audio data more complex than CF_WAVE
    CF_WAVE = 12,        // audio data in standard wav format
    CF_UNICODETEXT = 13, // unicode text, lines end with CRLF, null-terminated
    CF_ENHMETAFILE = 14, // handle to an enhanced metafile
    CF_HDROP = 15,       // identifies a list of files
    CF_LOCALE = 16,      // locale identifier, so application can lookup charset when pasting

    CF_PRIVATEFIRST = 0x0200, // range for private clipboard formats
    CF_PRIVATELAST = 0x02FF, // https://docs.microsoft.com/en-us/windows/win32/dataxchg/clipboard-formats#private-clipboard-formats

    CF_GDIOBJFIRST = 0x0300, // range for application-defined GDI object formats
    CF_GDIOBJLAST = 0x03FF, // https://docs.microsoft.com/en-us/windows/win32/dataxchg/clipboard-formats#private-clipboard-formats
}

/// Sent as a reply to the format list PDU - used to indicate whether
/// the format list PDU was processed succesfully.
#[derive(Debug)]
struct FormatListResponsePDU {
    // empty, the only information needed is the flags in the header
}

/// Sent by the recipient of the format list PDU  in order to request
/// the data for one of the clipboard formats that was listed in the
/// format list PDU.
///
/// See section 2.2.5.1: CLIPRDR_FORMAT_DATA_REQUEST
#[derive(Debug)]
struct FormatDataRequestPDU {
    format_id: u32,
}

/// Sent as a reply to the format data request PDU, and is used for both:
/// 1. Indicating that the processing of the request was succesful, and
/// 2. Sending the contents of the requested clipboard data
#[derive(Debug)]
struct FormatDataResponsePDU {
    data: Vec<u8>,
}

/// encode_message encodes a message by wrapping it in the appropriate
/// channel header
fn encode_message(msg_type: ClipboardPDUType, payload: Vec<u8>) -> RdpResult<Vec<u8>> {
    let msg_flags = match msg_type {
        // the spec requires 0 for these messages
        ClipboardPDUType::CB_CLIP_CAPS => ClipboardHeaderFlags::from_bits_truncate(0),
        ClipboardPDUType::CB_TEMP_DIRECTORY => ClipboardHeaderFlags::from_bits_truncate(0),
        ClipboardPDUType::CB_LOCK_CLIPDATA => ClipboardHeaderFlags::from_bits_truncate(0),
        ClipboardPDUType::CB_UNLOCK_CLIPDATA => ClipboardHeaderFlags::from_bits_truncate(0),
        ClipboardPDUType::CB_FORMAT_DATA_REQUEST => ClipboardHeaderFlags::from_bits_truncate(0),

        // assume success for now
        ClipboardPDUType::CB_FORMAT_DATA_RESPONSE => ClipboardHeaderFlags::CB_RESPONSE_OK,
        ClipboardPDUType::CB_FORMAT_LIST_RESPONSE => ClipboardHeaderFlags::CB_RESPONSE_OK,

        // we enforce ASCII names for simplicity
        ClipboardPDUType::CB_FORMAT_LIST => ClipboardHeaderFlags::CB_ASCII_NAMES,

        // we don't advertise support for file transfers, so the server should never send this,
        // but if it does, ensure the response indicates a failure
        ClipboardPDUType::CB_FILECONTENTS_RESPONSE => ClipboardHeaderFlags::CB_RESPONSE_FAIL,

        _ => ClipboardHeaderFlags::from_bits_truncate(0),
    };

    let mut inner = ClipboardPDUHeader::new(msg_type, msg_flags, payload.len() as u32).encode()?;
    inner.extend_from_slice(&payload);

    let mut outer = vchan::ChannelPDUHeader::new(inner.length() as u32).encode()?;
    outer.extend_from_slice(&inner);
    Ok(outer)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn encode_clipboard_capabilities() {
        let msg = ClipboardCapabilitiesPDU {
            general: Some(GeneralClipboardCapabilitySet {
                version: CB_CAPS_VERSION_2,
                flags: ClipboardGeneralCapabilityFlags::from_bits_truncate(0),
            }),
        }
        .encode()
        .unwrap();

        assert_eq!(
            msg,
            vec![
                0x01, 0x00, 0x00, 0x00, // count, pad
                0x01, 0x00, 0x0C, 0x00, // type, length
                0x02, 0x00, 0x00, 0x00, // version (2)
                0x00, 0x00, 0x00, 0x00, // flags (0)
            ]
        )
    }

    #[test]
    fn decode_clipboard_capabilities() {
        let msg = ClipboardCapabilitiesPDU::decode(&mut Cursor::new(vec![
            0x01, 0x00, 0x00, 0x00, // count, pad
            0x01, 0x00, 0x0C, 0x00, // type, length
            0x02, 0x00, 0x00, 0x00, // version (2)
            0x00, 0x00, 0x00, 0x00, // flags (0)
        ]))
        .unwrap();

        let general_set = msg.general.unwrap();
        assert_eq!(general_set.flags.bits(), 0);
        assert_eq!(general_set.version, CB_CAPS_VERSION_2);
    }

    #[test]
    fn responds_to_monitor_ready() {
        let c = Client::new();
        let responses = c
            .handle_monitor_ready(&mut Cursor::new(Vec::new()))
            .unwrap()
            .unwrap();
        assert_eq!(2, responses.len());

        // First response - our client capabilities:
        let mut payload = Cursor::new(responses[0].clone());
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        assert_eq!(header.msg_type, ClipboardPDUType::CB_CLIP_CAPS);

        let capabilities = ClipboardCapabilitiesPDU::decode(&mut payload).unwrap();
        let general = capabilities.general.unwrap();
        assert_eq!(general.flags.bits(), 0);

        // Second response - the format list PDU:
        let mut payload = Cursor::new(responses[1].clone());
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        assert_eq!(header.msg_type, ClipboardPDUType::CB_FORMAT_LIST);
        assert_eq!(header.msg_flags, ClipboardHeaderFlags::CB_ASCII_NAMES);
        assert_eq!(header.data_len, 72);

        let format_list = FormatListPDU::decode(&mut payload, header.data_len).unwrap();
        assert_eq!(format_list.format_names.len(), 2);
        assert_eq!(
            format_list.format_names[0].format_id,
            ClipboardFormat::CF_TEXT as u32
        );
        assert_eq!(format_list.format_names[0].format_name, [0u8; 32]);
        assert_eq!(
            format_list.format_names[1].format_id,
            ClipboardFormat::CF_UNICODETEXT as u32
        );
        assert_eq!(format_list.format_names[1].format_name, [0u8; 32]);
    }
}
