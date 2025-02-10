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

mod filesystem;
mod flags;
pub(crate) mod path;
pub(crate) mod scard;
pub(crate) mod tdp;

use self::filesystem::FilesystemBackend;
use self::scard::{ScardBackend, SCARD_DEVICE_ID};
use self::tdp::{
    SharedDirectoryCreateResponse, SharedDirectoryDeleteResponse, SharedDirectoryInfoResponse,
    SharedDirectoryListResponse,
};
use crate::client::ClientHandle;
use crate::CgoHandle;
use ironrdp_pdu::{custom_err, PduResult};
use ironrdp_rdpdr::pdu::efs::{
    DeviceControlRequest, NtStatus, ServerDeviceAnnounceResponse, ServerDriveIoRequest,
};
use ironrdp_rdpdr::pdu::esc::{ScardCall, ScardIoCtlCode};
use ironrdp_rdpdr::RdpdrBackend;
use ironrdp_svc::{impl_as_any, SvcMessage};

#[derive(Debug)]
pub struct TeleportRdpdrBackend {
    /// The backend for smart card redirection.
    scard: ScardBackend,
    /// The backend for directory sharing.
    fs: FilesystemBackend,
    /// Whether directory sharing is enabled.
    allow_directory_sharing: bool,
}

impl_as_any!(TeleportRdpdrBackend);

impl RdpdrBackend for TeleportRdpdrBackend {
    fn handle_server_device_announce_response(
        &mut self,
        pdu: ServerDeviceAnnounceResponse,
    ) -> PduResult<()> {
        // If the device announce for the smart card failed, return an error that will end the session.
        // Authentication is impossible without a smart card.
        if pdu.device_id == SCARD_DEVICE_ID && pdu.result_code != NtStatus::SUCCESS {
            return Err(custom_err!(TeleportRdpdrBackendError(format!(
                "ServerDeviceAnnounceResponse for smartcard failed with NtStatus: {:?}",
                pdu.result_code
            ))));
        }

        // If the device announce is not for a smart card, assume it's for a directory
        if pdu.device_id != SCARD_DEVICE_ID {
            self.fs.handle_server_device_announce_response(pdu)?;
        }

        // Nothing to send back to the server in either case
        Ok(())
    }

    fn handle_scard_call(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ScardCall,
    ) -> PduResult<()> {
        self.scard.handle(req, call)
    }

    fn handle_drive_io_request(&mut self, req: ServerDriveIoRequest) -> PduResult<Vec<SvcMessage>> {
        // If directory sharing isn't enabled, we don't advertise drive redirection as a supported
        // feature, so we should never receive a drive IO request. However this check acts as a
        // safeguard in case of a server bug or some other anomalous behavior.
        if self.allow_directory_sharing {
            self.fs.handle_rdp_drive_io_request(req)?;
            Ok(vec![])
        } else {
            Err(custom_err!(TeleportRdpdrBackendError(
                "Received a directory sharing PDU but directory sharing is not enabled".to_string()
            )))
        }
    }
}

impl TeleportRdpdrBackend {
    pub fn new(
        client_handle: ClientHandle,
        cert_der: Vec<u8>,
        key_der: Vec<u8>,
        pin: String,
        cgo_handle: CgoHandle,
        allow_directory_sharing: bool,
    ) -> Self {
        Self {
            scard: ScardBackend::new(client_handle.clone(), cert_der, key_der, pin),
            fs: FilesystemBackend::new(cgo_handle, client_handle),
            allow_directory_sharing,
        }
    }

    pub fn handle_tdp_sd_info_response(
        &mut self,
        tdp_resp: SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_info_response(tdp_resp)
    }

    pub fn handle_tdp_sd_create_response(
        &mut self,
        tdp_resp: SharedDirectoryCreateResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_create_response(tdp_resp)
    }

    pub fn handle_tdp_sd_delete_response(
        &mut self,
        tdp_resp: SharedDirectoryDeleteResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_delete_response(tdp_resp)
    }

    pub fn handle_tdp_sd_list_response(
        &mut self,
        tdp_resp: SharedDirectoryListResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_list_response(tdp_resp)
    }

    pub fn handle_tdp_sd_read_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryReadResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_read_response(tdp_resp)
    }

    pub fn handle_tdp_sd_write_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryWriteResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_write_response(tdp_resp)
    }

    pub fn handle_tdp_sd_move_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryMoveResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_move_response(tdp_resp)
    }

    pub fn handle_tdp_sd_truncate_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryTruncateResponse,
    ) -> PduResult<()> {
        self.fs.handle_tdp_sd_truncate_response(tdp_resp)
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
