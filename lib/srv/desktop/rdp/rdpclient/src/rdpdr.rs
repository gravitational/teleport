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

mod flags;
pub(crate) mod path;
pub(crate) mod scard;
pub(crate) mod tdp;

use self::scard::ScardBackend;
use self::tdp::{SharedDirectoryInfoRequest, SharedDirectoryInfoResponse};
use crate::client::{ClientFunction, ClientHandle};
use crate::{tdp_sd_info_request, CGOErrCode, CGOSharedDirectoryInfoRequest, CgoHandle};
use ironrdp_pdu::{custom_err, PduResult};
use ironrdp_rdpdr::pdu::efs::{
    DeviceControlRequest, DeviceCreateRequest, FilesystemRequest, NtStatus,
    ServerDeviceAnnounceResponse,
};
use ironrdp_rdpdr::pdu::esc::{ScardCall, ScardIoCtlCode};
use ironrdp_rdpdr::pdu::RdpdrPdu;
use ironrdp_rdpdr::RdpdrBackend;
use ironrdp_svc::impl_as_any;
use std::collections::HashMap;

#[derive(Debug)]
pub struct TeleportRdpdrBackend {
    /// The client handle for this backend, used to send messages to the RDP server.
    client_handle: ClientHandle,
    scard: ScardBackend,
    cgo_handle: CgoHandle,

