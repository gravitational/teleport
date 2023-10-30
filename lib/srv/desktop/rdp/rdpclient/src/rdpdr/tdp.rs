use std::ffi::CString;

use super::path::UnixPath;
use crate::{
    util::{self, from_c_string, from_go_array},
    CGOSharedDirectoryAnnounce, CGOSharedDirectoryCreateRequest, CGOSharedDirectoryCreateResponse,
    CGOSharedDirectoryInfoRequest, CGOSharedDirectoryInfoResponse, CGOSharedDirectoryListResponse,
    CGOSharedDirectoryReadResponse,
};
use ironrdp_pdu::{custom_err, PduResult};
use ironrdp_rdpdr::pdu::efs::DeviceCreateRequest;

/// SharedDirectoryAnnounce is sent by the TDP client to the server
/// to announce a new directory to be shared over TDP.
#[derive(Debug)]
pub struct SharedDirectoryAnnounce {
    pub directory_id: u32,
    pub name: String,
}

impl From<CGOSharedDirectoryAnnounce> for SharedDirectoryAnnounce {
    fn from(cgo: CGOSharedDirectoryAnnounce) -> SharedDirectoryAnnounce {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        unsafe {
            SharedDirectoryAnnounce {
                directory_id: cgo.directory_id,
                name: from_c_string(cgo.name),
            }
        }
    }
}

/// SharedDirectoryAcknowledge is sent by the TDP server to the client
/// to acknowledge that a SharedDirectoryAnnounce was received.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryAcknowledge {
    pub err_code: TdpErrCode,
    pub directory_id: u32,
}

/// SharedDirectoryInfoRequest is sent from the TDP server to the client
/// to request information about a file or directory at a given path.
#[derive(Debug)]
pub struct SharedDirectoryInfoRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: UnixPath,
}

impl SharedDirectoryInfoRequest {
    /// Converts this request into a [`CGOSharedDirectoryInfoRequest`].
    ///
    /// Returns a tuple containing the [`CGOSharedDirectoryInfoRequest`] and a [`CString`],
    /// which is the memory backing the [`CGOSharedDirectoryInfoRequest::path`] field.
    /// It is the caller's responsibility to ensure that the [`CString`] lives until
    /// the [`CGOSharedDirectoryInfoRequest::path`] is copied into Go-owned memory.
    ///
    /// See the example for [`SharedDirectoryCreateRequest`]'s `into_cgo`.
    pub fn into_cgo(self) -> PduResult<(CGOSharedDirectoryInfoRequest, CString)> {
        let path = self.path.to_cstring()?;
        Ok((
            CGOSharedDirectoryInfoRequest {
                completion_id: self.completion_id,
                directory_id: self.directory_id,
                path: path.as_ptr(),
            },
            path,
        ))
    }
}

impl From<&DeviceCreateRequest> for SharedDirectoryInfoRequest {
    fn from(req: &DeviceCreateRequest) -> SharedDirectoryInfoRequest {
        SharedDirectoryInfoRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            path: UnixPath::from(&req.path),
        }
    }
}

/// SharedDirectoryInfoResponse is sent by the TDP client to the server
/// in response to a `Shared Directory Info Request`.
#[derive(Debug)]
pub struct SharedDirectoryInfoResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: FileSystemObject,
}

impl From<CGOSharedDirectoryInfoResponse> for SharedDirectoryInfoResponse {
    fn from(cgo_res: CGOSharedDirectoryInfoResponse) -> SharedDirectoryInfoResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        SharedDirectoryInfoResponse {
            completion_id: cgo_res.completion_id,
            err_code: cgo_res.err_code,
            fso: cgo_res.fso.into(),
        }
    }
}

#[derive(Debug, Clone)]
/// FileSystemObject is a TDP structure containing the metadata
/// of a file or directory.
pub struct FileSystemObject {
    pub last_modified: u64,
    pub size: u64,
    pub file_type: FileType,
    pub is_empty: u8,
    pub path: UnixPath,
}

impl FileSystemObject {
    pub fn name(&self) -> PduResult<String> {
        if let Some(name) = self.path.last() {
            Ok(name.to_string())
        } else {
            Err(custom_err!(
                "FileSystemObject::name",
                TdpHandlingError(format!("failed to extract name from path: {:?}", self.path))
            ))
        }
    }
}

/// SharedDirectoryWriteRequest is sent by the TDP server to the client
/// to write to a file.
#[derive(Clone)]
pub struct SharedDirectoryWriteRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub offset: u64,
    pub path: UnixPath,
    pub write_data: Vec<u8>,
}

impl std::fmt::Debug for SharedDirectoryWriteRequest {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SharedDirectoryWriteRequest")
            .field("completion_id", &self.completion_id)
            .field("directory_id", &self.directory_id)
            .field("offset", &self.offset)
            .field("path", &self.path)
            .field("write_data", &util::vec_u8_debug(&self.write_data))
            .finish()
    }
}

/// SharedDirectoryReadRequest is sent by the TDP server to the client
/// to request the contents of a file.
#[derive(Debug)]
pub struct SharedDirectoryReadRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: UnixPath,
    pub offset: u64,
    pub length: u32,
}

/// SharedDirectoryReadResponse is sent by the TDP client to the server
/// with the data as requested by a SharedDirectoryReadRequest.
#[repr(C)]
pub struct SharedDirectoryReadResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub read_data: Vec<u8>,
}

impl std::fmt::Debug for SharedDirectoryReadResponse {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SharedDirectoryReadResponse")
            .field("completion_id", &self.completion_id)
            .field("err_code", &self.err_code)
            .field("read_data", &util::vec_u8_debug(&self.read_data))
            .finish()
    }
}

