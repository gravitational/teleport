//! Legacy TDP wire-format decoder. Tracks `TdpCodec.decodeMessage` in
//! `web/packages/shared/libs/tdp/codec.ts` field-for-field.
//!
//! Known type bytes the TS client never decodes (client→server or
//! server-only) fall through to `InboundMessage::Unsupported`; bytes not in
//! [`MessageType`] surface as `CodecError::UnknownTdpType`. Wire layouts
//! appear above each helper in `| field type |` shorthand.

use crate::error::CodecError;
use crate::incoming::InboundMessage;
use crate::messages::{
    Alert, ClipboardIn, ConnectionActivated, FastPathPdu, FileType, LatencyStats, MfaChallenge,
    MfaKind, MouseButton, MouseButtonKind, MouseMove, Png2Frame, PngFrame, Rect, ScreenSpec,
    Severity, ShareDirAck, ShareDirRequest, SharedDirErrCode, Unsupported,
};
use crate::tdp::cursor::Cursor;
use crate::tdp::message_type::MessageType;
use bytes::Bytes;

pub fn decode(data: Bytes) -> Result<InboundMessage, CodecError> {
    let mut cur = Cursor::new(data);
    let ty = MessageType::try_from(cur.u8("message type")?)?;

    Ok(match ty {
        MessageType::PngFrame => InboundMessage::PngFrame(decode_png_frame(&mut cur)?),
        MessageType::Png2Frame => InboundMessage::Png2Frame(decode_png2_frame(&mut cur)?),
        MessageType::RdpConnectionActivated => {
            InboundMessage::ConnectionActivated(decode_connection_activated(&mut cur)?)
        }
        MessageType::RdpFastPathPdu => InboundMessage::FastPathPdu(decode_fast_path_pdu(&mut cur)?),
        MessageType::ClipboardData => InboundMessage::ClipboardIn(decode_clipboard(&mut cur)?),
        MessageType::Error => InboundMessage::Alert(Alert {
            severity: Severity::Error,
            message: decode_string(&mut cur)?,
        }),
        MessageType::Alert => InboundMessage::Alert(decode_alert(&mut cur)?),
        MessageType::MfaJson => InboundMessage::MfaChallenge(Box::new(decode_mfa_json(&mut cur)?)),

        MessageType::ClientScreenSpec => {
            InboundMessage::ScreenSpec(decode_client_screen_spec(&mut cur)?)
        }
        MessageType::MouseButton => InboundMessage::MouseButton(decode_mouse_button(&mut cur)?),
        MessageType::MouseMove => InboundMessage::MouseMove(decode_mouse_move(&mut cur)?),

        MessageType::LatencyStats => InboundMessage::LatencyStats(decode_latency_stats(&mut cur)?),
        MessageType::TdpbUpgrade => InboundMessage::TdpbUpgrade,

        MessageType::SharedDirectoryAcknowledge => {
            InboundMessage::ShareDirAck(decode_share_dir_ack(&mut cur)?)
        }
        MessageType::SharedDirectoryInfoRequest => InboundMessage::ShareDirRequest(
            decode_share_dir_path_request(&mut cur, ShareDirRequestKind::Info)?,
        ),
        MessageType::SharedDirectoryCreateRequest => {
            InboundMessage::ShareDirRequest(decode_share_dir_create(&mut cur)?)
        }
        MessageType::SharedDirectoryDeleteRequest => InboundMessage::ShareDirRequest(
            decode_share_dir_path_request(&mut cur, ShareDirRequestKind::Delete)?,
        ),
        MessageType::SharedDirectoryListRequest => InboundMessage::ShareDirRequest(
            decode_share_dir_path_request(&mut cur, ShareDirRequestKind::List)?,
        ),
        MessageType::SharedDirectoryReadRequest => {
            InboundMessage::ShareDirRequest(decode_share_dir_read(&mut cur)?)
        }
        MessageType::SharedDirectoryWriteRequest => {
            InboundMessage::ShareDirRequest(decode_share_dir_write(&mut cur)?)
        }
        MessageType::SharedDirectoryMoveRequest => {
            InboundMessage::ShareDirRequest(decode_share_dir_move(&mut cur)?)
        }
        MessageType::SharedDirectoryTruncateRequest => {
            InboundMessage::ShareDirRequest(decode_share_dir_truncate(&mut cur)?)
        }

        // Client→server bytes (and a couple of server-only ones) that the
        // TS decoder ignores. They turn up in recordings; pass them through
        // as `Unsupported` and let the caller filter.
        MessageType::KeyboardButton
        | MessageType::ClientUsername
        | MessageType::MouseWheelScroll
        | MessageType::SharedDirectoryAnnounce
        | MessageType::SharedDirectoryInfoResponse
        | MessageType::SharedDirectoryCreateResponse
        | MessageType::SharedDirectoryDeleteResponse
        | MessageType::SharedDirectoryReadResponse
        | MessageType::SharedDirectoryWriteResponse
        | MessageType::SharedDirectoryMoveResponse
        | MessageType::SharedDirectoryListResponse
        | MessageType::SharedDirectoryTruncateResponse
        | MessageType::RdpResponsePdu
        | MessageType::SyncKeys
        | MessageType::ClientKeyboardLayout => {
            InboundMessage::Unsupported(Unsupported {
                tdp_type: u8::from(ty),
            })
        }
    })
}

