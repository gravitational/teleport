// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

use super::{
    path::UnixPath,
    tdp::{self, TdpErrCode},
};
use crate::client::{ClientError, ClientResult};
use crate::{
    cgo_tdp_sd_acknowledge, cgo_tdp_sd_create_request, cgo_tdp_sd_delete_request,
    cgo_tdp_sd_info_request, cgo_tdp_sd_list_request, cgo_tdp_sd_move_request,
    cgo_tdp_sd_read_request, cgo_tdp_sd_truncate_request, cgo_tdp_sd_write_request,
    client::ClientHandle, rdpdr::tdp::SharedDirectoryRemove, CGOErrCode, CgoHandle,
};
use ironrdp_core::{cast_length, EncodeError};
use ironrdp_pdu::PduResult;
use ironrdp_pdu::{pdu_other_err, PduError, PduErrorExt};
use ironrdp_rdpdr::pdu::{
    self,
    efs::{self, DeviceIoResponse, NtStatus},
    esc, RdpdrPdu,
};
use log::{debug, trace, warn};
use std::convert::TryInto;
use std::fmt::Debug;
use std::{collections::HashMap, vec};

pub(crate) fn cast_length<T, S: TryInto<T, Error: Debug>>(
    ctx: &str,
    field: &str,
    s: S,
) -> ClientResult<T> {
    s.try_into().map_err(|e| {
        ClientError::InternalError(format!("{}: can't convert {}: {:?}", ctx, field, e))
    })
}

#[derive(Debug)]
struct DirectoryContext {
    /// FileId-indexed cache of [`FileCacheObject`]s.
    ///
    /// See the documentation for [`FileCacheObject`].
    file_cache: FileCache,
    /// marked_for_deletion indicates that this DirectoryContext has been
    /// marked for deletion. All inbound requests will be canceled.
    marked_for_deletion: bool,
    /// response_cache holds all pending I/O response handlers for
    /// I/O requests against this DirectoryContext.
    response_cache: ResponseCache,
}

impl DirectoryContext {
    fn new() -> Self {
        DirectoryContext {
            file_cache: FileCache::new(),
            marked_for_deletion: false,
            response_cache: ResponseCache::new(),
        }
    }

    fn insert_file(&mut self, file: FileCacheObject) -> Result<u32, FilesystemBackendError> {
        let path = file.fso.path.path.clone();
        self.file_cache.insert(file).inspect(|id| {
            debug!(
                "inserted file id: {}, path: {} file_entries {}",
                id,
                path,
                self.file_cache.cache.len()
            )
        })
    }

    fn remove_file(&mut self, file_id: u32) -> Option<FileCacheObject> {
        self.file_cache.remove(file_id).inspect(|fco| {
            debug!(
                "removed file id: {}, path: {} file_entries {}",
                file_id,
                &fco.path().path,
                self.file_cache.cache.len()
            )
        })
    }

    fn get_file(&self, file_id: u32) -> Option<&FileCacheObject> {
        self.file_cache.get(file_id)
    }

    fn get_file_mut(&mut self, file_id: u32) -> Option<&mut FileCacheObject> {
        self.file_cache.get_mut(file_id)
    }

    fn insert_handler(
        &mut self,
        completion_id: CompletionId,
        handler: ResponseKind,
    ) -> Result<(), FilesystemBackendError> {
        self.response_cache.insert(completion_id, handler)
    }

    fn remove_handler(&mut self, completion_id: CompletionId) -> Option<ResponseKind> {
        self.response_cache.remove(&completion_id)
    }
}

#[derive(Debug)]
struct DirectoryCache(HashMap<u32, DirectoryContext>);

impl DirectoryCache {
    fn new() -> Self {
        Self(HashMap::new())
    }

    fn get_context_mut(
        &mut self,
        device_id: u32,
    ) -> Result<&mut DirectoryContext, FilesystemBackendError> {
        self.0.get_mut(&device_id).ok_or(FilesystemBackendError(
            format!("filesytem device with id {} not found", device_id).to_string(),
        ))
    }

    fn get_context(&self, device_id: u32) -> Result<&DirectoryContext, FilesystemBackendError> {
        self.0.get(&device_id).ok_or(FilesystemBackendError(
            format!("filesytem device with id {} not found", device_id).to_string(),
        ))
    }

    fn add_device(&mut self, device_id: u32) -> Result<(), FilesystemBackendError> {
        if self.0.contains_key(&device_id) {
            return Err(FilesystemBackendError(format!(
                "device {} already exists",
                device_id
            )));
        }

        self.0.insert(device_id, DirectoryContext::new());
        Ok(())
    }

    // Remove device. Clear all pending handlers
    fn remove_device(
        &mut self,
        device_id: u32,
    ) -> Result<DirectoryContext, FilesystemBackendError> {
        self.0
            .remove(&device_id)
            .ok_or(FilesystemBackendError(format!(
                "cannot remove unknown deviceId {}",
                device_id
            )))
    }

    fn add_file(
        &mut self,
        device_id: u32,
        file: FileCacheObject,
    ) -> Result<u32, FilesystemBackendError> {
        self.get_context_mut(device_id)
            .inspect_err(|e| warn!("Failed to add file: {}", e))?
            .insert_file(file)
    }

    fn remove_file(&mut self, device_id: u32, file_id: u32) -> Option<FileCacheObject> {
        self.get_context_mut(device_id)
            .inspect_err(|e| warn!("Failed to remove file: {}", e))
            .ok()?
            .remove_file(file_id)
    }

    fn get_file(&self, device_id: u32, file_id: u32) -> Option<&FileCacheObject> {
        self.get_context(device_id)
            .inspect_err(|e| warn!("Failed to retreive file: {}", e))
            .ok()?
            .get_file(file_id)
    }

    fn get_file_mut(&mut self, device_id: u32, file_id: u32) -> Option<&mut FileCacheObject> {
        self.get_context_mut(device_id)
            .inspect_err(|e| warn!("Failed to retreive file: {}", e))
            .ok()?
            .get_file_mut(file_id)
    }

    fn insert_handler<H: Into<ResponseKind>>(
        &mut self,
        device_id: u32,
        completion_id: CompletionId,
        handler: H,
    ) -> Result<(), FilesystemBackendError> {
        self.get_context_mut(device_id)
            .inspect_err(|e| warn!("Failed to insert response handler: {}", e))?
            .insert_handler(completion_id, handler.into())
    }

    fn remove_handler<H>(
        &mut self,
        device_id: u32,
        completion_id: CompletionId,
    ) -> Result<H, FilesystemBackendError>
    where
        H: TryFrom<ResponseKind>,
        H::Error: Into<FilesystemBackendError>,
    {
        let handler = self
            .get_context_mut(device_id)?
            .remove_handler(completion_id)
            .ok_or(FilesystemBackendError(
                "failed to remove response handler".to_string(),
            ))?;

        H::try_from(handler).map_err(|op| op.into())
    }
}

/// `FilesystemBackend` implements the filesystem redirection backend as described in [\[MS-RDPEFS\]: Remote Desktop Protocol: File System Virtual Channel Extension].
/// It does so in concert with the TDP directory sharing extension described in [RFD 0067].
///
/// [\[MS-RDPEFS\]: Remote Desktop Protocol: File System Virtual Channel Extension]: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/34d9de58-b2b5-40b6-b970-f82d4603bdb5
/// [RFD 0067]: https://github.com/gravitational/teleport/blob/master/rfd/0067-desktop-access-file-system-sharing.md
#[derive(Debug)]
pub struct FilesystemBackend {
    cgo_handle: CgoHandle,
    client_handle: ClientHandle,
    cache: DirectoryCache,
}

impl FilesystemBackend {
    pub fn new(cgo_handle: CgoHandle, client_handle: ClientHandle) -> Self {
        Self {
            cgo_handle,
            client_handle,
            cache: DirectoryCache::new(),
        }
    }

    pub fn add_device(&mut self, device_id: u32) -> PduResult<()> {
        // Create/insert new cache for this device.
        trace!("Adding device cache for device: {}", device_id);
        self.cache.add_device(device_id).map_err(|err| err.into())
    }

    // Cancels all pending I/O requests on this device and marks the device as
    // for deletion, which means all pending I/O requests will be automatically
    // cancelled. This function returns Ok(true) and actually removes the device
    // if it has no open file handlers, otherwise it returns Ok(false) and removal
    // must be retried after closing any open handles.
    pub fn mark_device_for_deletion(&mut self, device_id: u32) -> PduResult<(Vec<RdpdrPdu>, bool)> {
        let directory_context = self.cache.get_context_mut(device_id)?;
        // Drain all pending I/O handlers and collect a list of cancellation responses.
        let cancel_pdus: Vec<RdpdrPdu> = directory_context
            .response_cache
            .drain()
            .map(|(_completion, handler)| handler.cancel())
            .collect();

        // Mark as pending deletion. The FilesystemBackend will inspect this flag
        // and automatically cancel any new I/O requests from the server.
        directory_context.marked_for_deletion = true;

        // If the file cache is empty, then we can remove the device right away.
        // Otherwise, we'll wait for the next device close request to try again.
        // Pending I/O responses will be sent immediately regardless.
        if self
            .cache
            .get_context_mut(device_id)?
            .file_cache
            .cache
            .is_empty()
        {
            // File cache is empty. It's safe to remove this device.
            self.cache.remove_device(device_id)?;
            Ok((cancel_pdus, true))
        } else {
            // File cache is not empty. We'll need to wait for the server to
            // close any open file handles first.
            Ok((cancel_pdus, false))
        }
    }

