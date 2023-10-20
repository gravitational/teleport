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

use futures_util::future::err;
use std::fmt::{Debug, Formatter};

use ironrdp_cliprdr::backend::CliprdrBackend;
use ironrdp_cliprdr::pdu::{
    ClipboardFormat, ClipboardFormatId, ClipboardGeneralCapabilityFlags, FileContentsRequest,
    FileContentsResponse, FormatDataRequest, FormatDataResponse, LockDataId,
};
use ironrdp_cliprdr::{Cliprdr, CliprdrSvcMessages};
use ironrdp_pdu::PduResult;
use ironrdp_svc::impl_as_any;
use static_init::dynamic;

use crate::client::{ClientFunction, ClientHandle};
use crate::util;

#[dynamic]
static CF_UNICODETEXT: ClipboardFormat = ClipboardFormat::new(ClipboardFormatId::CF_UNICODETEXT);

#[dynamic]
static CF_TEXT: ClipboardFormat = ClipboardFormat::new(ClipboardFormatId::CF_TEXT);

#[derive(Debug)]
pub struct TeleportCliprdrBackend {
    client_handle: ClientHandle,
    clipboard_data: Option<String>,
    requested_formats: Vec<ClipboardFormatId>,
}

impl TeleportCliprdrBackend {
    pub fn new(client_handle: ClientHandle) -> Self {
        Self {
            client_handle,
            clipboard_data: None,
            requested_formats: vec![],
        }
    }

    pub fn set_clipboard_data(&mut self, data: String) {
        self.clipboard_data = Some(data);
        self.on_request_format_list();
    }

    fn send<F>(&self, name: &'static str, f: F)
    where
        F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
    {
        let res = self
            .client_handle
            .blocking_send(ClientFunction::WriteCliprdr(Box::new(
                ClipboardFnInternal::new(name, f),
            )));
        if let Err(e) = res {
            error!("Couldn't send request for {}: {:?}", name, e);
        }
    }
}

impl CliprdrBackend for TeleportCliprdrBackend {
    fn temporary_directory(&self) -> &str {
        ".cliprdr"
    }

    fn client_capabilities(&self) -> ClipboardGeneralCapabilityFlags {
        trace!("CLIPRDR: client_capabilities");
        ClipboardGeneralCapabilityFlags::USE_LONG_FORMAT_NAMES
    }

    fn on_request_format_list(&mut self) {
        trace!("CLIPRDR: on_request_format_list");
        let formats = available_formats(&self.clipboard_data);
        self.send("request_format_list", move |c| c.initiate_copy(&formats));
    }

    fn on_process_negotiated_capabilities(
        &mut self,
        capabilities: ClipboardGeneralCapabilityFlags,
    ) {
        // our capabilities are minimal, so we log the server
        // capabilities for debug purposes, but don't otherwise care
        // (the server will be forced into working with us)
        info!("RDP server clipboard capabilities: {:?}", capabilities);
    }

    fn on_remote_copy(&mut self, available_formats: &[ClipboardFormat]) {
        trace!(
            "CLIPRDR: on_remote_copy, available formats: {:?}",
            available_formats
        );

        let format = if available_formats.contains(&CF_UNICODETEXT) {
            ClipboardFormatId::CF_UNICODETEXT
        } else if available_formats.contains(&CF_TEXT) {
            ClipboardFormatId::CF_TEXT
        } else {
            debug!(
                "data was copied on the remote desktop, but no supported formats were found: {:?}",
                available_formats
            );
            return;
        };

        self.requested_formats.push(format);
        self.send("remote_copy", move |c| c.initiate_paste(format));
    }

    fn on_format_data_request(&mut self, format: FormatDataRequest) {
        trace!("CLIPRDR: on_format_data_request");
        let response = match &self.clipboard_data {
            Some(data) => convert_string(data, format.format),
            None => {
                debug!(
                    "format {:?} was requested but no data is available",
                    format.format
                );
                None
            }
        };
        self.send("format_data_request", move |c| {
            let response = match response.clone() {
                None => FormatDataResponse::new_error(),
                Some(data) => FormatDataResponse::new_data(data),
            };
            c.submit_format_data(response)
        });
    }

    fn on_format_data_response(&mut self, response: FormatDataResponse) {
        trace!("CLIPRDR: on_format_data_response");
        if self.requested_formats.is_empty() {
            error!("Received data response but no format was requested");
            return;
        }
        let format = self.requested_formats.remove(0);
        if response.is_error() {
            error!("Received error in format_data_response");
            return;
        }
        let data = response.data().to_vec();
        let data = match format {
            ClipboardFormatId::CF_UNICODETEXT => util::from_unicode(data).ok(),
            ClipboardFormatId::CF_TEXT => util::from_utf8(data).ok(),
            _ => {
                error!("Requested unknown format! This should never happen!");
                return;
            }
        };
        match data {
            Some(mut data) => {
                if let Err(e) = self
                    .client_handle
                    .blocking_send(ClientFunction::HandleRemoteCopy(data.into_bytes()))
                {
                    error!("Can't send format_data_response: {:?}", e);
                }
            }
            None => {
                error!("Can't convert string");
            }
        };
    }