impl From<CGOSharedDirectoryReadResponse> for SharedDirectoryReadResponse {
    fn from(cgo_response: CGOSharedDirectoryReadResponse) -> SharedDirectoryReadResponse {
        unsafe {
            SharedDirectoryReadResponse {
                completion_id: cgo_response.completion_id,
                err_code: cgo_response.err_code,
                read_data: from_go_array(cgo_response.read_data, cgo_response.read_data_length),
            }
        }
    }
}

/// SharedDirectoryWriteResponse is sent by the TDP client to the server
/// to acknowledge the completion of a SharedDirectoryWriteRequest.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryWriteResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub bytes_written: u32,
}

/// SharedDirectoryCreateRequest is sent by the TDP server to
/// the client to request the creation of a new file or directory.
#[derive(Debug)]
pub struct SharedDirectoryCreateRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub file_type: FileType,
    pub path: UnixPath,
}

impl SharedDirectoryCreateRequest {
    pub fn from(req: &DeviceCreateRequest, file_type: FileType) -> SharedDirectoryCreateRequest {
        SharedDirectoryCreateRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            file_type,
            path: UnixPath::from(&req.path),
        }
    }

    /// Converts this request into a [`CGOSharedDirectoryCreateRequest`].
    ///
    /// Returns a tuple containing the [`CGOSharedDirectoryCreateRequest`] and a [`CString`],
    /// which is the memory backing the [`CGOSharedDirectoryCreateRequest::path`] field.
    /// It is the caller's responsibility to ensure that the [`CString`] lives until
    /// the [`CGOSharedDirectoryCreateRequest::path`] is copied into Go-owned memory.
    ///
    /// ```
    /// fn example() {
    ///     let req = SharedDirectoryCreateRequest {
    ///         completion_id: 0,
    ///         directory_id: 0,
    ///         file_type: FileType::File,
    ///         path: UnixPath::from("/tmp/test.txt"),
    ///     };
    ///
    ///     // using _path (as opposed to _) ensures that the CString is not dropped
    ///     // until after the CGOSharedDirectoryCreateRequest is copied into Go-owned memory.
    ///     let (cgo_req, _path) = req.into_cgo().unwrap();
    ///
    ///     copy_into_go_memory(cgo_req);
    ///
    ///     // _path is dropped here
    /// }
    /// ```
    pub fn into_cgo(self) -> PduResult<(CGOSharedDirectoryCreateRequest, CString)> {
        let path = self.path.to_cstring()?;
        Ok((
            CGOSharedDirectoryCreateRequest {
                completion_id: self.completion_id,
                directory_id: self.directory_id,
                file_type: self.file_type,
                path: path.as_ptr(),
            },
            path,
        ))
    }
}

/// SharedDirectoryListResponse is sent by the TDP client to the server
/// in response to a SharedDirectoryInfoRequest.
#[derive(Debug)]
pub struct SharedDirectoryListResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso_list: Vec<FileSystemObject>,
}

impl From<CGOSharedDirectoryListResponse> for SharedDirectoryListResponse {
    fn from(cgo: CGOSharedDirectoryListResponse) -> SharedDirectoryListResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        unsafe {
            let cgo_fso_list = from_go_array(cgo.fso_list, cgo.fso_list_length);
            let mut fso_list = vec![];
            for cgo_fso in cgo_fso_list.into_iter() {
                fso_list.push(cgo_fso.into());
            }

            SharedDirectoryListResponse {
                completion_id: cgo.completion_id,
                err_code: cgo.err_code,
                fso_list,
            }
        }
    }
}

/// SharedDirectoryMoveRequest is sent from the TDP server to the client
/// to request a file at original_path be moved to new_path.
#[derive(Debug)]
pub struct SharedDirectoryMoveRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub original_path: UnixPath,
    pub new_path: UnixPath,
}

/// SharedDirectoryCreateResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryCreateRequest was received and executed.
#[derive(Debug)]
pub struct SharedDirectoryCreateResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: FileSystemObject,
}

impl From<CGOSharedDirectoryCreateResponse> for SharedDirectoryCreateResponse {
    fn from(cgo_res: CGOSharedDirectoryCreateResponse) -> SharedDirectoryCreateResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        SharedDirectoryCreateResponse {
            completion_id: cgo_res.completion_id,
            err_code: cgo_res.err_code,
            fso: cgo_res.fso.into(),
        }
    }
}

/// SharedDirectoryDeleteRequest is sent by the TDP server to the client
/// to request the deletion of a file or directory at path.
#[derive(Debug)]
pub struct SharedDirectoryDeleteRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: UnixPath,
}

/// SharedDirectoryDeleteResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryDeleteRequest was received and executed.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryDeleteResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
}

/// SharedDirectoryMoveResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryMoveRequest was received and expected.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryMoveResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
}

/// SharedDirectoryListRequest is sent by the TDP server to the client
/// to request the contents of a directory.
#[derive(Debug)]
pub struct SharedDirectoryListRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: UnixPath,
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum TdpErrCode {
    /// nil (no error, operation succeeded)
    Nil = 0,
    /// operation failed
    Failed = 1,
    /// resource does not exist
    DoesNotExist = 2,
    /// resource already exists
    AlreadyExists = 3,
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum FileType {
    File = 0,
    Directory = 1,
}

pub const FALSE: u8 = 0;
#[allow(dead_code)]
pub const TRUE: u8 = 1;

/// A generic error type that can contain any arbitrary error message.
///
/// TODO: This is a temporary solution until we can figure out a better error handling system.
#[derive(Debug)]
pub struct TdpHandlingError(pub String);

impl std::fmt::Display for TdpHandlingError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for TdpHandlingError {}
