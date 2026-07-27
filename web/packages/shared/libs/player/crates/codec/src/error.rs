use teleport_proto::prost;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum CodecError {
    #[error("failed to decode protobuf message: {0}")]
    Decode(#[from] prost::DecodeError),
    #[error("missing required field: {0}")]
    Missing(&'static str),
    #[error("invalid value: {0}")]
    Invalid(#[from] InvalidValue),
    #[error("truncated: ran out of bytes reading {0}")]
    Truncated(&'static str),
    #[error("unknown TDP message type: {0}")]
    UnknownTdpType(u8),
    #[error("encode error: {0}")]
    Encode(#[from] EncodeError),
    #[error("malformed JSON in {field}: {message}")]
    MalformedJson {
        field: &'static str,
        message: String,
    },
}

/// Enum/tag values that didn't match any known variant. Width follows the
/// wire: `i32` for fields sourced from a proto enum or TDP byte, `u32` for
/// fields whose only source is a proto `uint32`.
#[derive(Debug, Error)]
pub enum InvalidValue {
    #[error("mouse button: {0}")]
    MouseButton(i32),
    #[error("button state: {0}")]
    ButtonState(i32),
    #[error("scroll axis: {0}")]
    ScrollAxis(i32),
    #[error("severity: {0}")]
    Severity(i32),
    #[error("mfa kind: {0}")]
    MfaKind(i32),
    #[error("shared-dir error code: {0}")]
    ErrCode(u32),
    #[error("file type: {0}")]
    FileType(u32),
    #[error("clipboard data is not valid utf-8")]
    Utf8,
}

/// Encode-side failures: in-memory values that can't be serialised faithfully
/// to the wire. `field` names the offending slot for diagnostics.
#[derive(Debug, Error)]
pub enum EncodeError {
    #[error("length of {field} ({len} bytes) exceeds u32::MAX")]
    LengthOverflow { field: &'static str, len: usize },
    /// e.g. an RDP channel id wider than `u16`, or a mouse-wheel delta
    /// wider than `i16` — TDP's narrower slot can't represent it.
    #[error("value of {field} ({value}) out of range for {target}")]
    ValueOverflow {
        field: &'static str,
        value: i64,
        target: &'static str,
    },
    #[error("json {0}: {1}")]
    Json(&'static str, String),
    #[error("base64 decode of {0} failed")]
    Base64(&'static str),
    #[error("missing required input field: {0}")]
    MissingInput(&'static str),
    #[error("{0} is not representable in TDP")]
    UnsupportedInTdp(&'static str),
}
