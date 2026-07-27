//! TDP wire-format message type byte. Mirrors `MessageType` in
//! `web/packages/shared/libs/tdp/codec.ts`.

use crate::error::CodecError;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum MessageType {
    ClientScreenSpec = 1,
    PngFrame = 2,
    MouseMove = 3,
    MouseButton = 4,
    KeyboardButton = 5,
    ClipboardData = 6,
    ClientUsername = 7,
    MouseWheelScroll = 8,
    Error = 9,
    MfaJson = 10,
    SharedDirectoryAnnounce = 11,
    SharedDirectoryAcknowledge = 12,
    SharedDirectoryInfoRequest = 13,
    SharedDirectoryInfoResponse = 14,
    SharedDirectoryCreateRequest = 15,
    SharedDirectoryCreateResponse = 16,
    SharedDirectoryDeleteRequest = 17,
    SharedDirectoryDeleteResponse = 18,
    SharedDirectoryReadRequest = 19,
    SharedDirectoryReadResponse = 20,
    SharedDirectoryWriteRequest = 21,
    SharedDirectoryWriteResponse = 22,
    SharedDirectoryMoveRequest = 23,
    SharedDirectoryMoveResponse = 24,
    SharedDirectoryListRequest = 25,
    SharedDirectoryListResponse = 26,
    Png2Frame = 27,
    Alert = 28,
    RdpFastPathPdu = 29,
    RdpResponsePdu = 30,
    RdpConnectionActivated = 31,
    SyncKeys = 32,
    SharedDirectoryTruncateRequest = 33,
    SharedDirectoryTruncateResponse = 34,
    LatencyStats = 35,
    // 36 is reserved for a server→server Ping that never reaches the client.
    ClientKeyboardLayout = 37,
    TdpbUpgrade = 38,
}

impl TryFrom<u8> for MessageType {
    type Error = CodecError;

    fn try_from(v: u8) -> Result<Self, CodecError> {
        Ok(match v {
            1 => Self::ClientScreenSpec,
            2 => Self::PngFrame,
            3 => Self::MouseMove,
            4 => Self::MouseButton,
            5 => Self::KeyboardButton,
            6 => Self::ClipboardData,
            7 => Self::ClientUsername,
            8 => Self::MouseWheelScroll,
            9 => Self::Error,
            10 => Self::MfaJson,
            11 => Self::SharedDirectoryAnnounce,
            12 => Self::SharedDirectoryAcknowledge,
            13 => Self::SharedDirectoryInfoRequest,
            14 => Self::SharedDirectoryInfoResponse,
            15 => Self::SharedDirectoryCreateRequest,
            16 => Self::SharedDirectoryCreateResponse,
            17 => Self::SharedDirectoryDeleteRequest,
            18 => Self::SharedDirectoryDeleteResponse,
            19 => Self::SharedDirectoryReadRequest,
            20 => Self::SharedDirectoryReadResponse,
            21 => Self::SharedDirectoryWriteRequest,
            22 => Self::SharedDirectoryWriteResponse,
            23 => Self::SharedDirectoryMoveRequest,
            24 => Self::SharedDirectoryMoveResponse,
            25 => Self::SharedDirectoryListRequest,
            26 => Self::SharedDirectoryListResponse,
            27 => Self::Png2Frame,
            28 => Self::Alert,
            29 => Self::RdpFastPathPdu,
            30 => Self::RdpResponsePdu,
            31 => Self::RdpConnectionActivated,
            32 => Self::SyncKeys,
            33 => Self::SharedDirectoryTruncateRequest,
            34 => Self::SharedDirectoryTruncateResponse,
            35 => Self::LatencyStats,
            37 => Self::ClientKeyboardLayout,
            38 => Self::TdpbUpgrade,
            n => return Err(CodecError::UnknownTdpType(n)),
        })
    }
}

impl From<MessageType> for u8 {
    fn from(v: MessageType) -> Self {
        v as u8
    }
}
