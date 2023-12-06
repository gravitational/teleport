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

use super::errors::try_error;
use crate::errors::invalid_data_error;
use crate::{util, Message, Messages, MAX_ALLOWED_VCHAN_MSG_SIZE};
use crate::{vchan, Payload};
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use num_traits::FromPrimitive;
use rdp::core::{mcs, tpkt};
use rdp::model::error::*;
use std::collections::{HashMap, VecDeque};
use std::io::{Read, Write};

pub const CHANNEL_NAME: &str = "cliprdr";

/// Client implements a client for the clipboard virtual channel
/// (CLIPRDR) extension, as defined in:
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/fb9b7e0b-6db4-41c2-b83c-f889c1ee7688
pub struct Client {
    clipboard: HashMap<u32, Vec<u8>>,
    on_remote_copy: Box<dyn Fn(Vec<u8>) -> RdpResult<()>>,
    vchan: vchan::Client,
    incoming_paste_formats: VecDeque<ClipboardFormat>,
}

impl Default for Client {
    fn default() -> Self {
        Self::new(Box::new(|_| Ok(())))
    }
}

impl Client {
    pub fn new(on_remote_copy: Box<dyn Fn(Vec<u8>) -> RdpResult<()>>) -> Self {
        Client {
            clipboard: HashMap::new(),
            on_remote_copy,
            vchan: vchan::Client::new(MAX_ALLOWED_VCHAN_MSG_SIZE),
            incoming_paste_formats: VecDeque::new(),
        }
    }
    /// Reads raw RDP messages sent on the cliprdr virtual channel and replies as necessary.
    pub fn read_and_reply<S: Read + Write>(
        &mut self,
        payload: tpkt::Payload,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        if let Some(mut payload) = self.vchan.read(payload)? {
            let header = ClipboardPDUHeader::decode(&mut payload)?;

            debug!("received {:?}", header.msg_type);

            let responses = match header.msg_type {
                ClipboardPDUType::CB_CLIP_CAPS => self.handle_server_caps(&mut payload)?,
                ClipboardPDUType::CB_MONITOR_READY => self.handle_monitor_ready(&mut payload)?,
                ClipboardPDUType::CB_FORMAT_LIST => {
                    self.handle_format_list(&mut payload, header.data_len)?
                }
                ClipboardPDUType::CB_FORMAT_LIST_RESPONSE => {
                    self.handle_format_list_response(header.msg_flags)?
                }
                ClipboardPDUType::CB_FORMAT_DATA_REQUEST => {
                    self.handle_format_data_request(&mut payload)?
                }
                ClipboardPDUType::CB_FORMAT_DATA_RESPONSE => {
                    if header
                        .msg_flags
                        .contains(ClipboardHeaderFlags::CB_RESPONSE_OK)
                    {
                        self.handle_format_data_response(&mut payload, header.data_len)?
                    } else {
                        warn!("RDP server failed to process format data request");
                        vec![]
                    }
                }
                _ => {
                    warn!(
                        "CLIPRDR message {:?} not implemented, ignoring",
                        header.msg_type
                    );
                    vec![]
                }
            };

            let chan = &CHANNEL_NAME.to_string();
            for resp in responses {
                mcs.write(chan, resp)?;
            }
        }

        Ok(())
    }

    /// update_clipboard is invoked from Go.
    /// It updates the local clipboard cache and returns the encoded message
    /// that should be sent to the RDP server.
    pub fn update_clipboard(&mut self, data: String) -> RdpResult<Messages> {
        // convert LF to CRLF, as required by CF_TEXT and CF_UNICODETEXT
        let mut converted = String::with_capacity(data.len());
        let mut prev_was_cr = false;
        for c in data.chars() {
            match c {
                '\n' if !prev_was_cr => {
                    // convert LF to CRLF, so long as the previous character
                    // wasn't CR (in which case there's no conversion necessary)
                    converted.push('\r');
                    converted.push('\n');
                }
                c => {
                    converted.push(c);
                    if c == '\r' {
                        prev_was_cr = true;
                        continue;
                    }
                }
            }

            prev_was_cr = false;
        }

        let (data, format) = encode_clipboard(converted);
        self.clipboard.insert(format as u32, data);
        self.add_headers_and_chunkify(
            ClipboardPDUType::CB_FORMAT_LIST,
            FormatListPDU {
                format_names: vec![LongFormatName::id(format as u32)],
            }
            .encode()?,
        )
    }

