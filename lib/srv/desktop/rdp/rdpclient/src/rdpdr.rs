// Copyright 2021 Gravitational, Inc
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

mod filesystem;
mod flags;
pub(crate) mod path;
pub(crate) mod scard;
pub(crate) mod tdp;

use self::filesystem::FilesystemBackend;
use self::scard::ScardBackend;
use self::tdp::SharedDirectoryInfoResponse;
use crate::client::{ClientFunction, ClientHandle};
use crate::CgoHandle;
use ironrdp_pdu::{custom_err, PduResult};
use ironrdp_rdpdr::pdu::efs::{
    DeviceControlRequest, FilesystemRequest, NtStatus, ServerDeviceAnnounceResponse,
};
use ironrdp_rdpdr::pdu::esc::{ScardCall, ScardIoCtlCode};
use ironrdp_rdpdr::pdu::RdpdrPdu;
use ironrdp_rdpdr::RdpdrBackend;
use ironrdp_svc::impl_as_any;

#[derive(Debug)]
pub struct TeleportRdpdrBackend {
    /// The client handle for this backend, used to send messages to the RDP server.
    client_handle: ClientHandle,
    /// The backend for smart card redirection.
    scard: ScardBackend,
    /// The backend for directory sharing.
    fs: FilesystemBackend,
}

impl_as_any!(TeleportRdpdrBackend);

impl RdpdrBackend for TeleportRdpdrBackend {
    fn handle_server_device_announce_response(
        &mut self,
        pdu: ServerDeviceAnnounceResponse,
    ) -> PduResult<()> {
        if pdu.result_code != NtStatus::Success {
            return Err(custom_err!(
                "TeleportRdpdrBackend::handle_server_device_announce_response",
                TeleportRdpdrBackendError(format!(
                    "ServerDeviceAnnounceResponse failed with NtStatus: {:?}",
                    pdu.result_code
                ))
            ));
        }

        // Nothing to send back to the server
        Ok(())
    }

    fn handle_scard_call(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ScardCall,
    ) -> PduResult<()> {
        if let Some(resp) = self.scard.handle(req, call)? {
            self.write_rdpdr(resp.into())
        } else {
            // Nothing to send back to the server
            Ok(())
        }
    }

    fn handle_fs_request(&mut self, req: FilesystemRequest) -> PduResult<()> {
        self.fs.handle(req)
    }
}

impl TeleportRdpdrBackend {
    pub fn new(
        client_handle: ClientHandle,
        cert_der: Vec<u8>,
        key_der: Vec<u8>,
        pin: String,
        cgo_handle: CgoHandle,
    ) -> Self {
        Self {
            client_handle,
            scard: ScardBackend::new(cert_der, key_der, pin),
            fs: FilesystemBackend::new(cgo_handle),
        }
    }

    pub fn handle_tdp_sd_info_response(
        &mut self,
        tdp_res: SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        if let Some(resp) = self.fs.handle_tdp_sd_info_response(tdp_res)? {
            self.write_rdpdr(resp)
        } else {
            // Nothing to send back to the server
            Ok(())
        }
    }

    fn write_rdpdr(&mut self, pdu: RdpdrPdu) -> PduResult<()> {
        self.client_handle
            .blocking_send(ClientFunction::WriteRdpdr(pdu))
            .map_err(|e| {
                custom_err!(
                    "TeleportRdpdrBackend::write_rdpdr",
                    // Due to a long chain of trait dependencies in IronRDP that are impractical to unwind at this point,
                    // we can't put _e in the source field of the error because it isn't Sync (because ClientFunction itself
                    // isn't sync). We compromise here by just wrapping its Debug output in a TeleportRdpdrBackendError.
                    TeleportRdpdrBackendError(format!("{:?}", e))
                )
            })
    }
}

/// A generic error type for the TeleportRdpdrBackend that can contain any arbitrary error message.
#[derive(Debug)]
pub struct TeleportRdpdrBackendError(pub String);

impl std::fmt::Display for TeleportRdpdrBackendError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for TeleportRdpdrBackendError {}
