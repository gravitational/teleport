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

use ironrdp_cliprdr::backend::CliprdrBackend;
use ironrdp_cliprdr::pdu::{
    ClipboardFormat, ClipboardFormatId, ClipboardGeneralCapabilityFlags, FileContentsRequest,
    FileContentsResponse, FormatDataRequest, FormatDataResponse, LockDataId,
};

use crate::client::ClientFunction::HandleClipboard;
use crate::client::ClientHandle;
use crate::cliprdr::ClipboardFunction::{RemoteCopy, RequestFormatList};

#[derive(Debug)]
pub struct TeleportCliprdrBackend {
    client_handle: ClientHandle,
}

impl TeleportCliprdrBackend {
    pub fn new(client_handle: ClientHandle) -> Self {
        Self { client_handle }
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
            .blocking_send(HandleClipboard(RequestFormatList))
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
            .blocking_send(HandleClipboard(RemoteCopy(available_formats.to_vec())))
        {
            error!("Couldn't send remote copy message: {:?}", e);
        }
    }

    fn on_format_data_request(&mut self, format: FormatDataRequest) {
        trace!("CLIPRDR: on_format_data_request");
        if let Err(e) =
            self.client_handle
                .blocking_send(HandleClipboard(ClipboardFunction::FormatDataRequest(
                    format.format,
                )))
        {
            error!("Couldn't send format data request message: {:?}", e);
        }
    }

    fn on_format_data_response(&mut self, response: FormatDataResponse) {
        trace!("CLIPRDR: on_format_data_response");
        if !response.is_error() {
            if let Err(e) = self.client_handle.blocking_send(HandleClipboard(
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

#[derive(Debug)]
pub enum ClipboardFunction {
    RequestFormatList,
    RemoteCopy(Vec<ClipboardFormat>),
    FormatDataResponse(Vec<u8>),
    FormatDataRequest(ClipboardFormatId),
    Update(String),
}