    /// Handles the server capabilities message, which is the first message sent from the server
    /// to the client during the initialization sequence. Described in section 1.3.2.1.
    fn handle_server_caps(&self, payload: &mut Payload) -> RdpResult<Messages> {
        let caps = ClipboardCapabilitiesPDU::decode(payload)?;
        if let Some(general) = caps.general {
            // our capabilities are minimal, so we log the server
            // capabilities for debug purposes, but don't otherwise care
            // (the server will be forced into working with us)
            info!("RDP server clipboard capabilities: {:?}", general);
        }

        // we don't send our capabilities here, they get sent as a response
        // to the monitor ready PDU below
        Ok(vec![])
    }

    /// Handles the monitor ready PDU, which is sent from the server to the client during
    /// the initialization phase. Upon receiving this message, the client should respond
    /// with its capabilities, an optional temporary directory PDU, and a format list PDU.
    fn handle_monitor_ready(&self, _payload: &mut Payload) -> RdpResult<Messages> {
        // There's nothing additional to decode here, the monitor ready PDU is just a header.
        // In response, we need to:
        // 1. Send our clipboard capabilities
        // 2. Mimic a "copy" operation by sending a format list PDU
        // This completes the initialization process.
        let mut result = self.add_headers_and_chunkify(
            ClipboardPDUType::CB_CLIP_CAPS,
            ClipboardCapabilitiesPDU {
                general: Some(GeneralClipboardCapabilitySet {
                    version: CB_CAPS_VERSION_2,
                    flags: ClipboardGeneralCapabilityFlags::CB_USE_LONG_FORMAT_NAMES,
                }),
            }
            .encode()?,
        )?;
        result.extend(
            self.add_headers_and_chunkify(
                ClipboardPDUType::CB_FORMAT_LIST,
                FormatListPDU::<LongFormatName> {
                    format_names: vec![LongFormatName::id(0)],
                }
                .encode()?,
            )?,
        );

        Ok(result)
    }

    /// Handles the format list PDU, which is a notification from the server
    /// that some data was copied and can be requested at a later date.
    fn handle_format_list(&mut self, payload: &mut Payload, length: u32) -> RdpResult<Messages> {
        let list = FormatListPDU::<LongFormatName>::decode(payload, length)?;
        let formats = list
            .format_names
            .iter()
            .map(|n| n.format_id)
            .collect::<Vec<u32>>();

        debug!("{:?} data was copied on the RDP server", formats);
        let mut result =
            self.add_headers_and_chunkify(ClipboardPDUType::CB_FORMAT_LIST_RESPONSE, vec![])?;

        let request_format;
        if formats.contains(&(ClipboardFormat::CF_UNICODETEXT as u32)) {
            request_format = ClipboardFormat::CF_UNICODETEXT;
        } else if formats.contains(&(ClipboardFormat::CF_TEXT as u32)) {
            request_format = ClipboardFormat::CF_TEXT;
        } else if formats.contains(&(ClipboardFormat::CF_OEMTEXT as u32)) {
            request_format = ClipboardFormat::CF_OEMTEXT;
        } else {
            info!(
                "{:?} data was copied on the remote desktop, but no supported formats were found",
                formats
            );

            return Ok(result);
        }

        // Record the format of the data we're requesting so we can correcly decode the response later.
        // Response events are globally ordered so we use a FIFO queue for format tracking.
        self.incoming_paste_formats.push_back(request_format);

        // request the data by imitating a paste event.
        result.extend(self.add_headers_and_chunkify(
            ClipboardPDUType::CB_FORMAT_DATA_REQUEST,
            FormatDataRequestPDU::for_id(request_format as u32).encode()?,
        )?);

        Ok(result)
    }

