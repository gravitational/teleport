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

use ironrdp_cliprdr::backend::CliprdrBackend;
use ironrdp_cliprdr::pdu::{
    ClipboardFormat, ClipboardGeneralCapabilityFlags, FileContentsRequest, FileContentsResponse,
    FormatDataRequest, FormatDataResponse, LockDataId,
};
use ironrdp_cliprdr::{Cliprdr, CliprdrSvcMessages};
use ironrdp_pdu::PduResult;

use crate::client::{ClientFunction, ClientHandle};

#[derive(Debug)]
pub struct TeleportCliprdrBackend {
    client_handle: ClientHandle,
}

impl TeleportCliprdrBackend {
    pub fn new(client_handle: ClientHandle) -> Self {
        Self { client_handle }
    }

    fn send<F>(&self, name: &str, f: F)
    where
        F: FnOnce(&Cliprdr, Option<String>) -> PduResult<CliprdrSvcMessages> + Send + 'static,
    {
        let f = Box::new(ClipboardFnInternal::new(name, f));
        let res = self
            .client_handle
            .blocking_send(ClientFunction::WriteCliprdr(f));
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

        if let Err(e) = self
            .client_handle
            .blocking_send(WriteCliprdr(RequestFormatList))
        {
            error!("Couldn't send request format list message: {:?}", e);
        }
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
        if let Err(e) = self
            .client_handle
            .blocking_send(WriteCliprdr(RemoteCopy(available_formats.to_vec())))
        {
            error!("Couldn't send remote copy message: {:?}", e);
        }
    }

    fn on_format_data_request(&mut self, format: FormatDataRequest) {
        trace!("CLIPRDR: on_format_data_request");
        if let Err(e) =
            self.client_handle
                .blocking_send(WriteCliprdr(ClipboardFunction::FormatDataRequest(
                    format.format,
                )))
        {
            error!("Couldn't send format data request message: {:?}", e);
        }
    }

    fn on_format_data_response(&mut self, response: FormatDataResponse) {
        trace!("CLIPRDR: on_format_data_response");
        if !response.is_error() {
            if let Err(e) = self.client_handle.blocking_send(WriteCliprdr(
                ClipboardFunction::FormatDataResponse(response.data().to_vec()),
            )) {
                error!("Couldn't send format data response message: {:?}", e);
            }
        }
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
    F: Fn(&Cliprdr, &Option<String>) -> PduResult<CliprdrSvcMessages> + Send + 'static,
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