    fn on_file_contents_request(&mut self, _: FileContentsRequest) {
        warn!("CLIPRDR file contents request not implemented");
    }

    fn on_file_contents_response(&mut self, _: FileContentsResponse) {
        warn!("CLIPRDR file contents response not implemented");
    }

    fn on_lock(&mut self, _: LockDataId) {
        warn!("CLIPRDR locking not implemented");
    }

    fn on_unlock(&mut self, _: LockDataId) {
        warn!("CLIPRDR locking not implemented");
    }
}

impl_as_any!(TeleportCliprdrBackend);

pub trait ClipboardFn: Send + Debug + 'static {
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages>;
}

impl<F> ClipboardFn for F
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + Debug + 'static,
{
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages> {
        (self)(cliprdr)
    }
}

struct ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
{
    name: &'static str,
    closure: F,
}

impl<F> ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
{
    fn new(name: &'static str, closure: F) -> Self {
        Self { name, closure }
    }
}

impl<F> Debug for ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
{
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", &self.name)
    }
}

impl<F> ClipboardFn for ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
{
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages> {
        (self.closure)(cliprdr)
    }
}

pub fn available_formats(data: &Option<String>) -> Vec<ClipboardFormat> {
    if let Some(s) = data {
        let mut formats = vec![CF_UNICODETEXT.to_owned()];
        if s.is_ascii() {
            formats.push(CF_TEXT.to_owned())
        }
        return formats;
    }
    vec![]
}

fn adjust_new_lines(data: &str) -> String {
    // convert LF to CRLF, as required by CF_TEXT and CF_UNICODETEXT
    let mut converted = String::with_capacity(data.len());
    let mut prev = '_';
    for current in data.chars() {
        match current {
            '\n' if prev != '\r' => {
                // convert LF to CRLF, so long as the previous character
                // wasn't CR (in which case there's no conversion necessary)
                converted.push('\r');
                converted.push('\n');
            }
            c => converted.push(c),
        }
        prev = current;
    }
    converted
}

fn convert_string(data: &String, format_id: ClipboardFormatId) -> Option<Vec<u8>> {
    match format_id {
        ClipboardFormatId::CF_UNICODETEXT => Some(util::to_unicode(&adjust_new_lines(data), true)),
        ClipboardFormatId::CF_TEXT if data.is_ascii() => {
            let mut data = adjust_new_lines(data).into_bytes();
            if data.last().unwrap_or(&1u8) != &0u8 {
                data.push(0u8);
            }
            Some(data)
        }
        _ => {
            debug!("incorrect format requested: {:?}", format_id);
            None
        }
    }
}

#[cfg(test)]
mod tests {
    use ironrdp_cliprdr::pdu::ClipboardFormatId;

    use super::*;
    use crate::cliprdr::convert_string;
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
        struct Item(&'static str, Option<Vec<u8>>, ClipboardFormatId);
        for Item(input, expected, format) in [
            Item("🤑", None, ClipboardFormatId::CF_TEXT), //can't convert non-ascii to CF_TEXT
            Item("abc\0", Some(b"abc\0".to_vec()), ClipboardFormatId::CF_TEXT), // already null-terminated, no conversion necessary
            Item(
                "\n123",
                Some(b"\r\n123\0".to_vec()),
                ClipboardFormatId::CF_TEXT,
            ), // starts with LF
            Item(
                "def\r\n",
                Some(b"def\r\n\0".to_vec()),
                ClipboardFormatId::CF_TEXT,
            ), // already CRLF, no conversion necessary
            Item(
                "gh\r\nij\nk",
                Some(b"gh\r\nij\r\nk\0".to_vec()),
                ClipboardFormatId::CF_TEXT,
            ), // mixture of both
            Item(
                "🤑\n",
                Some(vec![62, 216, 17, 221, b'\r', 0, b'\n', 0, 0, 0]),
                ClipboardFormatId::CF_UNICODETEXT,
            ), // detection and utf8 -> utf16 conversion & CRLF conversion
            Item(
                "🤑\r\n",
                Some(vec![62, 216, 17, 221, b'\r', 0, b'\n', 0, 0, 0]),
                ClipboardFormatId::CF_UNICODETEXT,
            ), // detection and utf8 -> utf16 conversion & no CRLF conversion
        ] {
            assert_eq!(
                expected,
                convert_string(&input.to_string(), format),
                "testing {input}",
            );
        }
    }
}