    /// Handle the format list response, which is the server acknowledging that
    /// it recieved a notification that the client has updated clipboard data
    /// that may be requested in the future.
    fn handle_format_list_response(&self, flags: ClipboardHeaderFlags) -> RdpResult<Messages> {
        if !flags.contains(ClipboardHeaderFlags::CB_RESPONSE_OK) {
            warn!("RDP server did not process our copy operation");
        }
        Ok(vec![])
    }

    /// Handles a request from the RDP server for clipboard data.
    /// This message is received when a user executes a paste in the remote desktop.
    fn handle_format_data_request(&self, payload: &mut Payload) -> RdpResult<Messages> {
        let req = FormatDataRequestPDU::decode(payload)?;
        let data = match self.clipboard.get(&req.format_id) {
            Some(d) => d.clone(),
            // TODO(zmb3): send empty FORMAT_DATA_RESPONSE with RESPONSE_FAIL flag set in header
            None => {
                return Err(invalid_data_error(
                    format!(
                        "clipboard does not contain data for format {}",
                        req.format_id
                    )
                    .as_str(),
                ))
            }
        };

        self.add_headers_and_chunkify(
            ClipboardPDUType::CB_FORMAT_DATA_RESPONSE,
            FormatDataResponsePDU { data }.encode()?,
        )
    }

    /// Receives clipboard data from the remote desktop. This is the server responding
    /// to our format data request.
    fn handle_format_data_response(
        &mut self,
        payload: &mut Payload,
        length: u32,
    ) -> RdpResult<Messages> {
        let resp = FormatDataResponsePDU::decode(payload, length)?;
        let data_len = resp.data.len();
        let format = self.incoming_paste_formats.pop_front().ok_or_else(|| {
            try_error("no expected format found, possibly received too many format data responses")
        })?;

        debug!(
            "received {} bytes of copied data from Windows Desktop with format {:?}",
            data_len, format,
        );

        let decoded = decode_clipboard(resp.data, format)?;
        (self.on_remote_copy)(decoded)?;
        Ok(vec![])
    }

    /// add_headers_and_chunkify takes an encoded PDU ready to be sent over a virtual channel (payload),
    /// adds on the Clipboard PDU Header based the passed msg_type, adds the appropriate (virtual) Channel PDU Header,
    /// and splits the entire payload into chunks if the payload exceeds the maximum size.
    fn add_headers_and_chunkify(
        &self,
        msg_type: ClipboardPDUType,
        payload: Message,
    ) -> RdpResult<Messages> {
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

            // we don't advertise support for file transfers, so the server should never send this,
            // but if it does, ensure the response indicates a failure
            ClipboardPDUType::CB_FILECONTENTS_RESPONSE => ClipboardHeaderFlags::CB_RESPONSE_FAIL,

            _ => ClipboardHeaderFlags::from_bits_truncate(0),
        };

        let channel_flags = match msg_type {
            ClipboardPDUType::CB_FORMAT_LIST
            | ClipboardPDUType::CB_CLIP_CAPS
            | ClipboardPDUType::CB_FORMAT_DATA_REQUEST
            | ClipboardPDUType::CB_FORMAT_DATA_RESPONSE => {
                Some(vchan::ChannelPDUFlags::CHANNEL_FLAG_SHOW_PROTOCOL)
            }
            _ => None,
        };

        let mut inner =
            ClipboardPDUHeader::new(msg_type, msg_flags, payload.len() as u32).encode()?;
        inner.extend(payload);

        self.vchan.add_header_and_chunkify(channel_flags, inner)
    }
}

