//! Legacy TDP wire-format encoder. Tracks `TdpCodec.encode*` in
//! `web/packages/shared/libs/tdp/codec.ts`.
//!
//! Messages that legacy TDP doesn't carry (`Ping`, `MfaResponse`,
//! `ShareDirRemove`, `SessionSelection` — all TDPB-only) return an empty
//! `Vec` so the dispatch macro in `lib.rs` can stay variant-symmetric. The
//! caller is expected not to send these in TDP mode.

use crate::error::{CodecError, EncodeError};
use crate::messages::{
    ButtonState, ClientHello, ClipboardIn, Fso, KeyboardButton, MfaResponse, MouseButton,
    MouseMove, MouseWheel, Ping, RdpResponsePdu, RefreshRect, ScreenSpec, SessionSelection,
    ShareDirAnnounce, ShareDirRemove, ShareDirResponse, SyncKeys,
};
use crate::tdp::message_type::MessageType;

fn push_u8(buf: &mut Vec<u8>, v: u8) {
    buf.push(v);
}
fn push_i16(buf: &mut Vec<u8>, v: i16) {
    buf.extend_from_slice(&v.to_be_bytes());
}
fn push_u32(buf: &mut Vec<u8>, v: u32) {
    buf.extend_from_slice(&v.to_be_bytes());
}
fn push_u64(buf: &mut Vec<u8>, v: u64) {
    buf.extend_from_slice(&v.to_be_bytes());
}
fn push_type(buf: &mut Vec<u8>, ty: MessageType) {
    buf.push(u8::from(ty));
}

/// Narrows a buffer length to the wire-format `u32`, rejecting >4 GiB fields.
fn len_u32(field: &'static str, n: usize) -> Result<u32, CodecError> {
    u32::try_from(n).map_err(|_| CodecError::Encode(EncodeError::LengthOverflow { field, len: n }))
}

/// Writes `| u32 length | bytes |` for a UTF-8 string.
fn push_lp_str(buf: &mut Vec<u8>, field: &'static str, s: &str) -> Result<(), CodecError> {
    push_u32(buf, len_u32(field, s.len())?);
    buf.extend_from_slice(s.as_bytes());

    Ok(())
}

/// Writes `| u32 length | bytes |` for opaque bytes.
fn push_lp_bytes(buf: &mut Vec<u8>, field: &'static str, data: &[u8]) -> Result<(), CodecError> {
    push_u32(buf, len_u32(field, data.len())?);
    buf.extend_from_slice(data);

    Ok(())
}

pub fn screen_spec(m: &ScreenSpec) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(9);

    push_type(&mut buf, MessageType::ClientScreenSpec);
    push_u32(&mut buf, m.width);
    push_u32(&mut buf, m.height);

    Ok(buf)
}

/// TDP doesn't have a single hello frame; the proxy expects at least two
/// preamble messages before it activates. Emit `ClientUsername`, the screen
/// spec, then either the keyboard layout or a second screen spec to satisfy
/// the message-count requirement. Tracks `encodeInitialMessages` +
/// `encodeUsername` in the TS codec.
pub fn client_hello(m: &ClientHello) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::new();

    // | type=7 | username_length u32 | username []byte |
    push_type(&mut buf, MessageType::ClientUsername);
    push_lp_str(&mut buf, "ClientUsername.username", &m.username)?;

    buf.extend_from_slice(&screen_spec(&m.screen_spec)?);

    if m.keyboard_layout != 0 {
        // | type=37 | length=4 u32 | layout u32 |
        push_type(&mut buf, MessageType::ClientKeyboardLayout);
        push_u32(&mut buf, 4);
        push_u32(&mut buf, m.keyboard_layout);
    } else {
        // No layout to send — repeat the screen spec to satisfy the proxy's
        // two-message preamble count.
        buf.extend_from_slice(&screen_spec(&m.screen_spec)?);
    }

    Ok(buf)
}

/// TDPB-only message; sending one in TDP mode is a caller bug.
pub fn session_selection(_m: &SessionSelection) -> Result<Vec<u8>, CodecError> {
    Ok(Vec::new())
}

pub fn mouse_move(m: &MouseMove) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(9);

    push_type(&mut buf, MessageType::MouseMove);
    push_u32(&mut buf, m.x);
    push_u32(&mut buf, m.y);

    Ok(buf)
}

pub fn mouse_button(m: &MouseButton) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(3);

    push_type(&mut buf, MessageType::MouseButton);
    push_u8(&mut buf, u8::from(m.button));
    push_u8(&mut buf, u8::from(m.pressed));

    Ok(buf)
}

pub fn mouse_wheel(m: &MouseWheel) -> Result<Vec<u8>, CodecError> {
    // | type | axis u8 | delta i16 |
    let delta = i16::try_from(m.delta).map_err(|_| {
        CodecError::Encode(EncodeError::ValueOverflow {
            field: "MouseWheel.delta",
            value: i64::from(m.delta),
            target: "i16",
        })
    })?;
    let mut buf = Vec::with_capacity(4);

    push_type(&mut buf, MessageType::MouseWheelScroll);
    push_u8(&mut buf, u8::from(m.axis));
    push_i16(&mut buf, delta);

    Ok(buf)
}

pub fn keyboard_button(m: &KeyboardButton) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(6);

    push_type(&mut buf, MessageType::KeyboardButton);
    push_u32(&mut buf, m.key_code);
    push_u8(&mut buf, u8::from(m.pressed));

    Ok(buf)
}

