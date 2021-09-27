use rdp::model::error::*;

pub fn invalid_data_error(msg: &str) -> Error {
    Error::RdpError(RdpError::new(RdpErrorKind::InvalidData, msg))
}

pub const NTSTATUS_OK: u32 = 0;
pub const SPECIAL_NO_RESPONSE: u32 = 0xffffffff;