bitflags! {
    #[derive(PartialEq, Eq, Debug)]
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

    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.msg_type as u16)?;
        w.write_u16::<LittleEndian>(self.msg_flags.bits())?;
        w.write_u32::<LittleEndian>(self.data_len)?;
        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let typ = payload.read_u16::<LittleEndian>()?;
        Ok(Self {
            msg_type: ClipboardPDUType::from_u16(typ)
                .ok_or_else(|| invalid_data_error(&format!("invalid message type {typ:#04x}")))?,
            msg_flags: ClipboardHeaderFlags::from_bits(payload.read_u16::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("invalid flags in clipboard header"))?,
            data_len: payload.read_u32::<LittleEndian>()?,
        })
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
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        // there's either 0 or 1 capability sets included here
        w.write_u16::<LittleEndian>(self.general.is_some() as u16)?;
        w.write_u16::<LittleEndian>(0)?; // pad

        if let Some(set) = &self.general {
            w.write_u16::<LittleEndian>(ClipboardCapabilitySetType::General as u16)?;
            w.write_u16::<LittleEndian>(12)?; // length
            w.write_u32::<LittleEndian>(CB_CAPS_VERSION_2)?;
            w.write_u32::<LittleEndian>(set.flags.bits())?;
        }

        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let count = payload.read_u16::<LittleEndian>()?;
        payload.read_u16::<LittleEndian>()?; // pad

        match count {
            0 => Ok(Self { general: None }),
            1 => Ok(Self {
                general: Some(GeneralClipboardCapabilitySet::decode(payload)?),
            }),
            _ => Err(invalid_data_error("expected 0 or 1 capabilities")),
        }
    }
}

impl GeneralClipboardCapabilitySet {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let set_type = payload.read_u16::<LittleEndian>()?;
        if set_type != ClipboardCapabilitySetType::General as u16 {
            return Err(invalid_data_error(&format!(
                "expected general capability set (1), got {set_type}"
            )));
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
#[allow(dead_code)]
struct GeneralClipboardCapabilitySet {
    /// Specifies the RDP Clipboard Virtual Extension version number.
    /// Used for informational purposes only, and MUST NOT be used to
    /// make protocol capability decisions.
    version: u32,
    flags: ClipboardGeneralCapabilityFlags,
}

bitflags! {
    #[derive(Debug, PartialEq)]
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

/// The format list PDU is sent by either the client or server when
/// its local system clipboard is updated with new clipboard data.
///
/// It contains 0 or more format names, which are either all short
/// format or all long format depending on the server/client capabilities.
#[derive(Debug)]
struct FormatListPDU<T: FormatName> {
    format_names: Vec<T>,
}

trait FormatName: Sized {
    fn encode(&self) -> RdpResult<Message>;
    fn decode(payload: &mut Payload) -> RdpResult<Self>;
}

impl<T: FormatName> FormatListPDU<T> {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = Vec::new();
        for name in &self.format_names {
            w.extend(name.encode()?);
        }

        Ok(w)
    }

    fn decode(payload: &mut Payload, length: u32) -> RdpResult<Self> {
        let mut format_names: Vec<T> = Vec::new();

        let startpos = payload.position();
        while payload.position() - startpos < length as u64 {
            format_names.push(T::decode(payload)?);
        }

        Ok(Self { format_names })
    }
}

// encode_clipboard encodes data suitably for clipboard storage.
// This means determining the appropriate format and making sure the data has a nul terminator.
//
// This data must be valid UTF-8.
fn encode_clipboard(mut data: String) -> (Vec<u8>, ClipboardFormat) {
    if data.is_ascii() {
        if matches!(data.chars().last(), Some(x) if x != '\0') {
            data.push('\0');
        }

        (data.into_bytes(), ClipboardFormat::CF_TEXT)
    } else {
        let encoded = util::to_unicode(&data, true);
        (encoded, ClipboardFormat::CF_UNICODETEXT)
    }
}

// decode_clipboard decodes data from a given clipboard format into UTF-8.
fn decode_clipboard(mut data: Vec<u8>, format: ClipboardFormat) -> RdpResult<Vec<u8>> {
    match format {
        ClipboardFormat::CF_TEXT | ClipboardFormat::CF_OEMTEXT => {
            if data.last().copied() == Some(b'\0') {
                data.pop();
            }
            Ok(data)
        }
        ClipboardFormat::CF_UNICODETEXT => {
            let mut data = data.as_slice();
            let len = data.len();
            if len >= 2 {
                let clip = len - 2;
                if data[clip..] == [0, 0] {
                    data = &data[..clip];
                }
            }

            let units: Vec<u16> = data
                .chunks_exact(2)
                .map(|chunk| u16::from_le_bytes([chunk[0], chunk[1]]))
                .collect();

            Ok(String::from_utf16_lossy(&units).into_bytes())
        }
        _ => Err(try_error(
            "attempted to decode unsupported clipboard format",
        )),
    }
}

/// Represents the CLIPRDR_SHORT_FORMAT_NAME structure.
#[derive(Debug)]
struct ShortFormatName {
    format_id: u32,
    format_name: [u8; 32],
}

#[allow(dead_code)]
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
                format!("{name} is too long for short format name").as_str(),
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

impl FormatName for ShortFormatName {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = Vec::new();
        w.write_u32::<LittleEndian>(self.format_id)?;
        w.write_all(&self.format_name)?;

        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let format_id = payload.read_u32::<LittleEndian>()?;
        let mut format_name = [0u8; 32];
        payload.read_exact(&mut format_name)?;

        Ok(Self {
            format_id,
            format_name,
        })
    }
}

/// Represents the CLIPRDR_LONG_FORMAT_NAMES structure.
#[derive(Debug)]
struct LongFormatName {
    format_id: u32,
    format_name: Option<String>,
}

impl LongFormatName {
    fn id(id: u32) -> Self {
        Self {
            format_id: id,
            format_name: None,
        }
    }
}

impl FormatName for LongFormatName {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = Vec::new();
        w.write_u32::<LittleEndian>(self.format_id)?;
        match &self.format_name {
            // not all clipboard formats have a name; in such cases, the name
            // must be encoded as a single Unicode null character (two zero bytes)
            None => w.write_u16::<LittleEndian>(0)?,
            Some(name) => {
                w.append(&mut util::to_unicode(name, true));
            }
        };

        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let format_id = payload.read_u32::<LittleEndian>()?;
        let mut consumed = 0;
        let name: String = std::char::decode_utf16(
            payload
                .get_ref()
                .chunks_exact(2)
                .skip(payload.position() as usize / 2) // skip over format_id
                .take_while(|c| {
                    consumed += 2;
                    !matches!(c, [0x00, 0x00])
                })
                .map(|c| u16::from_le_bytes([c[0], c[1]])),
        )
        .map(|c| c.unwrap_or(std::char::REPLACEMENT_CHARACTER))
        .collect();