pub fn sync_keys(m: &SyncKeys) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(5);

    push_type(&mut buf, MessageType::SyncKeys);

    let b = |s: ButtonState| u8::from(matches!(s, ButtonState::Down));

    push_u8(&mut buf, b(m.scroll_lock));
    push_u8(&mut buf, b(m.num_lock));
    push_u8(&mut buf, b(m.caps_lock));
    push_u8(&mut buf, b(m.kana_lock));

    Ok(buf)
}

pub fn rdp_response(m: &RdpResponsePdu) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(5 + m.response.len());

    push_type(&mut buf, MessageType::RdpResponsePdu);
    push_lp_bytes(&mut buf, "RdpResponsePdu.data", &m.response)?;

    Ok(buf)
}

/// Legacy TDP has no equivalent of the TDPB refresh-rect intent. Callers
/// must upgrade to TDPB before requesting a server-side refresh. We fail
/// loudly rather than silently dropping the request.
pub fn refresh_rect(_m: &RefreshRect) -> Result<Vec<u8>, CodecError> {
    Err(CodecError::Encode(EncodeError::UnsupportedInTdp("RefreshRect")))
}

pub fn clipboard(m: &ClipboardIn) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::with_capacity(5 + m.data.len());

    push_type(&mut buf, MessageType::ClipboardData);
    push_lp_str(&mut buf, "ClipboardData.data", &m.data)?;

    Ok(buf)
}

/// Legacy framed JSON: `| type=10 | mfa_type='n' | json_length u32 | json |`.
/// `mfa_type='n'` is the webauthn discriminator; U2F (`'u'`) is unsupported.
/// Tracks `TdpCodec.encodeMfaJson` (TS uses `JSON.stringify` on the same
/// [`MfaResponse`] shape).
pub fn mfa_response(m: &MfaResponse) -> Result<Vec<u8>, CodecError> {
    let json = serde_json::to_vec(m)
        .map_err(|e| CodecError::Encode(EncodeError::Json("MfaResponse", e.to_string())))?;
    let mut buf = Vec::with_capacity(1 + 1 + 4 + json.len());

    push_type(&mut buf, MessageType::MfaJson);
    push_u8(&mut buf, b'n');
    push_lp_bytes(&mut buf, "MfaJson.json", &json)?;

    Ok(buf)
}

/// TDPB-only; the client never pings the proxy in TDP mode.
pub fn ping(_m: &Ping) -> Result<Vec<u8>, CodecError> {
    Ok(Vec::new())
}

pub fn share_dir_announce(m: &ShareDirAnnounce) -> Result<Vec<u8>, CodecError> {
    // | type=11 | discard u32 | directory_id u32 | name_length u32 | name []byte |
    //
    // The leading u32 is dead — a TS bug (`encodeSharedDirectoryAnnounce` in
    // codec.ts:1430) that the proxy now expects. Always emit zero.
    let mut buf = Vec::with_capacity(1 + 12 + m.name.len());

    push_type(&mut buf, MessageType::SharedDirectoryAnnounce);
    push_u32(&mut buf, 0);
    push_u32(&mut buf, m.directory_id);
    push_lp_str(&mut buf, "SharedDirectoryAnnounce.name", &m.name)?;

    Ok(buf)
}

/// TDPB-only; the proxy never directs a TDP client to detach a share.
pub fn share_dir_remove(_m: &ShareDirRemove) -> Result<Vec<u8>, CodecError> {
    Ok(Vec::new())
}

fn push_fso(buf: &mut Vec<u8>, fso: &Fso) -> Result<(), CodecError> {
    // | last_modified u64 | size u64 | file_type u32 | is_empty u8 | path_length u32 | path []byte |
    push_u64(buf, fso.last_modified);
    push_u64(buf, fso.size);
    push_u32(buf, u32::from(fso.file_type));
    push_u8(buf, u8::from(fso.is_empty));
    push_lp_str(buf, "FileSystemObject.path", &fso.path)
}

pub fn share_dir_response(m: &ShareDirResponse) -> Result<Vec<u8>, CodecError> {
    let mut buf = Vec::new();
    match m {
        ShareDirResponse::Info(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryInfoResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
            push_fso(&mut buf, &r.fso)?;
        }
        ShareDirResponse::Create(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryCreateResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
            push_fso(&mut buf, &r.fso)?;
        }
        ShareDirResponse::Delete(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryDeleteResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
        }
        ShareDirResponse::List(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryListResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
            push_u32(
                &mut buf,
                len_u32("SharedDirectoryListResponse.fso_list", r.fsos.len())?,
            );

            for fso in &r.fsos {
                push_fso(&mut buf, fso)?;
            }
        }
        ShareDirResponse::Read(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryReadResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
            push_lp_bytes(&mut buf, "SharedDirectoryReadResponse.data", &r.data)?;
        }
        ShareDirResponse::Write(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryWriteResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
            push_u32(&mut buf, r.bytes_written);
        }
        ShareDirResponse::Move(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryMoveResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
        }
        ShareDirResponse::Truncate(r) => {
            push_type(&mut buf, MessageType::SharedDirectoryTruncateResponse);
            push_u32(&mut buf, r.completion_id);
            push_u32(&mut buf, u32::from(r.err));
        }
    }

    Ok(buf)
}