fn decode_rect(cur: &mut Cursor) -> Result<Rect, CodecError> {
    Ok(Rect {
        left: cur.u32("PngFrame.left")?,
        top: cur.u32("PngFrame.top")?,
        right: cur.u32("PngFrame.right")?,
        bottom: cur.u32("PngFrame.bottom")?,
    })
}

// | type | left u32 | top u32 | right u32 | bottom u32 | png []byte |
fn decode_png_frame(cur: &mut Cursor) -> Result<PngFrame, CodecError> {
    let rect = decode_rect(cur)?;
    let png = cur.bytes_owned(cur.remaining(), "PngFrame.png")?;

    Ok(PngFrame { rect, png })
}

// | type | png_length u32 | left u32 | top u32 | right u32 | bottom u32 | png []byte |
//
// `png_length` is redundant given the surrounding length-prefixed framing,
// but the legacy wire still carries it — read and discard.
fn decode_png2_frame(cur: &mut Cursor) -> Result<Png2Frame, CodecError> {
    let _ = cur.u32("Png2Frame.png_length")?;
    let rect = decode_rect(cur)?;
    let png = cur.bytes_owned(cur.remaining(), "Png2Frame.png")?;

    Ok(Png2Frame { rect, png })
}

// | type | io_channel_id u16 | user_channel_id u16 | screen_width u16 | screen_height u16 |
fn decode_connection_activated(cur: &mut Cursor) -> Result<ConnectionActivated, CodecError> {
    Ok(ConnectionActivated {
        io_channel_id: cur.u16("ConnectionActivated.io_channel_id")?,
        user_channel_id: cur.u16("ConnectionActivated.user_channel_id")?,
        screen_width: cur.u16("ConnectionActivated.screen_width")?,
        screen_height: cur.u16("ConnectionActivated.screen_height")?,
    })
}

// | type | data_length u32 | data []byte |
fn decode_fast_path_pdu(cur: &mut Cursor) -> Result<FastPathPdu, CodecError> {
    let len = cur.u32("FastPathPdu.data_length")? as usize;

    Ok(FastPathPdu {
        pdu: cur.bytes_owned(len, "FastPathPdu.data")?,
    })
}

// | type | length u32 | data []byte |
fn decode_clipboard(cur: &mut Cursor) -> Result<ClipboardIn, CodecError> {
    Ok(ClipboardIn {
        data: cur.string("ClipboardData")?,
    })
}

