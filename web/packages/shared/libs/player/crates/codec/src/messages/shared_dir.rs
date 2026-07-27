use crate::error::{CodecError, InvalidValue};
use crate::messages::enums::FileType;
use crate::messages::types::Fso;
use bytes::Bytes;

#[derive(Debug, Clone)]
pub enum ShareDirRequest {
    Info {
        completion_id: u32,
        directory_id: u32,
        path: String,
    },
    Create {
        completion_id: u32,
        directory_id: u32,
        file_type: FileType,
        path: String,
    },
    Delete {
        completion_id: u32,
        directory_id: u32,
        path: String,
    },
    Read {
        completion_id: u32,
        directory_id: u32,
        path: String,
        offset: u64,
        length: u32,
    },
    Write {
        completion_id: u32,
        directory_id: u32,
        path: String,
        offset: u64,
        data: Bytes,
    },
    Move {
        completion_id: u32,
        directory_id: u32,
        original: String,
        new: String,
    },
    List {
        completion_id: u32,
        directory_id: u32,
        path: String,
    },
    Truncate {
        completion_id: u32,
        directory_id: u32,
        path: String,
        /// New file size in bytes (was `end_of_file: u32` in legacy TDP).
        size: u64,
    },
}

#[derive(Debug, Clone)]
pub struct SharedDirInfoResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
    pub fso: Fso,
}

#[derive(Debug, Clone)]
pub struct SharedDirCreateResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
    pub fso: Fso,
}

#[derive(Debug, Clone, Copy)]
pub struct SharedDirDeleteResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
}

#[derive(Debug, Clone)]
pub struct SharedDirReadResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
    pub data: Bytes,
}

#[derive(Debug, Clone, Copy)]
pub struct SharedDirWriteResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
    pub bytes_written: u32,
}

#[derive(Debug, Clone, Copy)]
pub struct SharedDirMoveResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
}

#[derive(Debug, Clone)]
pub struct SharedDirListResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
    pub fsos: Vec<Fso>,
}

#[derive(Debug, Clone, Copy)]
pub struct SharedDirTruncateResponse {
    pub completion_id: u32,
    pub err: SharedDirErrCode,
}

#[derive(Debug, Clone, Copy)]
pub struct ShareDirAck {
    pub err: SharedDirErrCode,
    pub directory_id: u32,
}

#[derive(Debug, Clone)]
pub struct ShareDirAnnounce {
    pub directory_id: u32,
    pub name: String,
}

#[derive(Debug, Clone, Copy)]
pub struct ShareDirRemove {
    pub directory_id: u32,
}

#[derive(Debug, Clone)]
pub enum ShareDirResponse {
    Info(SharedDirInfoResponse),
    Create(SharedDirCreateResponse),
    Delete(SharedDirDeleteResponse),
    List(SharedDirListResponse),
    Read(SharedDirReadResponse),
    Write(SharedDirWriteResponse),
    Move(SharedDirMoveResponse),
    Truncate(SharedDirTruncateResponse),
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u32)]
pub enum SharedDirErrCode {
    Nil = 0,
    Failed = 1,
    DoesNotExist = 2,
    AlreadyExists = 3,
}

impl TryFrom<u32> for SharedDirErrCode {
    type Error = CodecError;

    fn try_from(v: u32) -> Result<Self, CodecError> {
        match v {
            0 => Ok(Self::Nil),
            1 => Ok(Self::Failed),
            2 => Ok(Self::DoesNotExist),
            3 => Ok(Self::AlreadyExists),
            n => Err(CodecError::Invalid(InvalidValue::ErrCode(n))),
        }
    }
}

impl From<SharedDirErrCode> for u32 {
    fn from(e: SharedDirErrCode) -> Self {
        e as u32
    }
}