    /// Handles an RDP [`efs::ServerDeviceAnnounceResponse`] received from the RDP server.
    pub fn handle_server_device_announce_response(
        &mut self,
        res: efs::ServerDeviceAnnounceResponse,
    ) -> PduResult<()> {
        // TODO(zmb3): send the underlying NTSTATUS code instead
        // of converting everything to 0 or 1.
        let err_code = match res.result_code {
            NtStatus::SUCCESS => TdpErrCode::Nil,
            _ => TdpErrCode::Failed,
        };

        if err_code == TdpErrCode::Failed {
            warn!(
                "Directory sharing failed, server sent error {:?}",
                res.result_code
            )
        }

        self.send_tdp_sd_acknowledge(tdp::SharedDirectoryAcknowledge {
            err_code,
            directory_id: res.device_id,
        })
    }

    /// Handles an RDP [`efs::ServerDriveIoRequest`] received from the RDP server.
    pub fn handle_rdp_drive_io_request(&mut self, req: efs::ServerDriveIoRequest) -> PduResult<()> {
        trace!("received {:?}", &req);

        let device_pending_deletion = self
            .cache
            .get_context(DeviceId::from(&req).into())?
            .marked_for_deletion;
        if device_pending_deletion {
            // A device pending deletion is only allowed to handle device close requests.
            // All other requests are cancelled.
            match req {
                efs::ServerDriveIoRequest::DeviceCloseRequest(_) => {}
                _ => {
                    return self
                        .client_handle
                        .write_rdpdr(req.cancel())
                        .map_err(|err| err.into())
                }
            }
        }

        match req {
            efs::ServerDriveIoRequest::ServerCreateDriveRequest(req) => {
                self.handle_rdp_device_create_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryInformationRequest(req) => {
                self.handle_rdp_query_information_req(req)
            }
            efs::ServerDriveIoRequest::DeviceCloseRequest(req) => {
                // If the device is marked for deletion AND this request closes that final file
                // in the cache, then we'll finally send the request to remove the device.
                let device_id = req.device_io_request.device_id;
                let res = self.handle_rdp_device_close_req(req);
                if device_pending_deletion {
                    // HACK(rhammonds): We need to remove this device id from the rdpdr ServiceProcessor,
                    // but can't obtain a reference to it because the 'rdpdr' instance already holds
                    // a reference to us. We'll synthesize a new directory removal message for the for
                    // the client to process which will attempt to remove the device/directory again.
                    let _ = self
                        .client_handle
                        .handle_tdp_sd_remove(SharedDirectoryRemove {
                            directory_id: device_id,
                        });
                }
                //Return original close response
                res
            }
            efs::ServerDriveIoRequest::ServerDriveQueryDirectoryRequest(req) => {
                self.handle_rdp_query_directory_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveNotifyChangeDirectoryRequest(req) => {
                self.handle_rdp_notify_change_directory_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryVolumeInformationRequest(req) => {
                self.handle_rdp_query_volume_req(req)
            }
            efs::ServerDriveIoRequest::DeviceControlRequest(req) => {
                self.handle_rdp_device_control_req(req)
            }
            efs::ServerDriveIoRequest::DeviceReadRequest(req) => {
                self.handle_rdp_device_read_req(req)
            }
            efs::ServerDriveIoRequest::DeviceWriteRequest(req) => {
                self.handle_rdp_device_write_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveSetInformationRequest(req) => {
                self.handle_rdp_set_information_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveLockControlRequest(req) => {
                self.handle_rdp_lock_req(req)
            }
        }
    }

    /// Handles an RDP [`efs::DeviceCreateRequest`] received from the RDP server.
    fn handle_rdp_device_create_req(&mut self, rdp_req: efs::DeviceCreateRequest) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L210
        self.send_tdp_sd_info_request(tdp::SharedDirectoryInfoRequest::from(&rdp_req))?;

        self.cache.insert_handler(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.completion_id,
            ResponseKind::Info(SharedDirectoryInfoResponseHandler::new(
                rdp_req,
                |this: &mut FilesystemBackend,
                 tdp_resp: tdp::SharedDirectoryInfoResponse,
                 rdp_req: efs::DeviceCreateRequest|
                 -> PduResult<()> {
                    this.handle_rdp_device_create_req_continued(rdp_req, tdp_resp)
                },
            )),
        )?;
        Ok(())
    }

    /// Continues [`Self::handle_rdp_device_create_req`] after a [`tdp::SharedDirectoryInfoResponse`] is received from the browser,
    /// returning any [`RdpdrPdu`]s that need to be sent back to the RDP server.
    fn handle_rdp_device_create_req_continued(
        &mut self,
        req: efs::DeviceCreateRequest,
        res: tdp::SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        match res.err_code {
            TdpErrCode::Failed | TdpErrCode::AlreadyExists => {
                return Err(pdu_other_err!(
                    "",
                    source:FilesystemBackendError(format!(
                        "received unexpected TDP error code in SharedDirectoryInfoResponse: {:?}",
                        res.err_code,
                    ))
                ));
            }
            TdpErrCode::Nil => {
                // The file exists
                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L214
                if res.fso.file_type == tdp::FileType::Directory {
                    if req.create_disposition == efs::CreateDisposition::FILE_CREATE {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L221
                        return self.send_rdp_device_create_response(
                            &req,
                            efs::NtStatus::OBJECT_NAME_COLLISION,
                            0,
                        );
                    }

                    if req
                        .create_options
                        .contains(efs::CreateOptions::FILE_NON_DIRECTORY_FILE)
                    {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L227
                        return self.send_rdp_device_create_response(
                            &req,
                            efs::NtStatus::ACCESS_DENIED,
                            0,
                        );
                    }
                } else if req
                    .create_options
                    .contains(efs::CreateOptions::FILE_DIRECTORY_FILE)
                {
                    // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L237
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::NOT_A_DIRECTORY,
                        0,
                    );
                }
            }
            TdpErrCode::DoesNotExist => {
                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L242
                if req
                    .create_options
                    .contains(efs::CreateOptions::FILE_DIRECTORY_FILE)
                {
                    if req.create_disposition == efs::CreateDisposition::FILE_OPEN_IF
                        || req.create_disposition == efs::CreateDisposition::FILE_CREATE
                    {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L252
                        self.tdp_sd_create(req, tdp::FileType::Directory)?;
                        return Ok(());
                    } else {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L258
                        return self.send_rdp_device_create_response(
                            &req,
                            efs::NtStatus::NO_SUCH_FILE,
                            0,
                        );
                    }
                }
            }
        }

        // The actual creation of files and error mapping in FreeRDP happens here, for reference:
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/file.c#L781
        match req.create_disposition {
            efs::CreateDisposition::FILE_SUPERSEDE => {
                // If the file already exists, replace it with the given file. If it does not, create the given file.
                if res.err_code == TdpErrCode::Nil {
                    self.tdp_sd_overwrite(req)?;
                    return Ok(());
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    self.tdp_sd_create(req, tdp::FileType::File)?;
                    return Ok(());
                }
            }
            efs::CreateDisposition::FILE_OPEN => {
                // If the file already exists, open it instead of creating a new file. If it does not, fail the request and do not create a new file.
                if res.err_code == TdpErrCode::Nil {
                    let file_id = self.cache.add_file(
                        req.device_io_request.device_id,
                        FileCacheObject::new(UnixPath::from(&req.path), res.fso),
                    )?;
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::SUCCESS,
                        file_id,
                    );
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::NO_SUCH_FILE,
                        0,
                    );
                }
            }
            efs::CreateDisposition::FILE_CREATE => {
                // If the file already exists, fail the request and do not create or open the given file. If it does not, create the given file.
                if res.err_code == TdpErrCode::Nil {
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::OBJECT_NAME_COLLISION,
                        0,
                    );
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    self.tdp_sd_create(req, tdp::FileType::File)?;
                    return Ok(());
                }
            }
            efs::CreateDisposition::FILE_OPEN_IF => {
                // If the file already exists, open it. If it does not, create the given file.
                if res.err_code == TdpErrCode::Nil {
                    let file_id = self.cache.add_file(
                        req.device_io_request.device_id,
                        FileCacheObject::new(UnixPath::from(&req.path), res.fso),
                    )?;
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::SUCCESS,
                        file_id,
                    );
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    self.tdp_sd_create(req, tdp::FileType::File)?;
                    return Ok(());
                }
            }
            efs::CreateDisposition::FILE_OVERWRITE => {
                // If the file already exists, open it and overwrite it. If it does not, fail the request.
                if res.err_code == TdpErrCode::Nil {
                    self.tdp_sd_overwrite(req)?;
                    return Ok(());
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    return self.send_rdp_device_create_response(
                        &req,
                        efs::NtStatus::NO_SUCH_FILE,
                        0,
                    );
                }
            }
            efs::CreateDisposition::FILE_OVERWRITE_IF => {
                // If the file already exists, open it and overwrite it. If it does not, create the given file.
                if res.err_code == TdpErrCode::Nil {
                    self.tdp_sd_overwrite(req)?;
                    return Ok(());
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    self.tdp_sd_create(req, tdp::FileType::File)?;
                    return Ok(());
                }
            }
            _ => {
                return Err(pdu_other_err!(
                    "",
                    source:FilesystemBackendError(format!(
                        "received unknown CreateDisposition value for RDP {req:?}",
                        req = req
                    ))
                ));
            }
        }