/// Body of a TDP `string-message`: `| length u32 | bytes |`. Used by ERROR
/// and inside ALERT/MFA after each message's preamble.
fn decode_string(cur: &mut Cursor) -> Result<String, CodecError> {
    cur.string("string message")
}

// | type | message_length u32 | message []byte | severity u8 |
fn decode_alert(cur: &mut Cursor) -> Result<Alert, CodecError> {
    let message = cur.string("Alert.message")?;
    let severity = Severity::try_from(cur.u8("Alert.severity")?)?;

    Ok(Alert { severity, message })
}

// | type | mfa_type u8 | json_length u32 | json []byte |
fn decode_mfa_json(cur: &mut Cursor) -> Result<MfaChallenge, CodecError> {
    let kind = MfaKind::try_from(cur.u8("MfaJson.mfa_type")?)?;
    let len = cur.u32("MfaJson.message_length")? as usize;
    let bytes = cur.bytes(len, "MfaJson.json")?;
    let challenge: crate::messages::MfaChallengeJson =
        serde_json::from_slice(bytes).map_err(|e| CodecError::MalformedJson {
            field: "MfaJson.json",
            message: e.to_string(),
        })?;

    Ok(MfaChallenge { kind, challenge })
}

// | type | client_latency u32 | server_latency u32 |
fn decode_latency_stats(cur: &mut Cursor) -> Result<LatencyStats, CodecError> {
    Ok(LatencyStats {
        client_ms: cur.u32("LatencyStats.client_latency_ms")?,
        server_ms: cur.u32("LatencyStats.server_latency_ms")?,
    })
}

// | type | width u32 | height u32 |
fn decode_client_screen_spec(cur: &mut Cursor) -> Result<ScreenSpec, CodecError> {
    Ok(ScreenSpec {
        width: cur.u32("ClientScreenSpec.width")?,
        height: cur.u32("ClientScreenSpec.height")?,
        // Legacy TDP predates HiDPI; assume 1× until TDPB negotiates otherwise.
        scale: 100,
        // Legacy TDP predates multi-monitor; the proxy synthesizes a primary
        // from width/height when the vec is empty.
        monitors: Vec::new(),
    })
}

// | type | button u8 | state u8 |
fn decode_mouse_button(cur: &mut Cursor) -> Result<MouseButton, CodecError> {
    Ok(MouseButton {
        button: MouseButtonKind::try_from(cur.u8("MouseButton.button")?)?,
        pressed: cur.u8("MouseButton.state")? != 0,
    })
}

// | type | x u32 | y u32 |
//
// The TS `decodeMouseMove` reads x/y as uint8 — a long-standing bug that
// contradicts the format comment and the matching encoder. We follow the
// wire (uint32), which is what the proxy emits.
fn decode_mouse_move(cur: &mut Cursor) -> Result<MouseMove, CodecError> {
    Ok(MouseMove {
        x: cur.u32("MouseMove.x")?,
        y: cur.u32("MouseMove.y")?,
    })
}

// | type | err_code u32 | directory_id u32 |
fn decode_share_dir_ack(cur: &mut Cursor) -> Result<ShareDirAck, CodecError> {
    let err = SharedDirErrCode::try_from(cur.u32("ShareDirAck.err_code")?)?;
    let directory_id = cur.u32("ShareDirAck.directory_id")?;

    Ok(ShareDirAck { err, directory_id })
}

#[derive(Clone, Copy)]
enum ShareDirRequestKind {
    Info,
    Delete,
    List,
}

// | type | completion_id u32 | directory_id u32 | path_length u32 | path []byte |
//
// Info/Delete/List share this body; `kind` picks which variant to build.
fn decode_share_dir_path_request(
    cur: &mut Cursor,
    kind: ShareDirRequestKind,
) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("completion_id")?;
    let directory_id = cur.u32("directory_id")?;
    let path = cur.string("path")?;

    Ok(match kind {
        ShareDirRequestKind::Info => ShareDirRequest::Info {
            completion_id,
            directory_id,
            path,
        },
        ShareDirRequestKind::Delete => ShareDirRequest::Delete {
            completion_id,
            directory_id,
            path,
        },
        ShareDirRequestKind::List => ShareDirRequest::List {
            completion_id,
            directory_id,
            path,
        },
    })
}