        // advance cursor in case there are more names to decode
        payload.set_position(payload.position() + consumed);

        Ok(Self {
            format_id,
            format_name: if name.is_empty() { None } else { Some(name) },
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
#[allow(dead_code, non_camel_case_types)]
#[repr(u32)]
#[derive(Clone, Copy, Debug, Eq, PartialEq, FromPrimitive, ToPrimitive)]
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

impl FormatDataRequestPDU {
    fn for_id(format_id: u32) -> Self {
        Self { format_id }
    }

    fn encode(&self) -> RdpResult<Message> {
        let mut w = Vec::with_capacity(4);
        w.write_u32::<LittleEndian>(self.format_id)?;
        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            format_id: payload.read_u32::<LittleEndian>()?,
        })
    }
}

/// Sent as a reply to the format data request PDU, and is used for both:
/// 1. Indicating that the processing of the request was succesful, and
/// 2. Sending the contents of the requested clipboard data
#[derive(Debug)]
struct FormatDataResponsePDU {
    data: Vec<u8>,
}

impl FormatDataResponsePDU {
    fn encode(&self) -> RdpResult<Message> {
        Ok(self.data.clone())
    }

    fn decode(payload: &mut Payload, length: u32) -> RdpResult<Self> {
        let mut data = vec![0; length as usize];
        payload.read_exact(data.as_mut_slice())?;

        Ok(Self { data })
    }
}

#[cfg(test)]
mod tests {
    use crate::vchan::ChannelPDUFlags;

