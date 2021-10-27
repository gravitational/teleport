use rdp::model::error::*;

// Helper function to return an unmarshaling error.
pub fn invalid_data_error(msg: &str) -> Error {
    Error::RdpError(RdpError::new(RdpErrorKind::InvalidData, msg))
}

// NTSTATUS_OK is a Windows NTStatus value that means "success".
pub const NTSTATUS_OK: u32 = 0;
// SPECIAL_NO_RESPONSE is our custom (not defined by Windows) NTStatus value that means "don't send
// a response to this message". Used in scard.rs.
pub const SPECIAL_NO_RESPONSE: u32 = 0xffffffff;
