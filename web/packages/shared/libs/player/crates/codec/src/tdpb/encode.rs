//! TDPB wire-format encoder. Each message is `[u32 BE length][Envelope proto]`,
//! matching the framing in `lib/srv/desktop/tdp/protocol/tdpb/tdpb.go`'s
//! `marshalWithHeader`.

use crate::error::{CodecError, EncodeError};
use crate::messages::{
    ClientHello, ClipboardIn, KeyboardButton, MfaResponse, MouseButton, MouseMove, MouseWheel,
    Ping, RdpResponsePdu, RefreshRect, ScreenSpec, SessionSelection, ShareDirAnnounce,
    ShareDirRemove, ShareDirResponse, SyncKeys,
};
use base64::Engine as _;
use teleport_proto::prost::Message as _;
use teleport_proto::teleport::desktop::v1 as proto;
use teleport_proto::teleport::desktop::v1::envelope::Payload;
use teleport_proto::teleport::mfa::v1 as mfa_proto;
use teleport_proto::webauthn;

fn marshal(payload: Payload) -> Result<Vec<u8>, CodecError> {
    let envelope = proto::Envelope {
        payload: Some(payload),
    };
    let body_len = envelope.encoded_len();
    let body_len_u32 = u32::try_from(body_len).map_err(|_| {
        CodecError::Encode(EncodeError::LengthOverflow {
            field: "Envelope.body",
            len: body_len,
        })
    })?;

    let mut buf = Vec::with_capacity(4 + body_len);

    buf.extend_from_slice(&body_len_u32.to_be_bytes());
    envelope
        .encode(&mut buf)
        .expect("prost encode into Vec<u8> is infallible");

    Ok(buf)
}

pub fn client_hello(m: &ClientHello) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::ClientHello(proto::ClientHello {
        username: m.username.clone(),
        screen_spec: Some((&m.screen_spec).into()),
        keyboard_layout: m.keyboard_layout,
    }))
}

pub fn session_selection(m: &SessionSelection) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::SessionSelection(proto::SessionSelection {
        session: Some(proto::SessionIdentifier {
            name: m.name.clone(),
        }),
    }))
}

pub fn screen_spec(m: &ScreenSpec) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::ClientScreenSpec(m.into()))
}

pub fn sync_keys(m: &SyncKeys) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::SyncKeys(m.into()))
}

pub fn mouse_move(m: &MouseMove) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::MouseMove(proto::MouseMove { x: m.x, y: m.y }))
}

pub fn mouse_button(m: &MouseButton) -> Result<Vec<u8>, CodecError> {
    use crate::messages::MouseButtonKind;

    let button = i32::from(match m.button {
        MouseButtonKind::Left => proto::MouseButtonType::Left,
        MouseButtonKind::Middle => proto::MouseButtonType::Middle,
        MouseButtonKind::Right => proto::MouseButtonType::Right,
    });

    marshal(Payload::MouseButton(proto::MouseButton {
        button,
        pressed: m.pressed,
    }))
}

pub fn mouse_wheel(m: &MouseWheel) -> Result<Vec<u8>, CodecError> {
    use crate::messages::ScrollAxis;

    let axis = i32::from(match m.axis {
        ScrollAxis::Vertical => proto::MouseWheelAxis::Vertical,
        ScrollAxis::Horizontal => proto::MouseWheelAxis::Horizontal,
    });

    marshal(Payload::MouseWheel(proto::MouseWheel {
        axis,
        delta: m.delta,
    }))
}

pub fn keyboard_button(m: &KeyboardButton) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::KeyboardButton(proto::KeyboardButton {
        key_code: m.key_code,
        pressed: m.pressed,
    }))
}

pub fn rdp_response(m: &RdpResponsePdu) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::RdpResponsePdu(proto::RdpResponsePdu {
        response: m.response.clone(),
    }))
}

pub fn refresh_rect(m: &RefreshRect) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::RefreshRect(proto::RefreshRect {
        left: m.left,
        top: m.top,
        right: m.right,
        bottom: m.bottom,
    }))
}

pub fn clipboard(m: &ClipboardIn) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::ClipboardData(proto::ClipboardData {
        data: bytes::Bytes::copy_from_slice(m.data.as_bytes()),
    }))
}