    use super::*;
    use std::io::Cursor;
    use std::sync::mpsc::channel;

    #[test]
    fn decode_clipboard_overflow() {
        // a single byte is invalid for CF_UNICODETEXT
        let result = decode_clipboard(vec![54u8], ClipboardFormat::CF_UNICODETEXT).unwrap();
        assert!(result.is_empty());
    }

    #[test]
    fn encode_format_list_short() {
        let client = Client::default();
        let msg = client
            .add_headers_and_chunkify(
                ClipboardPDUType::CB_FORMAT_LIST,
                FormatListPDU {
                    format_names: vec![ShortFormatName::id(ClipboardFormat::CF_TEXT as u32)],
                }
                .encode()
                .unwrap(),
            )
            .unwrap();

        assert_eq!(
            msg[0],
            vec![
                // virtual channel header
                0x2C, 0x00, 0x00, 0x00, // length (44 bytes)
                0x13, 0x00, 0x00, 0x00, // flags (first + last + show protocol)
                // Clipboard PDU Header
                0x02, 0x00, // message type
                0x00, 0x00, // message flags (CB_ASCII_NAMES not set)
                0x24, 0x00, 0x00, 0x00, // message length (36 bytes after header)
                // Format List PDU starts here
                0x01, 0x00, 0x00, 0x00, // format ID (CF_TEXT)
                0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // format name (bytes 1-8)
                0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // format name (bytes 9-16)
                0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // format name (bytes 17-24)
                0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // format name (bytes 25-32)
            ]
        );
    }

    #[test]
    fn encode_format_list_long() {
        let empty = FormatListPDU::<LongFormatName> {
            format_names: vec![LongFormatName::id(0)],
        };

        let client = Client::default();

        let encoded = client
            .add_headers_and_chunkify(ClipboardPDUType::CB_FORMAT_LIST, empty.encode().unwrap())
            .unwrap();

        assert_eq!(
            encoded[0],
            vec![
                0x0e, 0x00, 0x00, 0x00, // message length (14 bytes)
                0x13, 0x00, 0x00, 0x00, // flags (first + last + show protocol)
                0x02, 0x00, 0x00, 0x00, // message type (format list), and flags (0)
                0x06, 0x00, 0x00, 0x00, // message length (6 bytes)
                0x00, 0x00, 0x00, 0x00, // format id 0
                0x00, 0x00 // null terminator
            ]
        );
    }

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
    fn decode_format_list_long() {
        let no_name = vec![0x01, 0x00, 0x00, 0x00, 0x00, 0x00];
        let l = no_name.len();
        let decoded =
            FormatListPDU::<LongFormatName>::decode(&mut Cursor::new(no_name), l as u32).unwrap();
        assert_eq!(decoded.format_names.len(), 1);
        assert_eq!(
            decoded.format_names[0].format_id,
            ClipboardFormat::CF_TEXT as u32
        );
        assert_eq!(decoded.format_names[0].format_name, None);

        let one_name = vec![
            0x01, 0x00, 0x00, 0x00, // CF_TEXT
            0x74, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00, // "test"
            0x00, 0x00, // null terminator
        ];
        let l = one_name.len();
        let decoded =
            FormatListPDU::<LongFormatName>::decode(&mut Cursor::new(one_name), l as u32).unwrap();
        assert_eq!(decoded.format_names.len(), 1);
        assert_eq!(
            decoded.format_names[0].format_id,
            ClipboardFormat::CF_TEXT as u32
        );
        assert_eq!(
            decoded.format_names[0].format_name,
            Some(String::from("test"))
        );

        let two_names = vec![
            0x01, 0x00, 0x00, 0x00, // CF_TEXT
            0x74, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00, // "test"
            0x00, 0x00, // null terminator
            0x01, 0x00, 0x00, 0x00, // CF_TEXT
            0x74, 0x00, 0x65, 0x00, 0x6c, 0x00, 0x65, 0x00, // "tele"
            0x70, 0x00, 0x6f, 0x00, 0x72, 0x00, 0x74, 0x00, // "port"
            0x00, 0x00, // null terminator
        ];
        let l = two_names.len();
        let decoded =
            FormatListPDU::<LongFormatName>::decode(&mut Cursor::new(two_names), l as u32).unwrap();
        assert_eq!(decoded.format_names.len(), 2);
        assert_eq!(
            decoded.format_names[0].format_id,
            ClipboardFormat::CF_TEXT as u32
        );
        assert_eq!(
            decoded.format_names[0].format_name,
            Some(String::from("test"))
        );
        assert_eq!(
            decoded.format_names[1].format_id,
            ClipboardFormat::CF_TEXT as u32
        );
        assert_eq!(
            decoded.format_names[1].format_name,
            Some(String::from("teleport"))
        );
    }

