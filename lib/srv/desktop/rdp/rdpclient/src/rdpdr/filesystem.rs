// Copyright 2023 Gravitational, Inc
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

use super::{
    path::UnixPath,
    tdp::{self, SharedDirectoryDeleteRequest, TdpErrCode},
};
use crate::{
    client::ClientHandle, tdp_sd_create_request, tdp_sd_delete_request, tdp_sd_info_request,
    CGOErrCode, CgoHandle,
};
use ironrdp_pdu::{cast_length, custom_err, other_err, PduResult};
use ironrdp_rdpdr::pdu::{
    efs::{self, NtStatus},
    RdpdrPdu,
};
use std::collections::HashMap;
use std::convert::TryInto;

/// `FilesystemBackend` implements the filesystem redirection backend as described in [\[MS-RDPEFS\]: Remote Desktop Protocol: File System Virtual Channel Extension].
/// It does so in concert with the TDP directory sharing extension described in [RFD 0067].
///
/// [\[MS-RDPEFS\]: Remote Desktop Protocol: File System Virtual Channel Extension]: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/34d9de58-b2b5-40b6-b970-f82d4603bdb5
/// [RFD 0067]: https://github.com/gravitational/teleport/blob/master/rfd/0067-desktop-access-file-system-sharing.md
#[derive(Debug)]
pub struct FilesystemBackend {
    cgo_handle: CgoHandle,
    client_handle: ClientHandle,
    /// FileId-indexed cache of FileCacheObjects.
    /// See the documentation of FileCacheObject
    /// for more detail on how this is used.
    file_cache: FileCache,
    /// CompletionId -> [`SharedDirectoryInfoResponseHandler`]
    pending_tdp_sd_info_resp_handlers: HashMap<u32, SharedDirectoryInfoResponseHandler>,
    /// CompletionId -> [`SharedDirectoryCreateResponseHandler`]
    pending_sd_create_resp_handlers: HashMap<u32, SharedDirectoryCreateResponseHandler>,
    /// CompletionId -> [`SharedDirectoryDeleteResponseHandler`]
    pending_sd_delete_resp_handlers: HashMap<u32, SharedDirectoryDeleteResponseHandler>,
}

impl FilesystemBackend {
    pub fn new(cgo_handle: CgoHandle, client_handle: ClientHandle) -> Self {
        Self {
            cgo_handle,
            client_handle,
            file_cache: FileCache::new(),
            pending_tdp_sd_info_resp_handlers: HashMap::new(),
            pending_sd_create_resp_handlers: HashMap::new(),
            pending_sd_delete_resp_handlers: HashMap::new(),
        }
    }

    pub fn handle(&mut self, req: efs::ServerDriveIoRequest) -> PduResult<()> {
        match req {
            efs::ServerDriveIoRequest::ServerCreateDriveRequest(req) => {
                self.handle_device_create_req(req)
            }
            efs::ServerDriveIoRequest::ServerDriveQueryInformationRequest(req) => {
                self.handle_query_information_req(req)
            }
        }
    }

    /// Handles an RDP [`efs::DeviceCreateRequest`] received from the RDP server.
    fn handle_device_create_req(&mut self, rdp_req: efs::DeviceCreateRequest) -> PduResult<()> {
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L210
        self.send_tdp_sd_info_request(tdp::SharedDirectoryInfoRequest::from(&rdp_req))?;
        self.pending_tdp_sd_info_resp_handlers.insert(
            rdp_req.device_io_request.completion_id,
            SharedDirectoryInfoResponseHandler::new(Box::new(
                |this: &mut FilesystemBackend,
                 tdp_resp: tdp::SharedDirectoryInfoResponse|
                 -> PduResult<()> {
                    this.handle_device_create_req_continued(rdp_req, tdp_resp)
                },
            )),
        );
        Ok(())
    }