    /// CompletionId -> SharedDirectoryInfoResponseHandler
    pending_sd_info_resp_handlers: HashMap<u32, SharedDirectoryInfoResponseHandler>,
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
            self.write_rdpdr(RdpdrPdu::DeviceControlResponse(resp))
        } else {
            Ok(())
        }
    }

    fn handle_fs_request(&mut self, req: FilesystemRequest) -> PduResult<()> {
        match req {
            FilesystemRequest::DeviceCreateRequest(req) => self.handle_fs_device_create(req),
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
    ) -> Self {
        Self {
            client_handle,
            scard: ScardBackend::new(cert_der, key_der, pin),
            cgo_handle,
            pending_sd_info_resp_handlers: HashMap::new(),
        }
    }

    fn tdp_sd_info_request(&self, req: SharedDirectoryInfoRequest) -> PduResult<()> {
        debug!("{:?}", req);
        let c_string = req.path.to_cstring()?;
        unsafe {
            let err = tdp_sd_info_request(
                self.cgo_handle,
                &mut CGOSharedDirectoryInfoRequest {
                    completion_id: req.completion_id,
                    directory_id: req.directory_id,
                    path: c_string.as_ptr(),
                },
            );
            if err != CGOErrCode::ErrCodeSuccess {
                TeleportRdpdrBackendError(format!(
                    "failed to send TDP Shared Directory Info Request: {:?}",
                    err
                ));
            };
        }
        Ok(())
    }

    pub fn handle_tdp_sd_info_response(
        &mut self,
        tdp_res: SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        if let Some((rdp_req, handler)) = self
            .pending_sd_info_resp_handlers
            .remove(&tdp_res.completion_id)
        {
            let rdp_responses = handler(self, rdp_req, tdp_res)?;
            return Ok(rdp_responses);
        }

        // Err(try_error(&format!(
        //     "received invalid completion id: {}",
        //     tdp_res.completion_id
        // )))

        Ok(())
    }

    fn handle_tdp_sd_info_response_internal(
        &mut self,
        rdp_req: DeviceCreateRequest,
        tdp_res: SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        // match tdp_res.err_code {
        //     TdpErrCode::Failed | TdpErrCode::AlreadyExists => {
        //         return Err(try_error(&format!(
        //             "received unexpected TDP error code in SharedDirectoryInfoResponse: {:?}",
        //             tdp_res.err_code,
        //         )));
        //     }
        //     TdpErrCode::Nil => {
        //         // The file exists
        //         // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L214
        //         if tdp_res.fso.file_type == FileType::Directory {
        //             if rdp_req.create_disposition == flags::CreateDisposition::FILE_CREATE {
        //                 // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L221
        //                 return cli.prep_device_create_response(
        //                     &rdp_req,
        //                     NTSTATUS::STATUS_OBJECT_NAME_COLLISION,
        //                     0,
        //                 );
        //             }

        //             if rdp_req
        //                 .create_options
        //                 .contains(flags::CreateOptions::FILE_NON_DIRECTORY_FILE)
        //             {
        //                 // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L227
        //                 return cli.prep_device_create_response(
        //                     &rdp_req,
        //                     NTSTATUS::STATUS_ACCESS_DENIED,
        //                     0,
        //                 );
        //             }
        //         } else if rdp_req
        //             .create_options
        //             .contains(flags::CreateOptions::FILE_DIRECTORY_FILE)
        //         {
        //             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L237
        //             return cli.prep_device_create_response(
        //                 &rdp_req,
        //                 NTSTATUS::STATUS_NOT_A_DIRECTORY,
        //                 0,
        //             );
        //         }
        //     }
        //     TdpErrCode::DoesNotExist => {
        //         // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L242
        //         if rdp_req
        //             .create_options
        //             .contains(flags::CreateOptions::FILE_DIRECTORY_FILE)
        //         {
        //             if rdp_req.create_disposition.intersects(
        //                 flags::CreateDisposition::FILE_OPEN_IF
        //                     | flags::CreateDisposition::FILE_CREATE,
        //             ) {
        //                 // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L252
        //                 return cli.tdp_sd_create(rdp_req, FileType::Directory);
        //             } else {
        //                 // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L258
        //                 return cli.prep_device_create_response(
        //                     &rdp_req,
        //                     NTSTATUS::STATUS_NO_SUCH_FILE,
        //                     0,
        //                 );
        //             }
        //         }
        //     }
        // }
        Ok(())
    }

    fn handle_fs_device_create(&mut self, rdp_req: DeviceCreateRequest) -> PduResult<()> {
        // Send a TDP Shared Directory Info Request
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L210
        let tdp_req = SharedDirectoryInfoRequest::from(&rdp_req);
        self.tdp_sd_info_request(tdp_req)?;

        // Add a [`SharedDirectoryInfoResponseHandler`] to the handler cache.
        // When we receive a TDP [`SharedDirectoryInfoResponse`] with this `completion_id`
        // (the result of the `self.tdp_sd_info_request(tdp_req)` call above), this handler
        // will be called.
        self.pending_sd_info_resp_handlers.insert(
            rdp_req.device_io_request.completion_id,
            (
                rdp_req,
                |_self: &mut Self,
                 rdp_req: DeviceCreateRequest,
                 tdp_res: SharedDirectoryInfoResponse|
                 -> PduResult<()> { Ok(()) },
            ),
        );

        // Add a TDP Shared Directory Info Response handler to the handler cache.
        // When we receive a TDP Shared Directory Info Response with this completion_id,
        // this handler will be called.
        // self.pending_sd_info_resp_handlers.insert(
        //     rdp_req.device_io_request.completion_id,
        //     Box::new(
        //         |cli: &mut Self, res: SharedDirectoryInfoResponse| -> PduResult<()> {
        //             match res.err_code {
        //                 TdpErrCode::Failed | TdpErrCode::AlreadyExists => {
        //                     return Err(try_error(&format!(
        //                         "received unexpected TDP error code in SharedDirectoryInfoResponse: {:?}",
        //                         res.err_code,
        //                     )));
        //                 }
        //                 TdpErrCode::Nil => {
        //                     // The file exists
        //                     // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L214
        //                     if res.fso.file_type == FileType::Directory {
        //                         if rdp_req.create_disposition
        //                             == flags::CreateDisposition::FILE_CREATE
        //                         {
        //                             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L221
        //                             return cli.prep_device_create_response(
        //                                 &rdp_req,
        //                                 NTSTATUS::STATUS_OBJECT_NAME_COLLISION,
        //                                 0,
        //                             );
        //                         }

        //                         if rdp_req
        //                             .create_options
        //                             .contains(flags::CreateOptions::FILE_NON_DIRECTORY_FILE)
        //                         {
        //                             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L227
        //                             return cli.prep_device_create_response(
        //                                 &rdp_req,
        //                                 NTSTATUS::STATUS_ACCESS_DENIED,
        //                                 0,
        //                             );
        //                         }
        //                     } else if rdp_req
        //                         .create_options
        //                         .contains(flags::CreateOptions::FILE_DIRECTORY_FILE)
        //                     {
        //                         // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L237
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_NOT_A_DIRECTORY,
        //                             0,
        //                         );
        //                     }
        //                 }
        //                 TdpErrCode::DoesNotExist => {
        //                     // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L242
        //                     if rdp_req
        //                         .create_options
        //                         .contains(flags::CreateOptions::FILE_DIRECTORY_FILE)
        //                     {
        //                         if rdp_req.create_disposition.intersects(
        //                             flags::CreateDisposition::FILE_OPEN_IF
        //                                 | flags::CreateDisposition::FILE_CREATE,
        //                         ) {
        //                             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L252
        //                             return cli.tdp_sd_create(
        //                                 rdp_req,
        //                                 FileType::Directory,
        //                             );
        //                         } else {
        //                             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L258
        //                             return cli.prep_device_create_response(
        //                                 &rdp_req,
        //                                 NTSTATUS::STATUS_NO_SUCH_FILE,
        //                                 0,
        //                             );
        //                         }
        //                     }
        //                 }
        //             }

        //             // The actual creation of files and error mapping in FreeRDP happens here, for reference:
        //             // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/file.c#L781
        //             match rdp_req.create_disposition {
        //                 flags::CreateDisposition::FILE_SUPERSEDE => {
        //                     // If the file already exists, replace it with the given file. If it does not, create the given file.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         return cli.tdp_sd_overwrite(rdp_req);
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.tdp_sd_create(rdp_req, FileType::File);
        //                     }
        //                 }
        //                 flags::CreateDisposition::FILE_OPEN => {
        //                     // If the file already exists, open it instead of creating a new file. If it does not, fail the request and do not create a new file.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         let file_id = cli.generate_file_id();
        //                         cli.file_cache.insert(
        //                             file_id,
        //                             FileCacheObject::new(UnixPath::from(&rdp_req.path), res.fso),
        //                         );
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_SUCCESS,
        //                             file_id,
        //                         );
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_NO_SUCH_FILE,
        //                             0,
        //                         )
        //                     }
        //                 }
        //                 flags::CreateDisposition::FILE_CREATE => {
        //                     // If the file already exists, fail the request and do not create or open the given file. If it does not, create the given file.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_OBJECT_NAME_COLLISION,
        //                             0,
        //                         );
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.tdp_sd_create(rdp_req, FileType::File);
        //                     }
        //                 }
        //                 flags::CreateDisposition::FILE_OPEN_IF => {
        //                     // If the file already exists, open it. If it does not, create the given file.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         let file_id = cli.generate_file_id();
        //                         cli.file_cache.insert(
        //                             file_id,
        //                             FileCacheObject::new(UnixPath::from(&rdp_req.path), res.fso),
        //                         );
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_SUCCESS,
        //                             file_id,
        //                         );
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.tdp_sd_create(rdp_req, FileType::File);
        //                     }
        //                 }
        //                 flags::CreateDisposition::FILE_OVERWRITE => {
        //                     // If the file already exists, open it and overwrite it. If it does not, fail the request.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         return cli.tdp_sd_overwrite(rdp_req);
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.prep_device_create_response(
        //                             &rdp_req,
        //                             NTSTATUS::STATUS_NO_SUCH_FILE,
        //                             0,
        //                         )
        //                     }
        //                 }
        //                 flags::CreateDisposition::FILE_OVERWRITE_IF => {
        //                     // If the file already exists, open it and overwrite it. If it does not, create the given file.
        //                     if res.err_code == TdpErrCode::Nil {
        //                         return cli.tdp_sd_overwrite(rdp_req);
        //                     } else if res.err_code == TdpErrCode::DoesNotExist {
        //                         return cli.tdp_sd_create(rdp_req, FileType::File);
        //                     }
        //                 }
        //                 _ => {
        //                     return Err(invalid_data_error(&format!(
        //                         "received unknown CreateDisposition value for RDP {rdp_req:?}"
        //                     )));
        //                 }
        //             }

        //             Err(try_error("Programmer error, this line should never be reached"))
        //         },
        //     ),
        // );

        Ok(())
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

type SharedDirectoryInfoResponseHandler = (
    DeviceCreateRequest,
    fn(
        &mut TeleportRdpdrBackend,
        DeviceCreateRequest,
        SharedDirectoryInfoResponse,
    ) -> PduResult<()>,
);