    #[test]
    fn responds_to_monitor_ready() {
        let c: Client = Default::default();
        let responses = c
            .handle_monitor_ready(&mut Cursor::new(Vec::new()))
            .unwrap();
        assert_eq!(2, responses.len());

        // First response - our client capabilities:
        let mut payload = Cursor::new(responses[0].clone());
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        assert_eq!(header.msg_type, ClipboardPDUType::CB_CLIP_CAPS);

        let capabilities = ClipboardCapabilitiesPDU::decode(&mut payload).unwrap();
        let general = capabilities.general.unwrap();
        assert_eq!(
            general.flags,
            ClipboardGeneralCapabilityFlags::CB_USE_LONG_FORMAT_NAMES
        );

        // Second response - the format list PDU:
        let mut payload = Cursor::new(responses[1].clone());
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        assert_eq!(header.msg_type, ClipboardPDUType::CB_FORMAT_LIST);
        assert_eq!(header.msg_flags.bits(), 0);
        assert_eq!(header.data_len, 6);

        let format_list =
            FormatListPDU::<LongFormatName>::decode(&mut payload, header.data_len).unwrap();
        assert_eq!(format_list.format_names.len(), 1);
        assert_eq!(format_list.format_names[0].format_id, 0);
        assert_eq!(format_list.format_names[0].format_name, None);
    }

    #[test]
    fn encodes_large_format_data_response() {
        let mut data = vec![0; vchan::CHANNEL_CHUNK_LEGNTH + 2];
        for (i, item) in data.iter_mut().enumerate() {
            *item = (i % 256) as u8;
        }
        let pdu = FormatDataResponsePDU { data };
        let encoded = pdu.encode().unwrap();
        let client = Client::default();
        let messages = client
            .add_headers_and_chunkify(ClipboardPDUType::CB_FORMAT_DATA_RESPONSE, encoded)
            .unwrap();
        assert_eq!(2, messages.len());

        let header0 =
            vchan::ChannelPDUHeader::decode(&mut Cursor::new(messages[0].clone())).unwrap();
        assert_eq!(
            ChannelPDUFlags::CHANNEL_FLAG_FIRST | ChannelPDUFlags::CHANNEL_FLAG_SHOW_PROTOCOL,
            header0.flags
        );
        let header1 =
            vchan::ChannelPDUHeader::decode(&mut Cursor::new(messages[1].clone())).unwrap();
        assert_eq!(
            ChannelPDUFlags::CHANNEL_FLAG_LAST | ChannelPDUFlags::CHANNEL_FLAG_SHOW_PROTOCOL,
            header1.flags
        );
    }

    #[test]
    fn responds_to_format_data_request_hasdata() {
        // a null-terminated utf-16 string, represented as a Vec<u8>
        let test_data = util::to_unicode("test", true);

        let mut c: Client = Default::default();
        c.clipboard
            .insert(ClipboardFormat::CF_UNICODETEXT as u32, test_data.clone());

        let req = FormatDataRequestPDU::for_id(ClipboardFormat::CF_UNICODETEXT as u32);
        let responses = c
            .handle_format_data_request(&mut Cursor::new(req.encode().unwrap()))
            .unwrap();

        // expect one FormatDataResponsePDU
        assert_eq!(responses.len(), 1);
        let mut payload = Cursor::new(responses[0].clone());
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        assert_eq!(header.msg_type, ClipboardPDUType::CB_FORMAT_DATA_RESPONSE);
        assert_eq!(header.msg_flags, ClipboardHeaderFlags::CB_RESPONSE_OK);
        assert_eq!(header.data_len, 10);
        let resp = FormatDataResponsePDU::decode(&mut payload, header.data_len).unwrap();
        assert_eq!(resp.data, test_data);
    }