// | type | completion_id u32 | directory_id u32 | file_type u32 | path_length u32 | path []byte |
fn decode_share_dir_create(cur: &mut Cursor) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("Create.completion_id")?;
    let directory_id = cur.u32("Create.directory_id")?;
    let file_type = FileType::try_from(cur.u32("Create.file_type")?)?;
    let path = cur.string("Create.path")?;

    Ok(ShareDirRequest::Create {
        completion_id,
        directory_id,
        file_type,
        path,
    })
}

// | type | completion_id u32 | directory_id u32 | path_length u32 | path []byte | offset u64 | length u32 |
fn decode_share_dir_read(cur: &mut Cursor) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("Read.completion_id")?;
    let directory_id = cur.u32("Read.directory_id")?;
    let path = cur.string("Read.path")?;
    let offset = cur.u64("Read.offset")?;
    let length = cur.u32("Read.length")?;

    Ok(ShareDirRequest::Read {
        completion_id,
        directory_id,
        path,
        offset,
        length,
    })
}

// | type | completion_id u32 | directory_id u32 | offset u64 | path_length u32 | path []byte | write_data_length u32 | write_data []byte |
//
// The TS format comment lists `path_length` before `offset`, but the TS
// decoder (and the proxy) reads `offset` first. The decoder is canonical.
fn decode_share_dir_write(cur: &mut Cursor) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("Write.completion_id")?;
    let directory_id = cur.u32("Write.directory_id")?;
    let offset = cur.u64("Write.offset")?;
    let path = cur.string("Write.path")?;
    let data_len = cur.u32("Write.write_data_length")? as usize;
    let data = cur.bytes_owned(data_len, "Write.write_data")?;

    Ok(ShareDirRequest::Write {
        completion_id,
        directory_id,
        path,
        offset,
        data,
    })
}

// | type | completion_id u32 | directory_id u32 | original_path_length u32 | original_path []byte | new_path_length u32 | new_path []byte |
fn decode_share_dir_move(cur: &mut Cursor) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("Move.completion_id")?;
    let directory_id = cur.u32("Move.directory_id")?;
    let original = cur.string("Move.original_path")?;
    let new = cur.string("Move.new_path")?;

    Ok(ShareDirRequest::Move {
        completion_id,
        directory_id,
        original,
        new,
    })
}

