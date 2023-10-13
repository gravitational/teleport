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

use std::fmt::{Debug, Formatter};
use std::ops::Deref;
use std::sync::{Arc, Mutex};

use byteorder::LittleEndian;
use ironrdp_cliprdr::backend::CliprdrBackend;
use ironrdp_cliprdr::pdu::{
    ClipboardFormat, ClipboardFormatId, ClipboardGeneralCapabilityFlags, FileContentsRequest,
    FileContentsResponse, FormatDataRequest, FormatDataResponse, LockDataId,
};
use ironrdp_cliprdr::{Cliprdr, CliprdrSvcMessages};
use ironrdp_pdu::PduResult;
use utf16string::WString;

use crate::client::{ClientFunction, ClientHandle};

#[derive(Debug)]
pub struct TeleportCliprdrBackend {
    client_handle: ClientHandle,
    clipboard_data: Arc<Mutex<Option<String>>>,
}

impl TeleportCliprdrBackend {
    pub fn new(client_handle: ClientHandle, clipboard_data: Arc<Mutex<Option<String>>>) -> Self {
        Self {
            client_handle,
            clipboard_data,
        }
    }

    fn send<F>(&self, name: &'static str, f: F)
    where
        F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
    {
        let res = self
            .client_handle
            .blocking_send(ClientFunction::WriteCliprdr(as_clipboard_fn(name, f)));
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
        let formats = available_formats(self.clipboard_data.lock().unwrap().deref());
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
        // always use CF_UNICODETEXT if available
        let mut format = available_formats
            .iter()
            .find(|cf| cf.id() == ClipboardFormatId::CF_UNICODETEXT);
        // if not fallback to CF_TEXT
        if format.is_none() {
            format = available_formats
                .iter()
                .find(|cf| cf.id() == ClipboardFormatId::CF_TEXT);
        }
        if let Some(format) = format.map(ClipboardFormat::id) {
            self.send("remote_copy", move |c| c.initiate_paste(format))
        }
    }

    fn on_format_data_request(&mut self, format: FormatDataRequest) {
        trace!("CLIPRDR: on_format_data_request");
        let response = self
            .clipboard_data
            .lock()
            .unwrap()
            .as_ref()
            .and_then(|data| match format.format {
                ClipboardFormatId::CF_UNICODETEXT => {
                    let utf16: WString<LittleEndian> = data.as_str().into();
                    let mut utf16 = utf16.into_bytes();
                    utf16.extend_from_slice(&[0u8, 0u8]);
                    Some(utf16)
                }
                ClipboardFormatId::CF_TEXT => {
                    if !data.is_ascii() {
                        return None;
                    }
                    let mut data = data.clone().into_bytes();
                    data.push(0u8);
                    Some(data)
                }
                _ => None,
            });
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
        if response.is_error() {
            error!("Received error in format_data_response");
            return;
        }
        let mut data = response.data().to_vec();
        let data = if data.ends_with(&[0u8, 0u8]) {
            WString::from_utf16le(data)
                .map(|s| s.to_utf8())
                .map_err(|_| ())
        } else {
            String::from_utf8(data).map_err(|_| ())
        };
        match data {
            Ok(mut data) => {
                data.pop();
                if let Err(e) = self
                    .client_handle
                    .blocking_send(ClientFunction::HandleRemoteCopy(data.into_bytes()))
                {
                    error!("Can't send format_data_response: {:?}", e);
                }
            }
            Err(_) => {
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
    match data {
        Some(s) => {
            let mut formats = vec![ClipboardFormat::new(ClipboardFormatId::CF_UNICODETEXT)];
            if s.is_ascii() {
                formats.push(ClipboardFormat::new(ClipboardFormatId::CF_TEXT))
            }
            formats
        }
        None => vec![],
    }
}

pub fn as_clipboard_fn<F>(name: &'static str, f: F) -> Box<dyn ClipboardFn>
where
    F: Fn(&Cliprdr) -> PduResult<CliprdrSvcMessages> + Send + 'static,
{
    Box::new(ClipboardFnInternal::new(name, f))
}
