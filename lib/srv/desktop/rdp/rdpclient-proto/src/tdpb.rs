// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

use crate::tdpb::shared_directory_request::Operation;
use ironrdp_pdu::{pdu_other_err, PduResult};
use ironrdp_rdpdr::pdu::efs;
use rdp_client_common::path::UnixPath;
use std::fmt::Formatter;

// Re-export the generated TDPB protocol types under the `tdpb` module.
pub use crate::desktop::v1::{
    envelope, shared_directory_request, shared_directory_response, Alert, ClipboardData,
    ConnectionActivated, Envelope, FastPathPdu, FileSystemObject, KeyboardButton, MouseButton,
    MouseButtonType, MouseMove, MouseWheel, MouseWheelAxis, ServerHello,
    SharedDirectoryAcknowledge, SharedDirectoryAnnounce, SharedDirectoryRemove,
    SharedDirectoryRequest, SharedDirectoryResponse, SyncKeys,
};

impl FileSystemObject {
    pub fn name(&self) -> PduResult<String> {
        let path = UnixPath::from(&self.path);
        if let Some(name) = path.last() {
            Ok(name.to_string())
        } else {
            Err(pdu_other_err!(
                "",
                source: TdpbError(format!(
                    "failed to extract name from path: {:?}",
                    self.path
                ))
            ))
        }
    }

    pub fn into_both_directory(self) -> PduResult<efs::FileBothDirectoryInformation> {
        let file_attributes = if self.file_type == FileType::Directory as u32 {
            efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
        } else {
            efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
        };

        let last_modified = to_windows_time(self.last_modified);

        Ok(efs::FileBothDirectoryInformation::new(
            last_modified,
            last_modified,
            last_modified,
            last_modified,
            self.size.try_into().map_err(|e| {
                pdu_other_err!(
                    "",
                    source: TdpbError(format!(
                        "FileSystemObject::into_both_directory: can't convert self.size: {e}"
                    ))
                )
            })?,
            file_attributes,
            self.name()?,
        ))
    }

    pub fn into_full_directory(self) -> PduResult<efs::FileFullDirectoryInformation> {
        let file_attributes = if self.file_type == FileType::Directory as u32 {
            efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
        } else {
            efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
        };

        let last_modified = to_windows_time(self.last_modified);

        Ok(efs::FileFullDirectoryInformation::new(
            last_modified,
            last_modified,
            last_modified,
            last_modified,
            self.size.try_into().map_err(|e| {
                pdu_other_err!(
                    "",
                    source: TdpbError(format!(
                        "FileSystemObject::into_full_directory: can't convert self.size: {e}"
                    ))
                )
            })?,
            file_attributes,
            self.name()?,
        ))
    }

    pub fn into_names(self) -> PduResult<efs::FileNamesInformation> {
        Ok(efs::FileNamesInformation::new(self.name()?))
    }

    pub fn into_directory(self) -> PduResult<efs::FileDirectoryInformation> {
        let file_attributes = if self.file_type == FileType::Directory as u32 {
            efs::FileAttributes::FILE_ATTRIBUTE_DIRECTORY
        } else {
            efs::FileAttributes::FILE_ATTRIBUTE_NORMAL
        };

        let last_modified = to_windows_time(self.last_modified);

        Ok(efs::FileDirectoryInformation::new(
            last_modified,
            last_modified,
            last_modified,
            last_modified,
            self.size.try_into().map_err(|e| {
                pdu_other_err!(
                    "",
                    source: TdpbError(format!(
                        "FileSystemObject::into_directory: can't convert self.size: {e}"
                    ))
                )
            })?,
            file_attributes,
            self.name()?,
        ))
    }

    pub fn is_empty_directory(&self) -> bool {
        self.file_type == FileType::Directory as u32 && self.is_empty
    }

    pub fn is_non_empty_directory(&self) -> bool {
        self.file_type == FileType::Directory as u32 && !self.is_empty
    }