// | type | completion_id u32 | directory_id u32 | path_length u32 | path []byte | end_of_file u32 |
fn decode_share_dir_truncate(cur: &mut Cursor) -> Result<ShareDirRequest, CodecError> {
    let completion_id = cur.u32("Truncate.completion_id")?;
    let directory_id = cur.u32("Truncate.directory_id")?;
    let path = cur.string("Truncate.path")?;
    let end_of_file = cur.u32("Truncate.end_of_file")?;

    Ok(ShareDirRequest::Truncate {
        completion_id,
        directory_id,
        path,
        size: u64::from(end_of_file),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::messages::{MouseWheel, ScrollAxis, ShareDirAnnounce};
    use crate::tdp::encode;

    fn ok(b: &[u8]) -> InboundMessage {
        decode(Bytes::copy_from_slice(b)).expect("decode")
    }

    // ── Wire-format regressions ────────────────────────────────────────────

    #[test]
    fn mouse_move_is_uint32_not_uint8() {
        // Guards against the TS `decodeMouseMove` bug that read x/y as u8.
        let bytes = [3, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08];
        let InboundMessage::MouseMove(m) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(m.x, 0x0102_0304);
        assert_eq!(m.y, 0x0506_0708);
    }

    #[test]
    fn rdp_connection_activated_is_uint16() {
        // Channel ids are narrower on the wire than in the proto; if the
        // decoder ever switches to u32 reads this assertion catches it.
        let bytes = [31, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08];
        let InboundMessage::ConnectionActivated(c) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(c.io_channel_id, 0x0102);
        assert_eq!(c.user_channel_id, 0x0304);
        assert_eq!(c.screen_width, 0x0506);
        assert_eq!(c.screen_height, 0x0708);
    }

    #[test]
    fn png2_frame_ignores_redundant_png_length() {
        // Wire: | type=27 | png_length=DEADBEEF | rect | png='OK' |.
        // Decoder must consume the body to EOF and ignore png_length so an
        // optimisation that trusts the prefix wouldn't go undetected.
        let mut bytes = vec![27];
        bytes.extend_from_slice(&0xDEAD_BEEFu32.to_be_bytes());
        bytes.extend_from_slice(&0u32.to_be_bytes()); // rect.left
        bytes.extend_from_slice(&0u32.to_be_bytes()); // rect.top
        bytes.extend_from_slice(&1u32.to_be_bytes()); // rect.right
        bytes.extend_from_slice(&1u32.to_be_bytes()); // rect.bottom
        bytes.extend_from_slice(b"OK");
        let InboundMessage::Png2Frame(f) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(&f.png[..], b"OK");
        assert_eq!(f.rect.right, 1);
    }

    #[test]
    fn fast_path_pdu_respects_data_length() {
        // Unlike PngFrame, FastPathPdu uses a length-prefixed payload; trailing
        // bytes past `data_length` must be ignored, not folded into the PDU.
        let mut bytes = vec![29];
        bytes.extend_from_slice(&3u32.to_be_bytes());
        bytes.extend_from_slice(b"PDUtail-should-not-appear");
        let InboundMessage::FastPathPdu(p) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(&p.pdu[..], b"PDU");
    }

    #[test]
    fn png_frame_payload_is_remainder() {
        let mut bytes = vec![2];
        bytes.extend_from_slice(&10u32.to_be_bytes()); // rect.left
        bytes.extend_from_slice(&20u32.to_be_bytes()); // rect.top
        bytes.extend_from_slice(&30u32.to_be_bytes()); // rect.right
        bytes.extend_from_slice(&40u32.to_be_bytes()); // rect.bottom
        bytes.extend_from_slice(b"PNGDATA");
        let InboundMessage::PngFrame(f) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(f.rect.left, 10);
        assert_eq!(f.rect.bottom, 40);
        assert_eq!(&f.png[..], b"PNGDATA");
    }

    // ── Variant routing ───────────────────────────────────────────────────

    #[test]
    fn alert_carries_message_then_severity() {
        // The severity byte trails the length-prefixed message; verifies the
        // cursor is positioned past the string before reading the discriminator.
        let mut bytes = vec![28, 0, 0, 0, 2, b'h', b'i', 1];
        let InboundMessage::Alert(a) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(a.message, "hi");
        assert_eq!(a.severity, Severity::Warning);
        bytes[7] = 2;
        let InboundMessage::Alert(a) = ok(&bytes) else {
            unreachable!()
        };
        assert_eq!(a.severity, Severity::Error);
    }

    #[test]
    fn error_byte_becomes_error_alert() {
        // The `Error` type byte (9) carries no severity; the decoder synthesises
        // `Severity::Error` and reuses the Alert variant.
        let mut bytes = vec![9, 0, 0, 0, 3];
        bytes.extend_from_slice(b"boo");
        let InboundMessage::Alert(a) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(a.severity, Severity::Error);
        assert_eq!(a.message, "boo");
    }

    #[test]
    fn clipboard_decodes_length_prefixed_utf8() {
        let mut bytes = vec![6, 0, 0, 0, 5];
        bytes.extend_from_slice(b"hello");
        let InboundMessage::ClipboardIn(c) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(c.data, "hello");
    }

    #[test]
    fn mfa_json_decodes_into_typed_challenge() {
        let json = br#"{"sso_challenge":{"channelId":"c","redirectUrl":"http://r","requestId":"r","device":{"connectorId":"id","connectorType":"t","displayName":"d"}}}"#;
        let mut bytes = vec![10, b'n'];
        let json_len = u32::try_from(json.len()).expect("test fixture fits in u32");
        bytes.extend_from_slice(&json_len.to_be_bytes());
        bytes.extend_from_slice(json);
        let InboundMessage::MfaChallenge(c) = ok(&bytes) else {
            panic!("wrong variant")
        };
        assert_eq!(c.kind, MfaKind::WebAuthn);
        let sso = c.challenge.sso_challenge.as_ref().expect("sso_challenge");
        assert_eq!(sso.channel_id, "c");
        assert_eq!(sso.device.connector_id, "id");
    }

    // ── Round-trips ───────────────────────────────────────────────────────

    #[test]
    fn mouse_move_roundtrip() {
        let original = MouseMove { x: 42, y: 1337 };
        let encoded = encode::mouse_move(&original).expect("encode");
        let InboundMessage::MouseMove(decoded) = ok(&encoded) else {
            panic!("wrong variant")
        };
        assert_eq!(decoded.x, original.x);
        assert_eq!(decoded.y, original.y);
    }

    #[test]
    fn mouse_button_roundtrip_each_variant() {
        for &(button, pressed) in &[
            (MouseButtonKind::Left, true),
            (MouseButtonKind::Middle, false),
            (MouseButtonKind::Right, true),
        ] {
            let original = MouseButton { button, pressed };
            let encoded = encode::mouse_button(&original).expect("encode");
            let InboundMessage::MouseButton(decoded) = ok(&encoded) else {
                panic!("wrong variant")
            };
            assert_eq!(decoded.button, button);
            assert_eq!(decoded.pressed, pressed);
        }
    }

    #[test]
    fn share_dir_announce_roundtrip_skips_legacy_discard_field() {
        // The encoder injects a leading zero u32 to match the TS proxy quirk.
        // Round-tripping confirms our decoder consumes the discard cleanly.
        let original = ShareDirAnnounce {
            directory_id: 0xCAFE_BABE,
            name: "share".to_owned(),
        };
        let encoded = encode::share_dir_announce(&original).expect("encode");
        let InboundMessage::Unsupported(u) = ok(&encoded) else {
            panic!("client-bound announce is server-only — should surface as Unsupported")
        };
        assert_eq!(u.tdp_type, u8::from(MessageType::SharedDirectoryAnnounce));
    }

    #[test]
    fn mouse_wheel_delta_clamps_to_i16() {
        // Wire slot is i16; an out-of-range delta should fail to encode rather
        // than truncate silently.
        let too_big = MouseWheel {
            axis: ScrollAxis::Vertical,
            delta: i32::from(i16::MAX) + 1,
        };
        let err = encode::mouse_wheel(&too_big).expect_err("must fail");
        assert!(matches!(
            err,
            CodecError::Encode(crate::error::EncodeError::ValueOverflow { target: "i16", .. })
        ));
    }

    // ── Error paths ───────────────────────────────────────────────────────

    #[test]
    fn unknown_type_byte_errors() {
        assert!(matches!(
            decode(Bytes::copy_from_slice(&[99])),
            Err(CodecError::UnknownTdpType(99))
        ));
    }

    #[test]
    fn keyboard_button_routes_to_unsupported() {
        // The TS client never decodes its own outbound types; in a recording
        // they show up server-side and the decoder must not return an error.
        let InboundMessage::Unsupported(u) = ok(&[5, 0, 0, 0, 0x1c, 1]) else {
            panic!("wrong variant")
        };
        assert_eq!(u.tdp_type, 5);
    }

    #[test]
    fn truncated_alert_surfaces_truncated_error() {
        // Decoder must distinguish "ran out of bytes" from "malformed".
        assert!(matches!(
            decode(Bytes::copy_from_slice(&[28])),
            Err(CodecError::Truncated(_))
        ));
    }
}