    /// Continues [`Self::handle_rdp_device_create_req`] after a [`tdp::SharedDirectoryInfoResponse`] is received from the browser,
    /// returning any [`RdpdrPdu`]s that need to be sent back to the RDP server.
    fn handle_device_create_req_continued(
        &mut self,
        req: efs::DeviceCreateRequest,
        res: tdp::SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        match res.err_code {
            TdpErrCode::Failed | TdpErrCode::AlreadyExists => {
                return Err(custom_err!(
                    "FilesystemBackend::pending_tdp_sd_info_resp_handlers",
                    FilesystemBackendError(format!(
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
                        return self.send_device_create_response(
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
                        return self.send_device_create_response(
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
                    return self.send_device_create_response(
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
                    if req.create_disposition.intersects(
                        efs::CreateDisposition::FILE_OPEN_IF | efs::CreateDisposition::FILE_CREATE,
                    ) {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L252
                        self.tdp_sd_create(req, tdp::FileType::Directory)?;
                        return Ok(());
                    } else {
                        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L258
                        return self.send_device_create_response(
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
                    let file_id = self
                        .file_cache
                        .insert(FileCacheObject::new(UnixPath::from(&req.path), res.fso))?;
                    return self.send_device_create_response(&req, efs::NtStatus::SUCCESS, file_id);
                } else if res.err_code == TdpErrCode::DoesNotExist {
                    return self.send_device_create_response(&req, efs::NtStatus::NO_SUCH_FILE, 0);
                }
            }
            efs::CreateDisposition::FILE_CREATE => {
                // If the file already exists, fail the request and do not create or open the given file. If it does not, create the given file.
                if res.err_code == TdpErrCode::Nil {
                    return self.send_device_create_response(
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
                    let file_id = self
                        .file_cache
                        .insert(FileCacheObject::new(UnixPath::from(&req.path), res.fso))?;
                    return self.send_device_create_response(&req, efs::NtStatus::SUCCESS, file_id);
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
                    return self.send_device_create_response(&req, efs::NtStatus::NO_SUCH_FILE, 0);
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
                return Err(custom_err!(
                    "FilesystemBackend::pending_tdp_sd_info_resp_handlers",
                    FilesystemBackendError(format!(
                        "received unknown CreateDisposition value for RDP {req:?}",
                        req = req
                    ))
                ));
            }
        }

        Err(other_err!(
            "FilesystemBackend::pending_tdp_sd_info_resp_handlers",
            "Programmer error, this line should never be reached"
        ))
    }

    /// Handles an RDP [`efs::ServerDriveQueryInformationRequest`] received from the RDP server.
    fn handle_query_information_req(
        &mut self,
        rdp_req: efs::ServerDriveQueryInformationRequest,
    ) -> PduResult<()> {
        let file = self.file_cache.get(rdp_req.device_io_request.file_id);
        let rdp_resp = Self::make_client_drive_query_information_response(rdp_req, file)?;
        self.client_handle.write_rdpdr(rdp_resp)?;
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
        self.pending_sd_create_resp_handlers.insert(
            rdp_req.device_io_request.completion_id,
            SharedDirectoryCreateResponseHandler::new(Box::new(
                move |this: &mut FilesystemBackend,
                      tdp_resp: tdp::SharedDirectoryCreateResponse|
                      -> PduResult<()> {
                    if tdp_resp.err_code != TdpErrCode::Nil {
                        return this.send_device_create_response(
                            &rdp_req,
                            NtStatus::UNSUCCESSFUL,
                            0,
                        );
                    }
                    let file_id = this.file_cache.insert(FileCacheObject::new(
                        UnixPath::from(&rdp_req.path),
                        tdp_resp.fso,
                    ))?;
                    this.send_device_create_response(&rdp_req, NtStatus::SUCCESS, file_id)
                },
            )),
        );
        Ok(())
    }

    /// Helper function for combining a [`tdp::SharedDirectoryDeleteRequest`]
    /// with a [`tdp::SharedDirectoryCreateRequest`] to overwrite a file.
    fn tdp_sd_overwrite(&mut self, rdp_req: efs::DeviceCreateRequest) -> PduResult<()> {
        let tdp_req = SharedDirectoryDeleteRequest::from(&rdp_req);
        self.send_tdp_sd_delete_request(tdp_req)?;
        self.pending_sd_delete_resp_handlers.insert(
            rdp_req.device_io_request.completion_id,
            SharedDirectoryDeleteResponseHandler::new(Box::new(
                move |this: &mut FilesystemBackend,
                      tdp_resp: tdp::SharedDirectoryDeleteResponse|
                      -> PduResult<()> {
                    match tdp_resp.err_code {
                        TdpErrCode::Nil => {
                            this.tdp_sd_create(rdp_req, tdp::FileType::File)?;
                            Ok(())
                        }
                        _ => this.send_device_create_response(&rdp_req, NtStatus::UNSUCCESSFUL, 0),
                    }
                },
            )),
        );
        Ok(())
    }

    /// Sends a [`tdp::SharedDirectoryInfoRequest`] to the browser.
    fn send_tdp_sd_info_request(&self, tdp_req: tdp::SharedDirectoryInfoRequest) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let (mut req, _path) = tdp_req.into_cgo()?;
        let err = unsafe { tdp_sd_info_request(self.cgo_handle, &mut req) };
        if err != CGOErrCode::ErrCodeSuccess {
            return Err(custom_err!(
                "FilesystemBackend::send_tdp_sd_info_request",
                FilesystemBackendError(format!("call to tdp_sd_info_request failed: {:?}", err))
            ));
        };
        Ok(())
    }

    /// Sends a [`tdp::SharedDirectoryCreateRequest`] to the browser.
    fn send_tdp_sd_create_request(
        &self,
        tdp_req: tdp::SharedDirectoryCreateRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let (mut req, _path) = tdp_req.into_cgo()?;
        let err = unsafe { tdp_sd_create_request(self.cgo_handle, &mut req) };
        if err != CGOErrCode::ErrCodeSuccess {
            return Err(custom_err!(
                "FilesystemBackend::send_tdp_sd_create_request",
                FilesystemBackendError(format!("call to tdp_sd_create_request failed: {:?}", err))
            ));
        };
        Ok(())
    }

    /// Sends a [`tdp::SharedDirectoryDeleteRequest`] to the browser.
    fn send_tdp_sd_delete_request(
        &self,
        tdp_req: tdp::SharedDirectoryDeleteRequest,
    ) -> PduResult<()> {
        debug!("sending tdp: {:?}", tdp_req);
        let (mut req, _path) = tdp_req.into_cgo()?;
        let err = unsafe { tdp_sd_delete_request(self.cgo_handle, &mut req) };
        if err != CGOErrCode::ErrCodeSuccess {
            return Err(custom_err!(
                "FilesystemBackend::send_tdp_sd_delete_request",
                FilesystemBackendError(format!("call to tdp_sd_create_request failed: {:?}", err))
            ));
        };
        Ok(())
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryInfoResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryInfoResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryInfoResponse`].
    pub fn handle_tdp_sd_info_response(
        &mut self,
        tdp_resp: tdp::SharedDirectoryInfoResponse,
    ) -> PduResult<()> {
        if let Some(handler) = self
            .pending_tdp_sd_info_resp_handlers
            .remove(&tdp_resp.completion_id)
        {
            handler.call(self, tdp_resp)
        } else {
            Err(custom_err!(
                "FilesystemBackend::handle_tdp_sd_info_response",
                FilesystemBackendError(format!(
                    "received invalid completion id: {}",
                    tdp_resp.completion_id
                ))
            ))
        }
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryCreateResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryCreateResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryCreateResponse`].
    pub fn handle_tdp_sd_create_response(
        &mut self,
        res: tdp::SharedDirectoryCreateResponse,
    ) -> PduResult<()> {
        if let Some(handler) = self
            .pending_sd_create_resp_handlers
            .remove(&res.completion_id)
        {
            handler.call(self, res)
        } else {
            Err(custom_err!(
                "FilesystemBackend::handle_tdp_sd_create_response",
                FilesystemBackendError(format!(
                    "received invalid completion id: {}",
                    res.completion_id
                ))
            ))
        }
    }

    /// Called from the Go code when a [`tdp::SharedDirectoryDeleteResponse`] is received from the browser.
    ///
    /// Calls the [`SharedDirectoryDeleteResponseHandler`] associated with the completion id of the
    /// [`tdp::SharedDirectoryDeleteResponse`].
    pub fn handle_tdp_sd_delete_response(
        &mut self,
        res: tdp::SharedDirectoryDeleteResponse,
    ) -> PduResult<()> {
        if let Some(handler) = self
            .pending_sd_delete_resp_handlers
            .remove(&res.completion_id)
        {
            handler.call(self, res)
        } else {
            Err(custom_err!(
                "FilesystemBackend::client_handle_tdp_sd_delete_response",
                FilesystemBackendError(format!(
                    "received invalid completion id: {}",
                    res.completion_id
                ))
            ))
        }
    }

    /// Helper function for creating an RDP [`efs::DeviceCreateResponse`] from an RDP [`efs::DeviceCreateRequest`].
    fn send_device_create_response(
        &self,
        device_create_request: &efs::DeviceCreateRequest,
        io_status: efs::NtStatus,
        new_file_id: u32,
    ) -> PduResult<()> {
        // See https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L187-L228
        let information = if io_status != efs::NtStatus::SUCCESS
            || device_create_request.create_disposition.intersects(
                efs::CreateDisposition::FILE_SUPERSEDE
                    | efs::CreateDisposition::FILE_OPEN
                    | efs::CreateDisposition::FILE_CREATE
                    | efs::CreateDisposition::FILE_OVERWRITE,
            ) {
            Ok(efs::Information::FILE_SUPERSEDED)
        } else if device_create_request.create_disposition == efs::CreateDisposition::FILE_OPEN_IF {
            Ok(efs::Information::FILE_OPENED)
        } else if device_create_request.create_disposition
            == efs::CreateDisposition::FILE_OVERWRITE_IF
        {
            Ok(efs::Information::FILE_OVERWRITTEN)
        } else {
            Err(other_err!(
                "FilesystemBackend::make_device_create_response",
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

    fn make_client_drive_query_information_response(
        rdp_req: efs::ServerDriveQueryInformationRequest,
        file: Option<&FileCacheObject>,
    ) -> PduResult<RdpdrPdu> {
        if file.is_none() {
            return Ok(RdpdrPdu::ClientDriveQueryInformationResponse(
                efs::ClientDriveQueryInformationResponse {
                    device_io_response: efs::DeviceIoResponse::new(
                        rdp_req.device_io_request.clone(),
                        NtStatus::UNSUCCESSFUL,
                    ),
                    buffer: None,
                },
            ));
        }
        let file = file.unwrap(); // safe because of the if statement above
        let device_io_response =
            efs::DeviceIoResponse::new(rdp_req.device_io_request.clone(), NtStatus::SUCCESS);

        // We support all the FsInformationClasses that FreeRDP does here
        // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L482
        match rdp_req.file_info_class_lvl {
            efs::FileInformationClassLevel::FILE_BASIC_INFORMATION => {
                Ok(RdpdrPdu::ClientDriveQueryInformationResponse(
                    efs::ClientDriveQueryInformationResponse {
                        device_io_response,
                        buffer: Some(efs::FileInformationClass::Basic(
                            efs::FileBasicInformation {
                                creation_time: to_windows_time(file.fso.last_modified),
                                last_access_time: to_windows_time(file.fso.last_modified),
                                last_write_time: to_windows_time(file.fso.last_modified),
                                change_time: to_windows_time(file.fso.last_modified),
                                file_attributes: if file.fso.file_type == tdp::FileType::File {
                                    efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
                                } else {
                                    efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
                                },
                            },
                        )),
                    },
                ))
            }
            efs::FileInformationClassLevel::FILE_STANDARD_INFORMATION => {
                let file_fso_size: i64 = cast_length!(
                    "FilesystemBackend::make_client_drive_query_information_response",
                    "file.fso.size",
                    file.fso.size
                )?;
                Ok(RdpdrPdu::ClientDriveQueryInformationResponse(
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
                    },
                ))
            }
            efs::FileInformationClassLevel::FILE_ATTRIBUTE_TAG_INFORMATION => {
                Ok(RdpdrPdu::ClientDriveQueryInformationResponse(
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
                    },
                ))
            }
            _ => Err(custom_err!(
                "FilesystemBackend::make_client_drive_query_information_response",
                FilesystemBackendError(format!(
                    "received unsupported FileInformationClass: {:?}",
                    rdp_req.file_info_class_lvl
                ))
            )),
        }
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
    fn insert(&mut self, file: FileCacheObject) -> PduResult<u32> {
        self.next_file_id = self.next_file_id.wrapping_add(1);
        if self.cache.insert(self.next_file_id, file).is_none() {
            Ok(self.next_file_id)
        } else {
            Err(other_err!(
                "FileCache::insert",
                "attempted to insert a FileCacheObject into the file cache with a file_id that already exists in the cache"
            ))
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
struct FileCacheObject {
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
}

/// FileCacheObject is used as an iterator for the implementation of
/// IRP_MJ_DIRECTORY_CONTROL, which requires that we iterate through
/// all the files of a directory one by one. In this case, the directory
/// is the FileCacheObject itself, with it's own fso field representing
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

/// TDP handles time in milliseconds since the UNIX epoch (https://en.wikipedia.org/wiki/Unix_time),
/// whereas Windows prefers 64-bit signed integers representing the number of 100-nanosecond intervals
/// that have elapsed since January 1, 1601, Coordinated Universal Time (UTC)
/// (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/a69cc039-d288-4673-9598-772b6083f8bf).
fn to_windows_time(tdp_time_ms: u64) -> i64 {
    // https://stackoverflow.com/a/5471380/6277051
    // https://docs.microsoft.com/en-us/windows/win32/sysinfo/converting-a-time-t-value-to-a-file-time
    let tdp_time_sec = tdp_time_ms / 1000;
    ((tdp_time_sec * 10000000) + 116444736000000000) as i64
}

/// A generic error type for the FilesystemBackend that can contain any arbitrary error message.
#[derive(Debug)]
struct FilesystemBackendError(pub String);

impl std::fmt::Display for FilesystemBackendError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for FilesystemBackendError {}

/// A macro for generating a response handler struct and function boilerplate for a given response type.
///
/// Call this like:
///
/// ```
/// response_handler!(
///   SharedDirectoryInfoResponseHandler,
///   InfoResponseHandlerFunction,
///   tdp::SharedDirectoryInfoResponse
/// );
/// ```
///
/// in order to generate the following:
///
/// ```
/// type InfoResponseHandlerFunction = Box<
///     dyn FnOnce(
///             &mut FilesystemBackend,
///             tdp::SharedDirectoryInfoResponse,
///         ) -> PduResult<Option<RdpdrPdu>>
///         + Send,
/// >;
///
/// struct SharedDirectoryInfoResponseHandler {
///     handler: InfoResponseHandlerFunction,
/// }
///
/// impl SharedDirectoryInfoResponseHandler {
///     fn new(handler: InfoResponseHandlerFunction) -> Self {
///         Self { handler }
///     }
///
///     fn call(
///         self,
///         this: &mut FilesystemBackend,
///         res: tdp::SharedDirectoryInfoResponse,
///     ) -> PduResult<Option<RdpdrPdu>> {
///         (self.handler)(this, res)
///     }
/// }
///
/// impl std::fmt::Debug for SharedDirectoryInfoResponseHandler {
///     fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
///         write!(f, "<SharedDirectoryInfoResponseHandler>")
///     }
/// }
/// ```
macro_rules! response_handler {
    ($name:ident, $func_name:ident, $response_type:ty) => {
        type $func_name =
            Box<dyn FnOnce(&mut FilesystemBackend, $response_type) -> PduResult<()> + Send>;

        struct $name {
            handler: $func_name,
        }

        impl $name {
            fn new(handler: $func_name) -> Self {
                Self { handler }
            }

            fn call(self, this: &mut FilesystemBackend, res: $response_type) -> PduResult<()> {
                (self.handler)(this, res)
            }
        }

        impl std::fmt::Debug for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(f, concat!("<", stringify!($name), ">"))
            }
        }
    };
}

response_handler!(
    SharedDirectoryInfoResponseHandler,
    InfoResponseHandlerFunction,
    tdp::SharedDirectoryInfoResponse
);

response_handler!(
    SharedDirectoryCreateResponseHandler,
    CreateResponseHandlerFunction,
    tdp::SharedDirectoryCreateResponse
);

response_handler!(
    SharedDirectoryDeleteResponseHandler,
    DeleteResponseHandlerFunction,
    tdp::SharedDirectoryDeleteResponse
);
