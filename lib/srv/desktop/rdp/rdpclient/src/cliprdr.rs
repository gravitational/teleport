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

use crate::client::ClientHandle;
use crate::util;
use ironrdp_cliprdr::backend::CliprdrBackend;
use ironrdp_cliprdr::pdu::{
    ClipboardFormat, ClipboardFormatId, ClipboardGeneralCapabilityFlags, FileContentsRequest,
    FileContentsResponse, FormatDataRequest, FormatDataResponse, LockDataId,
};
use ironrdp_cliprdr::{Client, CliprdrClient as Cliprdr, CliprdrSvcMessages};
use ironrdp_pdu::PduResult;
use ironrdp_svc::impl_as_any;
use log::{debug, error, info, trace, warn};
use static_init::dynamic;
use std::fmt::{Debug, Formatter};

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
        F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + 'static,
    {
        let res = self
            .client_handle
            .write_cliprdr(Box::new(ClipboardFnInternal::new(name, f)));
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
            Some(data) => {
                if let Err(e) = self.client_handle.handle_remote_copy(data.into_bytes()) {
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
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages<Client>>;
}

impl<F> ClipboardFn for F
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + Debug + 'static,
{
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> {
        (self)(cliprdr)
    }
}

struct ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + 'static,
{
    name: &'static str,
    closure: F,
}

impl<F> ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + 'static,
{
    fn new(name: &'static str, closure: F) -> Self {
        Self { name, closure }
    }
}

impl<F> Debug for ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + 'static,
{
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", &self.name)
    }
}

impl<F> ClipboardFn for ClipboardFnInternal<F>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> + Send + 'static,
{
    fn call(&self, cliprdr: &Cliprdr) -> PduResult<CliprdrSvcMessages<Client>> {
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

fn convert_string(data: &str, format_id: ClipboardFormatId) -> Option<Vec<u8>> {
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

    use crate::cliprdr::convert_string;

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
            assert_eq!(expected, convert_string(input, format), "testing {input}",);
        }
    }
}