/// Packages an [`MfaResponse`] into a TDPB `Mfa` envelope carrying an
/// `AuthenticateResponse`. `WebAuthn` buffer fields are base64url-encoded
/// strings on the [`MfaResponse`] side and must be re-decoded to raw bytes
/// before they reach the proto. Tracks the structured branch of the TS
/// `encodeMfaJson`.
pub fn mfa_response(m: &MfaResponse) -> Result<Vec<u8>, CodecError> {
    let authentication_response = Some(mfa_response_to_proto(m)?);

    marshal(Payload::Mfa(Box::new(proto::Mfa {
        r#type: i32::from(proto::MfaType::Webauthn),
        channel_id: String::new(),
        challenge: None,
        authentication_response,
    })))
}

fn mfa_response_to_proto(m: &MfaResponse) -> Result<mfa_proto::AuthenticateResponse, CodecError> {
    use mfa_proto::authenticate_response::Response;

    let response = if let Some(w) = &m.webauthn_response {
        let b64u = base64::engine::general_purpose::URL_SAFE_NO_PAD;
        let decode = |field: &'static str, s: &str| -> Result<bytes::Bytes, CodecError> {
            b64u.decode(s.trim_end_matches('='))
                .map(bytes::Bytes::from)
                .map_err(|_| CodecError::Encode(EncodeError::Base64(field)))
        };
        Some(Response::Webauthn(webauthn::CredentialAssertionResponse {
            r#type: w.kind.clone(),
            raw_id: decode("MfaResponse.webauthn_response.rawId", &w.raw_id)?,
            response: Some(webauthn::AuthenticatorAssertionResponse {
                client_data_json: decode(
                    "MfaResponse.webauthn_response.response.clientDataJSON",
                    &w.response.client_data_json,
                )?,
                authenticator_data: decode(
                    "MfaResponse.webauthn_response.response.authenticatorData",
                    &w.response.authenticator_data,
                )?,
                signature: decode(
                    "MfaResponse.webauthn_response.response.signature",
                    &w.response.signature,
                )?,
                user_handle: decode(
                    "MfaResponse.webauthn_response.response.userHandle",
                    &w.response.user_handle,
                )?,
            }),
            extensions: Some(webauthn::AuthenticationExtensionsClientOutputs {
                app_id: w.extensions.appid,
                cred_props: None,
            }),
        }))
    } else if let Some(s) = &m.sso_response {
        Some(Response::Sso(mfa_proto::SsoChallengeResponse {
            request_id: s.request_id.clone(),
            token: s.token.clone(),
        }))
    } else {
        return Err(CodecError::Encode(EncodeError::MissingInput(
            "MfaResponse needs webauthn_response or sso_response",
        )));
    };

    Ok(mfa_proto::AuthenticateResponse {
        name: String::new(),
        response,
    })
}

pub fn ping(m: &Ping) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::Ping(proto::Ping {
        uuid: m.uuid.clone(),
    }))
}

pub fn share_dir_announce(m: &ShareDirAnnounce) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::SharedDirectoryAnnounce(
        proto::SharedDirectoryAnnounce {
            directory_id: m.directory_id,
            name: m.name.clone(),
        },
    ))
}

pub fn share_dir_remove(m: &ShareDirRemove) -> Result<Vec<u8>, CodecError> {
    marshal(Payload::SharedDirectoryRemove(
        proto::SharedDirectoryRemove {
            directory_id: m.directory_id,
        },
    ))
}

pub fn share_dir_response(m: &ShareDirResponse) -> Result<Vec<u8>, CodecError> {
    use proto::shared_directory_response as r;

    let (completion_id, error_code, operation) = match m {
        ShareDirResponse::Info(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Info(r::Info {
                fso: Some((&x.fso).into()),
            }),
        ),
        ShareDirResponse::Create(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Create(r::Create {
                fso: Some((&x.fso).into()),
            }),
        ),
        ShareDirResponse::Delete(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Delete(r::Delete {}),
        ),
        ShareDirResponse::List(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::List(r::List {
                fso_list: x.fsos.iter().map(Into::into).collect(),
            }),
        ),
        ShareDirResponse::Read(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Read(r::Read {
                data: x.data.clone(),
            }),
        ),
        ShareDirResponse::Write(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Write(r::Write {
                bytes_written: x.bytes_written,
            }),
        ),
        ShareDirResponse::Move(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Move(r::Move {}),
        ),
        ShareDirResponse::Truncate(x) => (
            x.completion_id,
            u32::from(x.err),
            r::Operation::Truncate(r::Truncate {}),
        ),
    };

    marshal(Payload::SharedDirectoryResponse(
        proto::SharedDirectoryResponse {
            completion_id,
            error_code,
            operation: Some(operation),
        },
    ))
}