        Err(pdu_other_err!(
            "Programmer error, this line should never be reached"
        ))
    }

    /// Handles an RDP [`efs::ServerDriveQueryInformationRequest`] received from the RDP server.
    fn handle_rdp_query_information_req(
        &mut self,
        rdp_req: efs::ServerDriveQueryInformationRequest,
    ) -> PduResult<()> {
        let file = self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        );
        self.send_rdp_client_drive_query_info_response(rdp_req, file)?;
        Ok(())
    }

    /// Handles an RDP [`efs::DeviceCloseRequest`] received from the RDP server.
    fn handle_rdp_device_close_req(&mut self, rdp_req: efs::DeviceCloseRequest) -> PduResult<()> {
        if let Some(file) = self.cache.remove_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            if file.delete_pending {
                return self.tdp_sd_delete(rdp_req, file);
            }
            return self.send_rdp_device_close_response(rdp_req, NtStatus::SUCCESS);
        }

        self.send_rdp_device_close_response(rdp_req, NtStatus::UNSUCCESSFUL)
    }

    /// Handles an RDP [`efs::ServerDriveQueryDirectoryRequest`] received from the RDP server.
    fn handle_rdp_query_directory_req(
        &mut self,
        rdp_req: efs::ServerDriveQueryDirectoryRequest,
    ) -> PduResult<()> {
        let file_id = rdp_req.device_io_request.file_id;
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L610
        match self
            .cache
            .get_file(rdp_req.device_io_request.device_id, file_id)
        {
            // File not found in cache, return a failure
            None => {
                warn!("FILE NOT FOUND IN handle_rdp_query_directory_req ");
                self.send_rdp_drive_query_dir_response(
                    rdp_req.device_io_request,
                    NtStatus::UNSUCCESSFUL,
                    None,
                )
            }
            Some(dir) => {
                if dir.fso.file_type != tdp::FileType::Directory {
                    return Err(pdu_other_err!("received ServerDriveQueryDirectoryRequest request for a file rather than a directory"));
                }

                if rdp_req.initial_query == 0 {
                    // This isn't the initial query, ergo we already have this dir's contents filled in.
                    // Just send the next item.
                    return self.send_rdp_next_drive_query_dir_response(&rdp_req);
                }

                // On the initial query, we need to get the list of files in this directory from
                // the client by sending a TDP SharedDirectoryListRequest.
                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L775
                let path = dir.path.clone();

                // Ask the client for the list of files in this directory.
                self.send_tdp_sd_list_request(tdp::SharedDirectoryListRequest {
                    completion_id: rdp_req.device_io_request.completion_id,
                    directory_id: rdp_req.device_io_request.device_id,
                    path,
                })?;

                // When we get the response for that list of files...
                self.cache.insert_handler(
                    rdp_req.device_io_request.device_id,
                    rdp_req.device_io_request.completion_id,
                    SharedDirectoryListResponseHandler::new(
                        rdp_req,
                        move |cli: &mut Self,
                              tdp_resp: tdp::SharedDirectoryListResponse,
                              rdp_req: efs::ServerDriveQueryDirectoryRequest|
                              -> PduResult<()> {
                            cli.handle_rdp_query_directory_req_continued(rdp_req, tdp_resp)
                        },
                    ),
                )?;

                // Return nothing yet, an RDP message will be returned when the pending_sd_list_resp_handlers
                // closure gets called.
                Ok(())
            }
        }
    }

    /// Continues [`Self::handle_query_directory_req`] after a [`tdp::SharedDirectoryListResponse`] is received from the browser,
    /// returning any [`RdpdrPdu`]s that need to be sent back to the RDP server.
    fn handle_rdp_query_directory_req_continued(
        &mut self,
        rdp_req: efs::ServerDriveQueryDirectoryRequest,
        tdp_resp: tdp::SharedDirectoryListResponse,
    ) -> PduResult<()> {
        if tdp_resp.err_code != TdpErrCode::Nil {
            // For now any error will kill the session.
            // In the future, we might want to make this send back
            // an NTSTATUS::STATUS_UNSUCCESSFUL instead.
            return Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "SharedDirectoryListRequest failed with err_code = {:?}",
                    tdp_resp.err_code
                ))
            ));
        }

        // If SharedDirectoryListRequest succeeded, move the
        // list of FileSystemObjects that correspond to this directory's
        // contents to its entry in the file cache.
        if let Some(dir) = self.cache.get_file_mut(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            dir.contents = tdp_resp.fso_list;
            // And send back the "." directory over RDP
            return self.send_rdp_next_drive_query_dir_response(&rdp_req);
        }

        self.send_rdp_drive_query_dir_response(
            rdp_req.device_io_request,
            NtStatus::UNSUCCESSFUL,
            None,
        )
    }

    fn handle_rdp_notify_change_directory_req(
        &mut self,
        rdp_req: efs::ServerDriveNotifyChangeDirectoryRequest,
    ) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L661
        debug!("Received NotifyChangeDirectory, cancelling: {:?}", rdp_req);
        self.client_handle.write_rdpdr(rdp_req.cancel())?;
        Ok(())
    }

    /// Handles an RDP [`efs::ServerDriveQueryVolumeInformationRequest`] received from the RDP server.
    fn handle_rdp_query_volume_req(
        &mut self,
        rdp_req: efs::ServerDriveQueryVolumeInformationRequest,
    ) -> PduResult<()> {
        match self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            // File not found in cache
            None => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "failed to retrieve an item from the file cache with FileId = {}",
                    rdp_req.device_io_request.file_id
                ))
            )),
            Some(dir) => {
                let buffer: Option<efs::FileSystemInformationClass> = match rdp_req
                    .fs_info_class_lvl
                {
                    efs::FileSystemInformationClassLevel::FILE_FS_VOLUME_INFORMATION => {
                        Some(
                            efs::FileFsVolumeInformation {
                                volume_creation_time: cast_length!(
                                    "FilesystemBackend::handle_query_volume_req",
                                    "dir.fso.last_modified",
                                    dir.fso.last_modified
                                )
                                .map_err(|e: EncodeError| ClientError::from(e))?,
                                // Equivalent to `u32::MAX & 0xffff` which is what FreeRDP does between
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/file.c#L1018-L1021
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L492
                                volume_serial_number: 0xffff,
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L494
                                supports_objects: efs::Boolean::False,
                                // volume_label can just be something we make up
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L446
                                volume_label: "TELEPORT".to_string(),
                            }
                            .into(),
                        )
                    }
                    efs::FileSystemInformationClassLevel::FILE_FS_ATTRIBUTE_INFORMATION => {
                        Some(
                            efs::FileFsAttributeInformation {
                                file_system_attributes:
                                    efs::FileSystemAttributes::FILE_CASE_SENSITIVE_SEARCH
                                        | efs::FileSystemAttributes::FILE_CASE_PRESERVED_NAMES
                                        | efs::FileSystemAttributes::FILE_UNICODE_ON_DISK,
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L536
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/include/winpr/file.h#L36
                                max_component_name_len: 260,
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L447
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L519
                                // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L538
                                file_system_name: "FAT32".to_string(),
                            }
                            .into(),
                        )
                    }
                    efs::FileSystemInformationClassLevel::FILE_FS_FULL_SIZE_INFORMATION => Some(
                        // Fill these out with the default fallback values FreeRDP uses
                        // Written here: https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L552-L557
                        // With default fallback values ultimately found here:
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/file.c#L1018-L1021
                        efs::FileFsFullSizeInformation {
                            total_alloc_units: u32::MAX as i64,
                            caller_available_alloc_units: u32::MAX as i64,
                            actual_available_alloc_units: u32::MAX as i64,
                            sectors_per_alloc_unit: u32::MAX,
                            bytes_per_sector: 1,
                        }
                        .into(),
                    ),
                    efs::FileSystemInformationClassLevel::FILE_FS_DEVICE_INFORMATION => Some(
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L570-L571
                        efs::FileFsDeviceInformation {
                            device_type: 0x00000007, // FILE_DEVICE_DISK
                            characteristics: efs::Characteristics::empty(),
                        }
                        .into(),
                    ),
                    efs::FileSystemInformationClassLevel::FILE_FS_SIZE_INFORMATION => Some(
                        // Fill these out with the default fallback values FreeRDP uses
                        // Written here: https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L510-L513
                        // With default fallback values ultimately found here:
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/file.c#L1018-L1021
                        efs::FileFsSizeInformation {
                            total_alloc_units: u32::MAX as i64,
                            available_alloc_units: u32::MAX as i64,
                            sectors_per_alloc_unit: u32::MAX,
                            bytes_per_sector: 1,
                        }
                        .into(),
                    ),
                    _ => None,
                };

                let io_status = match buffer {
                    Some(_) => NtStatus::SUCCESS,
                    None => NtStatus::UNSUCCESSFUL,
                };

                self.send_rdp_query_vol_info_response(rdp_req.device_io_request, io_status, buffer)
            }
        }
    }

    /// Handles an RDP [`efs::DeviceControlRequest`] received from the RDP server.
    fn handle_rdp_device_control_req(
        &self,
        req: efs::DeviceControlRequest<efs::AnyIoCtlCode>,
    ) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L677-L684
        self.send_rdp_device_control_response(req, NtStatus::SUCCESS, None)
    }

    /// Handles an RDP [`efs::DeviceReadRequest`] received from the RDP server.
    fn handle_rdp_device_read_req(&mut self, req: efs::DeviceReadRequest) -> PduResult<()> {
        self.tdp_sd_read(req)
    }

    /// Handles an RDP [`efs::DeviceWriteRequest`] received from the RDP server.
    fn handle_rdp_device_write_req(&mut self, req: efs::DeviceWriteRequest) -> PduResult<()> {
        self.tdp_sd_write(req)
    }

    /// Handles an RDP [`efs::ServerDriveSetInformationRequest`] received from the RDP server.
    fn handle_rdp_set_information_req(
        &mut self,
        rdp_req: efs::ServerDriveSetInformationRequest,
    ) -> PduResult<()> {
        // Determine whether to send back a STATUS_DIRECTORY_NOT_EMPTY
        // or STATUS_SUCCESS in the case of a succesful operation
        // https://github.com/FreeRDP/FreeRDP/blob/dfa231c0a55b005af775b833f92f6bcd30363d77/channels/drive/client/drive_main.c#L430-L431
        let io_status = match self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            Some(file) => {
                if file.fso.is_non_empty_directory() {
                    NtStatus::DIRECTORY_NOT_EMPTY
                } else {
                    NtStatus::SUCCESS
                }
            }
            None => {
                // File not found in cache
                return self.send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL);
            }
        };

        match rdp_req.set_buffer {
            efs::FileInformationClass::Rename(ref rename_info) => {
                self.tdp_sd_rename(rdp_req.clone(), rename_info, io_status)
            }
            efs::FileInformationClass::Disposition(ref info) => {
                match self.cache.get_file_mut(
                    rdp_req.device_io_request.device_id,
                    rdp_req.device_io_request.file_id,
                ) {
                    // File not found in cache
                    None => self.send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL),
                    Some(file) => {
                        if file.fso.is_file() || file.fso.is_empty_directory() {
                            // https://github.com/FreeRDP/FreeRDP/blob/dfa231c0a55b005af775b833f92f6bcd30363d77/channels/drive/client/drive_file.c#L681
                            file.delete_pending = info.delete_pending == 1;
                        }

                        self.send_rdp_set_info_response(&rdp_req, io_status)
                    }
                }
            }
            efs::FileInformationClass::EndOfFile(ref eof) => {
                self.tdp_sd_truncate(rdp_req.clone(), eof, io_status)
            }
            efs::FileInformationClass::Basic(_) | efs::FileInformationClass::Allocation(_) => {
                // Each of these ask us to change something we don't have control over at the browser
                // level, so we just do nothing and send back a success.
                // https://github.com/FreeRDP/FreeRDP/blob/dfa231c0a55b005af775b833f92f6bcd30363d77/channels/drive/client/drive_file.c#L579
                self.send_rdp_set_info_response(&rdp_req, io_status)
            }
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "received unsupported FileInformationClass value for RDP {:?}",
                    rdp_req
                ))
            )),
        }
    }

    /// Handles an RDP [`efs::ServerDriveLockControlRequest`] received from the RDP server.
    fn handle_rdp_lock_req(&self, _req: efs::ServerDriveLockControlRequest) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/dfa231c0a55b005af775b833f92f6bcd30363d77/channels/drive/client/drive_main.c#L601
        self.client_handle
            .write_rdpdr(pdu::RdpdrPdu::EmptyResponse)?;
        Ok(())
    }

    /// Helper function for writing a [`tdp::SharedDirectoryCreateRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryCreateResponse`] that is received in response.
    fn tdp_sd_create(
        &mut self,
        rdp_req: efs::DeviceCreateRequest,
        file_type: tdp::FileType,
    ) -> PduResult<()> {
        self.send_tdp_sd_create_request(tdp::SharedDirectoryCreateRequest::from(
            &rdp_req, file_type,
        ))?;
        self.cache.insert_handler(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.completion_id,
            SharedDirectoryCreateResponseHandler::new(
                rdp_req,
                move |this: &mut FilesystemBackend,
                      tdp_resp: tdp::SharedDirectoryCreateResponse,
                      rdp_req: efs::DeviceCreateRequest|
                      -> PduResult<()> {
                    if tdp_resp.err_code != TdpErrCode::Nil {
                        return this.send_rdp_device_create_response(
                            &rdp_req,
                            NtStatus::UNSUCCESSFUL,
                            0,
                        );
                    }
                    let file_id = this.cache.add_file(
                        rdp_req.device_io_request.device_id,
                        FileCacheObject::new(UnixPath::from(&rdp_req.path), tdp_resp.fso),
                    )?;
                    this.send_rdp_device_create_response(&rdp_req, NtStatus::SUCCESS, file_id)
                },
            ),
        )?;
        Ok(())
    }

    /// Helper function for combining a [`tdp::SharedDirectoryDeleteRequest`]
    /// with a [`tdp::SharedDirectoryCreateRequest`] to overwrite a file.
    fn tdp_sd_overwrite(&mut self, rdp_req: efs::DeviceCreateRequest) -> PduResult<()> {
        let tdp_req = tdp::SharedDirectoryDeleteRequest::from(&rdp_req);
        self.send_tdp_sd_delete_request(tdp_req)?;
        self.cache.insert_handler(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.completion_id,
            SharedDirectoryDeleteResponseHandler::new(
                rdp_req,
                move |this: &mut FilesystemBackend,
                      tdp_resp: tdp::SharedDirectoryDeleteResponse,
                      rdp_req: efs::DeviceCreateRequest|
                      -> PduResult<()> {
                    match tdp_resp.err_code {
                        TdpErrCode::Nil => {
                            this.tdp_sd_create(rdp_req, tdp::FileType::File)?;
                            Ok(())
                        }
                        _ => this.send_rdp_device_create_response(
                            &rdp_req,
                            NtStatus::UNSUCCESSFUL,
                            0,
                        ),
                    }
                },
            ),
        )?;
        Ok(())
    }

    /// Helper function for sending a [`tdp::SharedDirectoryDeleteRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryDeleteResponse`] that is received in response.
    fn tdp_sd_delete(
        &mut self,
        rdp_req: efs::DeviceCloseRequest,
        file: FileCacheObject,
    ) -> PduResult<()> {
        let tdp_req = tdp::SharedDirectoryDeleteRequest::from_fco(&rdp_req, file);
        self.send_tdp_sd_delete_request(tdp_req)?;
        self.cache.insert_handler(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.completion_id,
            SharedDirectoryDeleteResponseHandler::new(
                rdp_req,
                move |this: &mut FilesystemBackend,
                      tdp_resp: tdp::SharedDirectoryDeleteResponse,
                      rdp_req: efs::DeviceCloseRequest|
                      -> PduResult<()> {
                    let io_status = if tdp_resp.err_code == TdpErrCode::Nil {
                        NtStatus::SUCCESS
                    } else {
                        NtStatus::UNSUCCESSFUL
                    };
                    this.send_rdp_device_close_response(rdp_req, io_status)
                },
            ),
        )?;
        Ok(())
    }

    /// Helper function for sending a [`tdp::SharedDirectoryReadRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryReadResponse`] that is received in response.
    fn tdp_sd_read(&mut self, rdp_req: efs::DeviceReadRequest) -> PduResult<()> {
        match self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            // File not found in cache
            None => self.send_rdp_read_response(
                rdp_req.device_io_request,
                NtStatus::UNSUCCESSFUL,
                vec![],
            ),
            Some(file) => {
                let tdp_req = tdp::SharedDirectoryReadRequest::from_fco(&rdp_req, file);
                self.send_tdp_sd_read_request(tdp_req)?;
                self.cache.insert_handler(
                    rdp_req.device_io_request.device_id,
                    rdp_req.device_io_request.completion_id,
                    SharedDirectoryReadResponseHandler::new(
                        rdp_req,
                        move |this: &mut FilesystemBackend,
                              tdp_res: tdp::SharedDirectoryReadResponse,
                              rdp_req: efs::DeviceReadRequest|
                              -> PduResult<()> {
                            this.tdp_sd_read_continued(rdp_req, tdp_res)
                        },
                    ),
                )?;

                Ok(())
            }
        }
    }

    fn tdp_sd_read_continued(
        &mut self,
        rdp_req: efs::DeviceReadRequest,
        tdp_res: tdp::SharedDirectoryReadResponse,
    ) -> PduResult<()> {
        match tdp_res.err_code {
            TdpErrCode::Nil => self.send_rdp_read_response(
                rdp_req.device_io_request,
                NtStatus::SUCCESS,
                tdp_res.read_data,
            ),
            _ => self.send_rdp_read_response(
                rdp_req.device_io_request,
                NtStatus::UNSUCCESSFUL,
                vec![],
            ),
        }
    }

    /// Helper function for sending a [`tdp::SharedDirectoryWriteRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryWriteResponse`] that is received in response.
    fn tdp_sd_write(&mut self, rdp_req: efs::DeviceWriteRequest) -> PduResult<()> {
        match self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            // File not found in cache
            None => {
                self.send_rdp_write_response(rdp_req.device_io_request, NtStatus::UNSUCCESSFUL, 0)
            }
            Some(file) => {
                self.send_tdp_sd_write_request(tdp::SharedDirectoryWriteRequest::from_fco(
                    &rdp_req, file,
                ))?;
                self.cache.insert_handler(
                    rdp_req.device_io_request.device_id,
                    rdp_req.device_io_request.completion_id,
                    SharedDirectoryWriteResponseHandler::new(
                        rdp_req,
                        move |this: &mut FilesystemBackend,
                              tdp_res: tdp::SharedDirectoryWriteResponse,
                              rdp_req: efs::DeviceWriteRequest|
                              -> PduResult<()> {
                            this.tdp_sd_write_continued(rdp_req, tdp_res)
                        },
                    ),
                )?;

                Ok(())
            }
        }
    }

    fn tdp_sd_write_continued(
        &mut self,
        rdp_req: efs::DeviceWriteRequest,
        tdp_res: tdp::SharedDirectoryWriteResponse,
    ) -> PduResult<()> {
        match tdp_res.err_code {
            TdpErrCode::Nil => self.send_rdp_write_response(
                rdp_req.device_io_request,
                NtStatus::SUCCESS,
                tdp_res.bytes_written,
            ),
            _ => self.send_rdp_write_response(rdp_req.device_io_request, NtStatus::UNSUCCESSFUL, 0),
        }
    }

    /// Helper function for renaming a file. Handles the logic for whether to "replace if exists" or not
    /// based on the passed `rename_info.replace_if_exists` value.
    fn tdp_sd_rename(
        &mut self,
        rdp_req: efs::ServerDriveSetInformationRequest,
        rename_info: &efs::FileRenameInformation,
        io_status: NtStatus,
    ) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/dfa231c0a55b005af775b833f92f6bcd30363d77/channels/drive/client/drive_file.c#L709
        match rename_info.replace_if_exists {
            // If replace_if_exists is true, we can just send a TDP SharedDirectoryMoveRequest,
            // which works like the unix `mv` utility (meaning it will automatically replace if exists).
            efs::Boolean::True => self.tdp_sd_move(rdp_req, rename_info.clone(), io_status),
            efs::Boolean::False => {
                // If replace_if_exists is false, first check if the new_path exists.
                self.send_tdp_sd_info_request(tdp::SharedDirectoryInfoRequest {
                    completion_id: rdp_req.device_io_request.completion_id,
                    directory_id: rdp_req.device_io_request.device_id,
                    path: UnixPath::from(&rename_info.file_name),
                })?;

                let rename_info = (*rename_info).clone();

                self.cache.insert_handler(
                    rdp_req.device_io_request.device_id,
                    rdp_req.device_io_request.completion_id,
                    ResponseKind::Info(SharedDirectoryInfoResponseHandler::new(
                        rdp_req,
                        move |this: &mut FilesystemBackend,
                              res: tdp::SharedDirectoryInfoResponse,
                              rdp_req: efs::ServerDriveSetInformationRequest|
                              -> PduResult<()> {
                            if res.err_code == TdpErrCode::DoesNotExist {
                                // If the file doesn't already exist, send a move request.
                                return this.tdp_sd_move(rdp_req, rename_info.clone(), io_status);
                            }
                            // If it does, send back a name collision error, as is done in FreeRDP.
                            this.send_rdp_set_info_response(
                                &rdp_req,
                                NtStatus::OBJECT_NAME_COLLISION,
                            )
                        },
                    )),
                )?;

                Ok(())
            }
        }
    }

    /// Helper function for sending a [`tdp::SharedDirectoryMoveRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryMoveResponse`] that is received in response.
    fn tdp_sd_truncate(
        &mut self,
        rdp_req: efs::ServerDriveSetInformationRequest,
        eof: &efs::FileEndOfFileInformation,
        io_status: NtStatus,
    ) -> PduResult<()> {
        if let Some(file) = self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            let end_of_file = eof.end_of_file;
            self.send_tdp_truncate_request(tdp::SharedDirectoryTruncateRequest {
                completion_id: rdp_req.device_io_request.completion_id,
                directory_id: rdp_req.device_io_request.device_id,
                path: file.path.clone(),
                end_of_file: cast_length("tdp_sd_truncate", "end_of_file", eof.end_of_file)?,
            })?;

            self.cache.insert_handler(
                rdp_req.device_io_request.device_id,
                rdp_req.device_io_request.completion_id,
                SharedDirectoryTruncateResponseHandler::new(
                    rdp_req,
                    move |this: &mut FilesystemBackend,
                          res: tdp::SharedDirectoryTruncateResponse,
                          rdp_req: efs::ServerDriveSetInformationRequest|
                          -> PduResult<()> {
                        if res.err_code != TdpErrCode::Nil {
                            return this
                                .send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL);
                        }

                        let io_status = if let Some(file) = this.cache.get_file_mut(
                            rdp_req.device_io_request.device_id,
                            rdp_req.device_io_request.file_id,
                        ) {
                            // Truncate succeeded, update our internal books to reflect the new size.
                            file.fso.size =
                                cast_length("tdp_sd_truncate", "end_of_file", end_of_file)?;
                            io_status
                        } else {
                            // This shouldn't happen.
                            warn!("file unexpectedly not found in cache after truncate");
                            NtStatus::UNSUCCESSFUL
                        };

                        this.send_rdp_set_info_response(&rdp_req, io_status)
                    },
                ),
            )?;

            return Ok(());
        }

        // This shouldn't happen.
        warn!("attempted to truncate a file that wasn't in the file cache");
        self.send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL)
    }

    /// Helper function for sending a [`tdp::SharedDirectoryMoveRequest`] to the browser
    /// and handling the [`tdp::SharedDirectoryMoveResponse`] that is received in response.
    fn tdp_sd_move(
        &mut self,
        rdp_req: efs::ServerDriveSetInformationRequest,
        rename_info: efs::FileRenameInformation,
        io_status: NtStatus,
    ) -> PduResult<()> {
        if let Some(file) = self.cache.get_file(
            rdp_req.device_io_request.device_id,
            rdp_req.device_io_request.file_id,
        ) {
            self.send_tdp_sd_move_request(tdp::SharedDirectoryMoveRequest {
                completion_id: rdp_req.device_io_request.completion_id,
                directory_id: rdp_req.device_io_request.device_id,
                original_path: file.path.clone(),
                new_path: UnixPath::from(&rename_info.file_name),
            })?;

            self.cache.insert_handler(
                rdp_req.device_io_request.device_id,
                rdp_req.device_io_request.completion_id,
                SharedDirectoryMoveResponseHandler::new(
                    rdp_req,
                    move |this: &mut FilesystemBackend,
                          res: tdp::SharedDirectoryMoveResponse,
                          rdp_req: efs::ServerDriveSetInformationRequest|
                          -> PduResult<()> {
                        if res.err_code != TdpErrCode::Nil {
                            return this
                                .send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL);
                        }

                        this.send_rdp_set_info_response(&rdp_req, io_status)
                    },
                ),
            )?;

            return Ok(());
        }

        // File not found in cache
        self.send_rdp_set_info_response(&rdp_req, NtStatus::UNSUCCESSFUL)
    }

    /// Sends a [`tdp::SharedDirectoryAcknowledge`] to the browser.
    fn send_tdp_sd_acknowledge(
        &self,
        mut tdp_req: tdp::SharedDirectoryAcknowledge,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let err = unsafe { cgo_tdp_sd_acknowledge(self.cgo_handle, &mut tdp_req) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_acknowledge failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryInfoRequest`] to the browser.
    fn send_tdp_sd_info_request(&self, tdp_req: tdp::SharedDirectoryInfoRequest) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_info_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_info_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryTruncateRequest`] to the browser.
    fn send_tdp_truncate_request(
        &self,
        tdp_req: tdp::SharedDirectoryTruncateRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_truncate_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_truncate_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryCreateRequest`] to the browser.
    fn send_tdp_sd_create_request(
        &self,
        tdp_req: tdp::SharedDirectoryCreateRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_create_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_create_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryDeleteRequest`] to the browser.
    fn send_tdp_sd_delete_request(
        &self,
        tdp_req: tdp::SharedDirectoryDeleteRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_delete_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_delete_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryListRequest`] to the browser.
    fn send_tdp_sd_list_request(&self, tdp_req: tdp::SharedDirectoryListRequest) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_list_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_list_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryReadRequest`] to the browser.
    fn send_tdp_sd_read_request(&self, tdp_req: tdp::SharedDirectoryReadRequest) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_read_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_read_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryWriteRequest`] to the browser.
    fn send_tdp_sd_write_request(
        &self,
        tdp_req: tdp::SharedDirectoryWriteRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_write_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_write_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Sends a [`tdp::SharedDirectoryMoveRequest`] to the browser.
    fn send_tdp_sd_move_request(&self, tdp_req: tdp::SharedDirectoryMoveRequest) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let mut req = tdp_req.into_cgo()?;
        let err = unsafe { cgo_tdp_sd_move_request(self.cgo_handle, req.cgo()) };
        match err {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "call to tdp_sd_move_request failed: {:?}",
                    err
                ))
            )),
        }
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryInfoResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryInfoResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryInfoResponse`].
    pub fn handle_tdp_sd_info_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryInfoResponseHandler>(
                tdp_resp.device_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryCreateResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryCreateResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryCreateResponse`].
    pub fn handle_tdp_sd_create_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryCreateResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryCreateResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryDeleteResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryDeleteResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryDeleteResponse`].
    pub fn handle_tdp_sd_delete_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryDeleteResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryDeleteResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryListResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryListResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryListResponse`].
    pub fn handle_tdp_sd_list_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryListResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryListResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryReadResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryReadResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryReadResponse`].
    pub fn handle_tdp_sd_read_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryReadResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryReadResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryWriteResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryWriteResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryWriteResponse`].
    pub fn handle_tdp_sd_write_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryWriteResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryWriteResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    pub fn handle_tdp_sd_move_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryMoveResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryMoveResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    pub fn handle_tdp_sd_truncate_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryTruncateResponse,
    ) -> PduResult<()> {
        self.cache
            .remove_handler::<SharedDirectoryTruncateResponseHandler>(
                tdp_resp.directory_id,
                tdp_resp.completion_id,
            )?
            .call(self, tdp_resp)
    }

    /// Helper function for sending an RDP [`efs::DeviceCreateResponse`] based on an RDP [`efs::DeviceCreateRequest`].
    fn send_rdp_device_create_response(
        &self,
        device_create_request: &efs::DeviceCreateRequest,
        io_status: efs::NtStatus,
        new_file_id: u32,
    ) -> PduResult<()> {
        // See https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L187-L228
        let information = if io_status != efs::NtStatus::SUCCESS
            || matches!(
                device_create_request.create_disposition,
                efs::CreateDisposition::FILE_SUPERSEDE
                    | efs::CreateDisposition::FILE_OPEN
                    | efs::CreateDisposition::FILE_CREATE
                    | efs::CreateDisposition::FILE_OVERWRITE
            ) {
            Ok(efs::Information::FILE_SUPERSEDED)
        } else if device_create_request.create_disposition == efs::CreateDisposition::FILE_OPEN_IF {
            Ok(efs::Information::FILE_OPENED)
        } else if device_create_request.create_disposition
            == efs::CreateDisposition::FILE_OVERWRITE_IF
        {
            Ok(efs::Information::FILE_OVERWRITTEN)
        } else {
            Err(pdu_other_err!(
                "program error, CreateDispositionFlags check should be exhaustive"
            ))
        }?;

        self.client_handle.write_rdpdr(
            efs::DeviceCreateResponse {
                device_io_reply: efs::DeviceIoResponse::new(
                    device_create_request.device_io_request.clone(),
                    io_status,
                ),
                file_id: new_file_id,
                information,
            }
            .into(),
        )?;
        Ok(())
    }

    /// Helper function for sending an RDP [`efs::ClientDriveQueryInformationResponse`]
    /// to the RDP server.
    fn send_rdp_client_drive_query_info_response(
        &self,
        rdp_req: efs::ServerDriveQueryInformationRequest,
        file: Option<&FileCacheObject>,
    ) -> PduResult<()> {
        let file = match file {
            Some(file) => file,
            None => {
                // Early return with NtStatus::UNSUCCESSFUL if the file is not found
                self.client_handle.write_rdpdr(
                    efs::ClientDriveQueryInformationResponse {
                        device_io_response: efs::DeviceIoResponse::new(
                            rdp_req.device_io_request.clone(),
                            NtStatus::UNSUCCESSFUL,
                        ),
                        buffer: None,
                    }
                    .into(),
                )?;
                return Ok(());
            }
        };

        let device_io_response =
            efs::DeviceIoResponse::new(rdp_req.device_io_request.clone(), NtStatus::SUCCESS);

        // We support all the FsInformationClasses that FreeRDP does here
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L482
        match rdp_req.file_info_class_lvl {
            efs::FileInformationClassLevel::FILE_BASIC_INFORMATION => {
                self.send_rdp_file_basic_info(device_io_response, file)
            }
            efs::FileInformationClassLevel::FILE_STANDARD_INFORMATION => {
                self.send_rdp_file_standard_info(device_io_response, file)
            }
            efs::FileInformationClassLevel::FILE_ATTRIBUTE_TAG_INFORMATION => {
                self.send_rdp_file_attr_tag_info(device_io_response, file)
            }
            _ => Err(pdu_other_err!(
                "",
                source:FilesystemBackendError(format!(
                    "received unsupported FileInformationClass: {:?}",
                    rdp_req.file_info_class_lvl
                ))
            )),
        }
    }

    fn send_rdp_file_basic_info(
        &self,
        device_io_response: efs::DeviceIoResponse,
        file: &FileCacheObject,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::ClientDriveQueryInformationResponse {
                device_io_response,
                buffer: Some(efs::FileInformationClass::Basic(
                    efs::FileBasicInformation {
                        creation_time: tdp::to_windows_time(file.fso.last_modified),
                        last_access_time: tdp::to_windows_time(file.fso.last_modified),
                        last_write_time: tdp::to_windows_time(file.fso.last_modified),
                        change_time: tdp::to_windows_time(file.fso.last_modified),
                        file_attributes: if file.fso.file_type == tdp::FileType::File {
                            efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
                        } else {
                            efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
                        },
                    },
                )),
            }
            .into(),
        )?;
        Ok(())
    }

    fn send_rdp_file_standard_info(
        &self,
        device_io_response: efs::DeviceIoResponse,
        file: &FileCacheObject,
    ) -> PduResult<()> {
        let file_fso_size: i64 = cast_length(
            "FilesystemBackend::send_file_standard_info",
            "file.fso.size",
            file.fso.size,
        )?;

        self.client_handle.write_rdpdr(
            efs::ClientDriveQueryInformationResponse {
                device_io_response,
                buffer: Some(efs::FileInformationClass::Standard(
                    efs::FileStandardInformation {
                        allocation_size: file_fso_size,
                        end_of_file: file_fso_size,
                        number_of_links: 0,
                        delete_pending: if file.delete_pending {
                            efs::Boolean::True
                        } else {
                            efs::Boolean::False
                        },
                        directory: if file.fso.file_type == tdp::FileType::File {
                            efs::Boolean::False
                        } else {
                            efs::Boolean::True
                        },
                    },
                )),
            }
            .into(),
        )?;
        Ok(())
    }

    fn send_rdp_file_attr_tag_info(
        &self,
        device_io_response: efs::DeviceIoResponse,
        file: &FileCacheObject,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::ClientDriveQueryInformationResponse {
                device_io_response,
                buffer: Some(efs::FileInformationClass::AttributeTag(
                    efs::FileAttributeTagInformation {
                        file_attributes: if file.fso.file_type == tdp::FileType::File {
                            efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
                        } else {
                            efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
                        },
                        reparse_tag: 0,
                    },
                )),
            }
            .into(),
        )?;
        Ok(())
    }

    /// Sends an RDP [`efs::DeviceCloseResponse`] to the RDP server.
    fn send_rdp_device_close_response(
        &self,
        rdp_req: efs::DeviceCloseRequest,
        io_status: NtStatus,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::DeviceCloseResponse {
                device_io_response: efs::DeviceIoResponse::new(
                    rdp_req.device_io_request.clone(),
                    io_status,
                ),
            }
            .into(),
        )?;
        Ok(())
    }

    /// Sends the next RDP [`efs::ClientDriveQueryDirectoryResponse`] in the series of expected
    /// responses to the RDP server.
    fn send_rdp_next_drive_query_dir_response(
        &mut self,
        req: &efs::ServerDriveQueryDirectoryRequest,
    ) -> PduResult<()> {
        // req gives us a FileId, which we use to get the FileCacheObject for the directory that
        // this request is targeted at. We use that FileCacheObject as an iterator, grabbing the
        // next() FileSystemObject (starting with ".", then "..", then iterating through the contents
        // of the target directory), which we then convert to an RDP FileInformationClass for sending back
        // to the RDP server.
        if let Some(dir) = self.cache.get_file_mut(
            req.device_io_request.device_id,
            req.device_io_request.file_id,
        ) {
            if let Some(fso) = dir.next() {
                let buffer = match req.file_info_class_lvl {
                    efs::FileInformationClassLevel::FILE_BOTH_DIRECTORY_INFORMATION => Some(
                        efs::FileInformationClass::BothDirectory(fso.into_both_directory()?),
                    ),
                    efs::FileInformationClassLevel::FILE_FULL_DIRECTORY_INFORMATION => Some(
                        efs::FileInformationClass::FullDirectory(fso.into_full_directory()?),
                    ),
                    efs::FileInformationClassLevel::FILE_NAMES_INFORMATION => {
                        Some(efs::FileInformationClass::Names(fso.into_names()?))
                    }
                    efs::FileInformationClassLevel::FILE_DIRECTORY_INFORMATION => {
                        Some(efs::FileInformationClass::Directory(fso.into_directory()?))
                    }
                    _ => {
                        return Err(pdu_other_err!(
                            "",
                            source:FilesystemBackendError(format!(
                                "received unsupported file information class level: {:?}",
                                req.file_info_class_lvl,
                            ))
                        ));
                    }
                };

                return self.send_rdp_drive_query_dir_response(
                    req.device_io_request.clone(),
                    NtStatus::SUCCESS,
                    buffer,
                );
            }

            // If we reach here it means our iterator is exhausted,
            // so we send back a NtStatus::NO_MORE_FILES to
            // alert RDP that we've listed all the contents of this directory.
            // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/winpr/libwinpr/file/generic.c#L1193
            // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L114
            return self.send_rdp_drive_query_dir_response(
                req.device_io_request.clone(),
                NtStatus::NO_MORE_FILES,
                None,
            );
        }

        // File not found in cache
        self.send_rdp_drive_query_dir_response(
            req.device_io_request.clone(),
            NtStatus::UNSUCCESSFUL,
            None,
        )
    }

    /// Sends an RDP [`efs::ClientDriveQueryDirectoryResponse`] to the RDP server.
    fn send_rdp_drive_query_dir_response(
        &self,
        device_io_request: efs::DeviceIoRequest,
        io_status: NtStatus,
        buffer: Option<efs::FileInformationClass>,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::ClientDriveQueryDirectoryResponse {
                device_io_reply: efs::DeviceIoResponse::new(device_io_request, io_status),
                buffer,
            }
            .into(),
        )?;
        Ok(())
    }

    /// Sends an RDP [`efs::ClientDriveQueryVolumeInformationResponse`] to the RDP server.
    fn send_rdp_query_vol_info_response(
        &self,
        device_io_request: efs::DeviceIoRequest,
        io_status: NtStatus,
        buffer: Option<efs::FileSystemInformationClass>,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::ClientDriveQueryVolumeInformationResponse::new(
                device_io_request,
                io_status,
                buffer,
            )
            .into(),
        )?;
        Ok(())
    }

    /// Sends an RDP [`efs::DeviceControlResponse`] to the RDP server.
    fn send_rdp_device_control_response<T: efs::IoCtlCode>(
        &self,
        req: efs::DeviceControlRequest<T>,
        io_status: NtStatus,
        output_buffer: Option<Box<dyn esc::rpce::Encode>>,
    ) -> PduResult<()> {
        self.client_handle
            .write_rdpdr(efs::DeviceControlResponse::new(req, io_status, output_buffer).into())?;
        Ok(())
    }

    /// Sends an RDP [`efs::DeviceReadResponse`] to the RDP server.
    fn send_rdp_read_response(
        &self,
        device_io_request: efs::DeviceIoRequest,
        io_status: NtStatus,
        read_data: Vec<u8>,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::DeviceReadResponse {
                device_io_reply: efs::DeviceIoResponse::new(device_io_request, io_status),
                read_data,
            }
            .into(),
        )?;
        Ok(())
    }

    /// Sends an RDP [`efs::DeviceWriteResponse`] to the RDP server.
    fn send_rdp_write_response(
        &self,
        device_io_request: efs::DeviceIoRequest,
        io_status: NtStatus,
        bytes_written: u32,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::DeviceWriteResponse {
                device_io_reply: efs::DeviceIoResponse::new(device_io_request, io_status),
                length: bytes_written,
            }
            .into(),
        )?;
        Ok(())
    }

    /// Sends an RDP [`efs::ServerDriveSetInformationResponse`] to the RDP server.
    fn send_rdp_set_info_response(
        &self,
        req: &efs::ServerDriveSetInformationRequest,
        io_status: NtStatus,
    ) -> PduResult<()> {
        self.client_handle.write_rdpdr(
            efs::ClientDriveSetInformationResponse::new(req, io_status)
                .map_err(|e| PduError::encode("send_rdp_set_info_response", e))?
                .into(),
        )?;
        Ok(())
    }
}

#[derive(Debug)]
struct FileCache {
    cache: HashMap<u32, FileCacheObject>,
    next_file_id: u32,
}

impl FileCache {
    fn new() -> Self {
        Self {
            cache: HashMap::new(),
            next_file_id: 0,
        }
    }

    /// Insert a [`FileCacheObject`] into the file cache.
    ///
    /// Returns the `file_id` of the inserted [`FileCacheObject`],
    /// or an error if the `file_id` already exists in the cache.
    fn insert(&mut self, file: FileCacheObject) -> Result<u32, FilesystemBackendError> {
        self.next_file_id = self.next_file_id.wrapping_add(1);
        if self.cache.insert(self.next_file_id, file).is_none() {
            Ok(self.next_file_id)
        } else {
            Err(FilesystemBackendError("attempted to insert a FileCacheObject into the file cache with a file_id that already exists in the cache".to_string()))
        }
    }

    /// Retrieves a FileCacheObject from the file cache,
    /// without removing it from the cache.
    fn get(&self, file_id: u32) -> Option<&FileCacheObject> {
        self.cache.get(&file_id)
    }
    /// Retrieves a mutable FileCacheObject from the file cache,
    /// without removing it from the cache.
    fn get_mut(&mut self, file_id: u32) -> Option<&mut FileCacheObject> {
        self.cache.get_mut(&file_id)
    }

    /// Retrieves a FileCacheObject from the file cache,
    /// removing it from the cache.
    fn remove(&mut self, file_id: u32) -> Option<FileCacheObject> {
        self.cache.remove(&file_id)
    }
}

/// FileCacheObject is an in-memory representation of
/// of a file or directory holding the metadata necessary
/// for RDP drive redirection. They are stored in map indexed
/// by their RDP FileId.
///
/// The lifecycle for a FileCacheObject is a function of the
/// MajorFunction of RDP DeviceIoRequests:
///
/// | Sequence | MajorFunction | results in                                               |
/// | -------- | ------------- | ---------------------------------------------------------|
/// | 1        | IRP_MJ_CREATE | A new FileCacheObject is created and assigned a FileId   |
/// | -------- | ------------- | ---------------------------------------------------------|
/// | 2        | <other>       | The FCO is retrieved from the cache by the FileId in the |
/// |          |               | DeviceIoRequest and metadata is used to craft a response |
/// | -------- | ------------- | ---------------------------------------------------------|
/// | 3        | IRP_MJ_CLOSE  | The FCO is deleted from the cache                        |
/// | -------- | ------------- | ---------------------------------------------------------|
#[derive(Debug, Clone)]
pub struct FileCacheObject {
    path: UnixPath,
    delete_pending: bool,
    /// The tdp::FileSystemObject pertaining to the file or directory at path.
    fso: tdp::FileSystemObject,
    /// A vector of the contents of the directory at path.
    contents: Vec<tdp::FileSystemObject>,

    /// Book-keeping variable, see Iterator implementation
    contents_i: usize,
    /// Book-keeping variable, see Iterator implementation
    dot_sent: bool,
    /// Book-keeping variable, see Iterator implementation
    dotdot_sent: bool,
}

impl FileCacheObject {
    fn new(path: UnixPath, fso: tdp::FileSystemObject) -> Self {
        Self {
            path,
            delete_pending: false,
            fso,
            contents: Vec::new(),

            contents_i: 0,
            dot_sent: false,
            dotdot_sent: false,
        }
    }

    pub fn path(&self) -> UnixPath {
        self.path.clone()
    }
}

/// FileCacheObject is used as an iterator for the implementation of
/// IRP_MJ_DIRECTORY_CONTROL, which requires that we iterate through
/// all the files of a directory one by one. In this case, the directory
/// is the FileCacheObject itself, with its own fso field representing
/// the directory, and its contents being represented by tdp::FileSystemObject's
/// in its contents field.
///
/// We account for an idiosyncrasy of the RDP protocol here: when fielding an
/// IRP_MJ_DIRECTORY_CONTROL, RDP first expects to receive an entry for the "."
/// directory, and next an entry for the ".." directory. Only after those two
/// directories have been sent do we begin sending the actual contents of this
/// directory (the contents field). (This is why we maintain dot_sent and dotdot_sent
/// fields on each FileCacheObject)
///
/// Note that this implementation only makes sense in the case that this FileCacheObject
/// is itself a directory (fso.file_type == FileType::Directory). We leave it up to the
/// caller to ensure iteration makes sense in the given context that it's used.
impl Iterator for FileCacheObject {
    type Item = tdp::FileSystemObject;

    fn next(&mut self) -> Option<Self::Item> {
        // On the first call to next, return the "." directory
        if !self.dot_sent {
            self.dot_sent = true;
            Some(tdp::FileSystemObject {
                last_modified: self.fso.last_modified,
                size: self.fso.size,
                file_type: self.fso.file_type,
                is_empty: tdp::FALSE,
                path: UnixPath::from(".".to_string()),
            })
        } else if !self.dotdot_sent {
            // On the second call to next, return the ".." directory
            self.dotdot_sent = true;
            Some(tdp::FileSystemObject {
                last_modified: self.fso.last_modified,
                size: 0,
                file_type: tdp::FileType::Directory,
                is_empty: tdp::FALSE,
                path: UnixPath::from("..".to_string()),
            })
        } else {
            // "." and ".." have been sent, now start iterating through
            // the actual contents of the directory
            if self.contents_i < self.contents.len() {
                let i = self.contents_i;
                self.contents_i += 1;
                return Some(self.contents[i].clone());
            }
            None
        }
    }
}

/// A generic error type for the FilesystemBackend that can contain any arbitrary error message.
#[derive(Debug)]
#[allow(dead_code)] // The internal `String` is "dead code" according to the compiler, but we want it for debugging purposes.
struct FilesystemBackendError(pub String);

impl From<FilesystemBackendError> for PduError {
    fn from(value: FilesystemBackendError) -> Self {
        PduError::new(
            "filesystem",
            ironrdp_pdu::PduErrorKind::Other { description: "" },
        )
        .with_source(value)
    }
}

impl std::fmt::Display for FilesystemBackendError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl std::error::Error for FilesystemBackendError {}

// Cancel allows us implement generic cancellation methods for various efs::<request> types
trait Cancel {
    fn cancel(&self) -> RdpdrPdu;
}

impl Cancel for efs::DeviceCreateRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::DeviceCreateResponse {
            device_io_reply: DeviceIoResponse {
                device_id: self.device_io_request.device_id,
                completion_id: self.device_io_request.completion_id,
                io_status: NtStatus::UNSUCCESSFUL,
            },
            file_id: 0,
            information: efs::Information::empty(),
        }
        .into()
    }
}

impl Cancel for efs::ServerDriveQueryDirectoryRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::ClientDriveQueryDirectoryResponse {
            device_io_reply: efs::DeviceIoResponse::new(
                self.device_io_request.clone(),
                NtStatus::UNSUCCESSFUL,
            ),
            buffer: None,
        }
        .into()
    }
}

impl Cancel for efs::DeviceReadRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::DeviceReadResponse {
            device_io_reply: efs::DeviceIoResponse::new(
                self.device_io_request.clone(),
                NtStatus::UNSUCCESSFUL,
            ),
            read_data: vec![],
        }
        .into()
    }
}

impl Cancel for efs::DeviceWriteRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::DeviceWriteResponse {
            device_io_reply: efs::DeviceIoResponse::new(
                self.device_io_request.clone(),
                NtStatus::UNSUCCESSFUL,
            ),
            length: 0,
        }
        .into()
    }
}

impl Cancel for efs::ServerDriveSetInformationRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::ClientDriveSetInformationResponse::new(self, NtStatus::UNSUCCESSFUL)
            .map(|resp| resp.into())
            .unwrap_or(RdpdrPdu::EmptyResponse)
    }
}

impl Cancel for efs::DeviceCloseRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::DeviceCloseResponse {
            device_io_response: efs::DeviceIoResponse::new(
                self.device_io_request.clone(),
                NtStatus::UNSUCCESSFUL,
            ),
        }
        .into()
    }
}

impl Cancel for efs::ServerDriveNotifyChangeDirectoryRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::ClientDriveQueryDirectoryResponse {
            device_io_reply: efs::DeviceIoResponse {
                device_id: self.device_io_request.device_id,
                completion_id: self.device_io_request.completion_id,
                // https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-erref/596a1078-e883-4972-9bbc-49e60bebca55
                // STATUS_CANCELLED - 0xC0000120
                io_status: NtStatus::from(0xC0000120),
            },
            buffer: None,
        }
        .into()
    }
}

impl Cancel for efs::ServerDriveQueryInformationRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::ClientDriveQueryInformationResponse {
            device_io_response: efs::DeviceIoResponse::new(
                self.device_io_request.clone(),
                // Testing shows that we MUST set this to "cancel" rather than UNSUCCESSFUL
                // otherwise the rdpdr channel can end up in a broken state upon device removal.
                NtStatus::from(0xC0000120),
            ),
            buffer: None,
        }
        .into()
    }
}
impl Cancel for efs::ServerDriveQueryVolumeInformationRequest {
    fn cancel(&self) -> RdpdrPdu {
        efs::ClientDriveQueryVolumeInformationResponse::new(
            self.device_io_request.clone(),
            NtStatus::UNSUCCESSFUL,
            None,
        )
        .into()
    }
}
impl Cancel for efs::DeviceControlRequest<efs::AnyIoCtlCode> {
    fn cancel(&self) -> RdpdrPdu {
        efs::DeviceControlResponse::new(self.clone(), NtStatus::UNSUCCESSFUL, None).into()
    }
}
impl Cancel for efs::ServerDriveLockControlRequest {
    fn cancel(&self) -> RdpdrPdu {
        RdpdrPdu::EmptyResponse
    }
}

trait HandlerFn<T>: FnOnce(&mut FilesystemBackend, T) -> PduResult<()> + Send {}

impl<T, F> HandlerFn<T> for F where F: FnOnce(&mut FilesystemBackend, T) -> PduResult<()> + Send {}

struct ResponseHandler<T> {
    cancellable: Box<dyn Cancel + Send>,
    handler: Box<dyn HandlerFn<T>>,
}

// Write a function whose return type depends on the input

// ResponseHandler is allowed to either invoke 'call' XOR 'cancel' exactly once.
impl<T> ResponseHandler<T> {
    fn new<R: Cancel + Clone + Send + 'static>(
        req: R,
        handler: impl FnOnce(&mut FilesystemBackend, T, R) -> PduResult<()> + Send + 'static,
    ) -> Self {
        Self {
            cancellable: Box::new(req.clone()),
            handler: Box::new(move |this, input| handler(this, input, req)),
        }
    }

    fn call(self, this: &mut FilesystemBackend, res: T) -> PduResult<()> {
        (self.handler)(this, res)
    }

    fn cancel(self) -> RdpdrPdu {
        self.cancellable.cancel()
    }
}

impl<T> std::fmt::Debug for ResponseHandler<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "<{}>", std::any::type_name::<T>())
    }
}

type SharedDirectoryInfoResponseHandler = ResponseHandler<tdp::SharedDirectoryInfoResponse>;
type SharedDirectoryCreateResponseHandler = ResponseHandler<tdp::SharedDirectoryCreateResponse>;
type SharedDirectoryDeleteResponseHandler = ResponseHandler<tdp::SharedDirectoryDeleteResponse>;
type SharedDirectoryListResponseHandler = ResponseHandler<tdp::SharedDirectoryListResponse>;
type SharedDirectoryReadResponseHandler = ResponseHandler<tdp::SharedDirectoryReadResponse>;
type SharedDirectoryWriteResponseHandler = ResponseHandler<tdp::SharedDirectoryWriteResponse>;
type SharedDirectoryMoveResponseHandler = ResponseHandler<tdp::SharedDirectoryMoveResponse>;
type SharedDirectoryTruncateResponseHandler = ResponseHandler<tdp::SharedDirectoryTruncateResponse>;

#[derive(Debug)]
enum ResponseKind {
    Info(SharedDirectoryInfoResponseHandler),
    Create(SharedDirectoryCreateResponseHandler),
    Delete(SharedDirectoryDeleteResponseHandler),
    List(SharedDirectoryListResponseHandler),
    Read(SharedDirectoryReadResponseHandler),
    Write(SharedDirectoryWriteResponseHandler),
    Move(SharedDirectoryMoveResponseHandler),
    Truncate(SharedDirectoryTruncateResponseHandler),
}

impl ResponseKind {
    fn cancel(self) -> RdpdrPdu {
        match self {
            ResponseKind::Info(h) => h.cancel(),
            ResponseKind::Create(h) => h.cancel(),
            ResponseKind::Delete(h) => h.cancel(),
            ResponseKind::List(h) => h.cancel(),
            ResponseKind::Read(h) => h.cancel(),
            ResponseKind::Write(h) => h.cancel(),
            ResponseKind::Move(h) => h.cancel(),
            ResponseKind::Truncate(h) => h.cancel(),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryInfoResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Info(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryCreateResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Create(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryDeleteResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Delete(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryListResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::List(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryReadResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Read(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryWriteResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Write(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryMoveResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Move(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl TryFrom<ResponseKind> for SharedDirectoryTruncateResponseHandler {
    type Error = FilesystemBackendError;
    fn try_from(value: ResponseKind) -> Result<Self, Self::Error> {
        match value {
            ResponseKind::Truncate(h) => Ok(h),
            _ => Err(FilesystemBackendError("unexpected handler".to_string())),
        }
    }
}

impl From<SharedDirectoryInfoResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryInfoResponseHandler) -> Self {
        ResponseKind::Info(value)
    }
}

impl From<SharedDirectoryCreateResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryCreateResponseHandler) -> Self {
        ResponseKind::Create(value)
    }
}

impl From<SharedDirectoryDeleteResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryDeleteResponseHandler) -> Self {
        ResponseKind::Delete(value)
    }
}

impl From<SharedDirectoryReadResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryReadResponseHandler) -> Self {
        ResponseKind::Read(value)
    }
}

impl From<SharedDirectoryWriteResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryWriteResponseHandler) -> Self {
        ResponseKind::Write(value)
    }
}

impl From<SharedDirectoryListResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryListResponseHandler) -> Self {
        ResponseKind::List(value)
    }
}

impl From<SharedDirectoryMoveResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryMoveResponseHandler) -> Self {
        ResponseKind::Move(value)
    }
}

impl From<SharedDirectoryTruncateResponseHandler> for ResponseKind {
    fn from(value: SharedDirectoryTruncateResponseHandler) -> Self {
        ResponseKind::Truncate(value)
    }
}

type CompletionId = u32;

/// A generic cache for storing [`ResponseHandler`]s indexed by [`CompletionId`].
#[derive(Debug)]
struct ResponseCache {
    cache: HashMap<CompletionId, ResponseKind>,
}

impl ResponseCache {
    fn new() -> Self {
        Self {
            cache: HashMap::new(),
        }
    }

    fn contains(&self, completion_id: CompletionId) -> bool {
        self.cache.contains_key(&completion_id)
    }

    fn insert(
        &mut self,
        completion_id: u32,
        handler: ResponseKind,
    ) -> Result<(), FilesystemBackendError> {
        if self.contains(completion_id) {
            return Err(FilesystemBackendError(format!(
                "completion id {} already exists",
                completion_id
            )));
        };

        self.cache.insert(completion_id, handler);
        Ok(())
    }

    fn remove(&mut self, completion_id: &CompletionId) -> Option<ResponseKind> {
        self.cache.remove(completion_id)
    }

    fn drain(&mut self) -> impl std::iter::Iterator<Item = (CompletionId, ResponseKind)> + '_ {
        self.cache.drain()
    }
}

impl Cancel for efs::ServerDriveIoRequest {
    fn cancel(&self) -> RdpdrPdu {
        match self {
            efs::ServerDriveIoRequest::ServerCreateDriveRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveQueryInformationRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::DeviceCloseRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveQueryDirectoryRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveNotifyChangeDirectoryRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveQueryVolumeInformationRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::DeviceControlRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::DeviceReadRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::DeviceWriteRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveSetInformationRequest(h) => h.cancel(),
            efs::ServerDriveIoRequest::ServerDriveLockControlRequest(h) => h.cancel(),
        }
    }
}

struct DeviceId(u32);

impl From<DeviceId> for u32 {
    fn from(value: DeviceId) -> Self {
        value.0
    }
}

// Grab the DeviceId from any IO request
impl From<&efs::ServerDriveIoRequest> for DeviceId {
    fn from(value: &efs::ServerDriveIoRequest) -> Self {
        match value {
            efs::ServerDriveIoRequest::ServerCreateDriveRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryInformationRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::DeviceCloseRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryDirectoryRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveNotifyChangeDirectoryRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryVolumeInformationRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::DeviceControlRequest(h) => DeviceId(h.header.device_id),
            efs::ServerDriveIoRequest::DeviceReadRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::DeviceWriteRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveSetInformationRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
            efs::ServerDriveIoRequest::ServerDriveLockControlRequest(h) => {
                DeviceId(h.device_io_request.device_id)
            }
        }
    }
}