    pub fn is_file(&self) -> bool {
        self.file_type == FileType::File as u32
    }
}

impl SharedDirectoryRequest {
    pub fn info_req(req: &efs::DeviceCreateRequest) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            directory_id: req.device_io_request.device_id,
            completion_id: req.device_io_request.completion_id,
            operation: Some(Operation::Info(shared_directory_request::Info {
                path: UnixPath::from(&req.path).path,
            })),
        }
    }

    pub fn create_req(
        req: &efs::DeviceCreateRequest,
        file_type: FileType,
    ) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            operation: Some(Operation::Create(shared_directory_request::Create {
                path: UnixPath::from(&req.path).path,
                file_type: file_type as u32,
            })),
        }
    }

    pub fn delete_req(req: &efs::DeviceCreateRequest) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            operation: Some(Operation::Delete(shared_directory_request::Delete {
                path: UnixPath::from(&req.path).path,
            })),
        }
    }

    pub fn delete_req_from_fco(
        req: &efs::DeviceCloseRequest,
        path: UnixPath,
    ) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            operation: Some(Operation::Delete(shared_directory_request::Delete {
                path: path.path,
            })),
        }
    }

    pub fn read_req(req: &efs::DeviceReadRequest, path: UnixPath) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            operation: Some(Operation::Read(shared_directory_request::Read {
                path: path.path,
                offset: req.offset,
                length: req.length,
            })),
        }
    }

    pub fn write_req(req: &efs::DeviceWriteRequest, path: UnixPath) -> SharedDirectoryRequest {
        SharedDirectoryRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            operation: Some(Operation::Write(shared_directory_request::Write {
                path: path.path,
                offset: req.offset,
                data: req.write_data.clone(),
            })),
        }
    }
}

#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum ErrCode {
    /// Nil (no error, operation succeeded)
    Nil = 0,
    /// Operation failed
    Failed = 1,
    /// Resource does not exist
    DoesNotExist = 2,
    /// Resource already exists
    AlreadyExists = 3,
}

impl TryFrom<u32> for ErrCode {
    type Error = u32;

    fn try_from(value: u32) -> Result<Self, Self::Error> {
        match value {
            0 => Ok(ErrCode::Nil),
            1 => Ok(ErrCode::Failed),
            2 => Ok(ErrCode::DoesNotExist),
            3 => Ok(ErrCode::AlreadyExists),
            _ => Err(value),
        }
    }
}

#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum FileType {
    File = 0,
    Directory = 1,
}

impl std::fmt::Debug for shared_directory_request::Write {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("shared_directory_request::Write")
            .field("path", &self.path)
            .field("offset", &self.offset)
            .field("data", &format_args!("&[u8] of length {}", self.data.len()))
            .finish()
    }
}

impl std::fmt::Debug for shared_directory_response::Read {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("shared_directory_response::Read")
            .field("data", &format_args!("&[u8] of length {}", self.data.len()))
            .finish()
    }
}

/// TDP handles time in milliseconds since the UNIX epoch (https://en.wikipedia.org/wiki/Unix_time),
/// whereas Windows prefers 64-bit signed integers representing the number of 100-nanosecond intervals
/// that have elapsed since January 1, 1601, Coordinated Universal Time (UTC)
/// (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/a69cc039-d288-4673-9598-772b6083f8bf).
pub fn to_windows_time(tdp_time_ms: u64) -> i64 {
    // https://stackoverflow.com/a/5471380/6277051
    // https://docs.microsoft.com/en-us/windows/win32/sysinfo/converting-a-time-t-value-to-a-file-time
    let tdp_time_sec = tdp_time_ms / 1000;
    ((tdp_time_sec * 10000000) + 116444736000000000) as i64
}

/// A generic error type that can contain any arbitrary error message.
///
/// TODO: This is a temporary solution until we can figure out a better error handling system.
#[allow(dead_code)]
#[derive(Debug)]
pub struct TdpbError(pub String);

impl core::fmt::Display for TdpbError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for TdpbError {}