    #[test]
    fn invokes_callback_with_clipboard_data() {
        let (send, recv) = channel();

        let mut c = Client::new(Box::new(move |vec| {
            send.send(vec).unwrap();
            Ok(())
        }));

        let data_format_list = FormatListPDU {
            format_names: vec![LongFormatName {
                format_id: ClipboardFormat::CF_TEXT as u32,
                format_name: None,
            }],
        }
        .encode()
        .unwrap();

        let data_resp = FormatDataResponsePDU {
            data: String::from("abc\0").into_bytes(),
        }
        .encode()
        .unwrap();

        let mut len = data_format_list.len() as u32;
        c.handle_format_list(&mut Cursor::new(data_format_list), len)
            .unwrap();

        len = data_resp.len() as u32;
        c.handle_format_data_response(&mut Cursor::new(data_resp), len)
            .unwrap();

        // ensure that the null terminator was trimmed
        let received = recv.try_recv().unwrap();
        assert_eq!(received, String::from("abc").into_bytes());
    }

    #[test]
    fn update_clipboard_returns_format_list_pdu() {
        let mut c: Client = Default::default();
        let messages = c.update_clipboard("abc".to_owned()).unwrap();
        let bytes = messages[0].clone();

        // verify that it returns a properly encoded format list PDU
        let mut payload = Cursor::new(bytes);
        let _pdu_header = vchan::ChannelPDUHeader::decode(&mut payload).unwrap();
        let header = ClipboardPDUHeader::decode(&mut payload).unwrap();
        let format_list =
            FormatListPDU::<LongFormatName>::decode(&mut payload, header.data_len).unwrap();
        assert_eq!(ClipboardPDUType::CB_FORMAT_LIST, header.msg_type);
        assert_eq!(1, format_list.format_names.len());
        assert_eq!(
            ClipboardFormat::CF_TEXT as u32,
            format_list.format_names[0].format_id
        );

        // verify that the clipboard data is now cached
        // (with a null-terminating character)
        assert_eq!(
            String::from("abc\0").into_bytes(),
            *c.clipboard.get(&(ClipboardFormat::CF_TEXT as u32)).unwrap()
        );
    }

    #[test]
    fn update_clipboard_conversion() {
        struct Item(&'static str, &'static [u8], ClipboardFormat);
        for Item(input, expected, format) in [
            Item("abc\0", b"abc\0", ClipboardFormat::CF_TEXT), // already null-terminated, no conversion necessary
            Item("\n123", b"\r\n123\0", ClipboardFormat::CF_TEXT), // starts with LF
            Item("def\r\n", b"def\r\n\0", ClipboardFormat::CF_TEXT), // already CRLF, no conversion necessary
            Item("gh\r\nij\nk", b"gh\r\nij\r\nk\0", ClipboardFormat::CF_TEXT), // mixture of both
            Item(
                "🤑\n",
                &[62, 216, 17, 221, b'\r', 0, b'\n', 0, 0, 0],
                ClipboardFormat::CF_UNICODETEXT,
            ), // detection and utf8 -> utf16 conversion & CRLF conversion
            Item(
                "🤑\r\n",
                &[62, 216, 17, 221, b'\r', 0, b'\n', 0, 0, 0],
                ClipboardFormat::CF_UNICODETEXT,
            ), // detection and utf8 -> utf16 conversion & no CRLF conversion
        ] {
            let mut c: Client = Default::default();
            c.update_clipboard(input.to_owned()).unwrap();

            assert_eq!(
                expected,
                *c.clipboard.get(&(format as u32)).unwrap(),
                "testing {input}",
            );
        }
    }
}
