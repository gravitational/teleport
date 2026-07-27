use crate::error::CodecError;
use crate::incoming::InboundMessage;
use crate::messages::{
    AllowedCredential, ExtensionsInput, MfaChallenge, MfaChallengeJson, MfaKind, PublicKeyRequest,
    RefreshRect, SsoChallenge, SsoDevice, WebauthnChallenge,
};
use base64::Engine as _;
use bytes::Bytes;
use teleport_proto::prost::Message as _;
use teleport_proto::teleport::desktop::v1 as proto;
use teleport_proto::teleport::desktop::v1::envelope::Payload;
use teleport_proto::teleport::desktop::v1::Envelope;
use teleport_proto::teleport::mfa::v1 as mfa_proto;

pub fn decode(mut data: Bytes) -> Result<InboundMessage, CodecError> {
    // TDPB framing is `[u32 BE body length][Envelope proto bytes]`. WS already
    // delimits frames so the length is redundant, but the Go server still
    // writes it (see `marshalWithHeader`) and the TS client still strips it
    // — match that here. `Envelope::decode` takes `impl Buf`; passing `Bytes`
    // lets prost's bytes-mode fields come out as refcounted slices of `data`
    // rather than fresh allocations.
    if data.len() < 4 {
        return Err(CodecError::Missing("Envelope length prefix"));
    }
    let _len = u32::from_be_bytes([data[0], data[1], data[2], data[3]]);
    let _ = data.split_to(4);

    let envelope = Envelope::decode(data)?;
    let payload = envelope
        .payload
        .ok_or(CodecError::Missing("Envelope.payload"))?;

    Ok(match payload {
        Payload::ClientHello(p) => InboundMessage::ClientHello(p.try_into()?),
        Payload::ServerHello(p) => InboundMessage::ServerHello(p.try_into()?),
        Payload::SessionSelection(p) => InboundMessage::SessionSelection(p.try_into()?),

        Payload::PngFrame(p) => InboundMessage::PngFrame(p.try_into()?),
        Payload::FastPathPdu(p) => InboundMessage::FastPathPdu(p.into()),
        Payload::EgfxBitmap(p) => InboundMessage::EgfxBitmap(p.into()),
        Payload::EgfxAvcFrame(p) => InboundMessage::EgfxAvcFrame(p.into()),
        Payload::EgfxClearCodec(p) => InboundMessage::EgfxClearCodec(p.into()),
        Payload::EgfxUncompressed(p) => InboundMessage::EgfxUncompressed(p.into()),
        Payload::EgfxPlanar(p) => InboundMessage::EgfxPlanar(p.into()),
        Payload::EgfxAvc420(p) => InboundMessage::EgfxAvc420(p.into()),
        Payload::EgfxSolidFill(p) => InboundMessage::EgfxSolidFill(p.into()),
        Payload::EgfxSurfaceToCache(p) => InboundMessage::EgfxSurfaceToCache(p.into()),
        Payload::EgfxCacheToSurface(p) => InboundMessage::EgfxCacheToSurface(p.into()),
        Payload::EgfxEvictCacheEntry(p) => InboundMessage::EgfxEvictCacheEntry(p.into()),
        Payload::EgfxSurfaceToSurface(p) => InboundMessage::EgfxSurfaceToSurface(p.into()),
        Payload::EgfxWireToSurface2(p) => InboundMessage::EgfxWireToSurface2(p.into()),
        Payload::EgfxDeleteEncodingContext(p) => {
            InboundMessage::EgfxDeleteEncodingContext(p.into())
        }
        Payload::EgfxEndFrame(p) => InboundMessage::EgfxEndFrame(p.into()),
        Payload::RdpResponsePdu(p) => InboundMessage::RdpResponsePdu(p.into()),

        Payload::Alert(p) => InboundMessage::Alert(p.try_into()?),
        Payload::ClipboardData(p) => InboundMessage::ClipboardIn(p.try_into()?),
        Payload::LatencyStats(p) => InboundMessage::LatencyStats(p.into()),
        Payload::Mfa(p) => InboundMessage::MfaChallenge(Box::new(decode_mfa(*p)?)),
        Payload::Ping(p) => InboundMessage::Ping(p.into()),
        Payload::ClientScreenSpec(p) => InboundMessage::ScreenSpec(p.into()),

        Payload::KeyboardButton(p) => InboundMessage::KeyboardButton(p.into()),
        Payload::MouseButton(p) => InboundMessage::MouseButton(p.try_into()?),
        Payload::MouseMove(p) => InboundMessage::MouseMove(p.into()),
        Payload::MouseWheel(p) => InboundMessage::MouseWheel(p.try_into()?),
        Payload::SyncKeys(p) => InboundMessage::SyncKeys(p.into()),

        Payload::SharedDirectoryAnnounce(p) => InboundMessage::ShareDirAnnounce(p.into()),
        Payload::SharedDirectoryAcknowledge(p) => InboundMessage::ShareDirAck(p.try_into()?),
        Payload::SharedDirectoryRequest(p) => InboundMessage::ShareDirRequest(p.try_into()?),
        Payload::SharedDirectoryResponse(p) => InboundMessage::ShareDirResponse(p.try_into()?),
        Payload::SharedDirectoryRemove(p) => InboundMessage::ShareDirRemove(p.into()),
        Payload::RefreshRect(p) => InboundMessage::RefreshRect(RefreshRect {
            left: p.left,
            top: p.top,
            right: p.right,
            bottom: p.bottom,
        }),
    })
}

/// Bridges the TDPB `Mfa` proto into the same [`MfaChallenge`] consumers
/// already get from legacy TDP. Tracks `toMfaWebauthnChallenge` and
/// `toMfaSsoChallenge` in `codec.ts`; opaque buffers are base64-encoded with
/// the standard alphabet to match the TS `btoa` output verbatim.
fn decode_mfa(p: proto::Mfa) -> Result<MfaChallenge, CodecError> {
    // The server's `NewTDPBMFAPrompt` populates the challenge fields but
    // leaves `type` at its proto3 default (`MFA_TYPE_UNSPECIFIED`); treat
    // that as the historical WebAuthn default rather than rejecting it. A
    // non-zero `type` that doesn't map to a known variant is still an error.
    let kind = if p.r#type == i32::from(proto::MfaType::Unspecified) {
        MfaKind::WebAuthn
    } else {
        MfaKind::try_from(p.r#type)?
    };
    let challenge_proto = p.challenge.ok_or(CodecError::Missing("Mfa.challenge"))?;

    let mut out = MfaChallengeJson::default();
    if let Some(w) = challenge_proto.webauthn_challenge {
        out.webauthn_challenge = Some(webauthn_challenge_from_proto(w)?);
    }
    if let Some(s) = challenge_proto.sso_challenge {
        out.sso_challenge = Some(sso_challenge_from_proto(s, p.channel_id)?);
    }

    Ok(MfaChallenge {
        kind,
        challenge: out,
    })
}

fn webauthn_challenge_from_proto(
    w: teleport_proto::webauthn::CredentialAssertion,
) -> Result<WebauthnChallenge, CodecError> {
    let public_key = w
        .public_key
        .ok_or(CodecError::Missing("webauthn.Challenge.public_key"))?;
    let extensions = public_key
        .extensions
        .map(|e| ExtensionsInput {
            app_id: e.app_id,
            cred_props: e.cred_props,
        })
        .ok_or(CodecError::Missing(
            "webauthn.Challenge.public_key.extensions",
        ))?;

    let b64 = base64::engine::general_purpose::STANDARD;

    Ok(WebauthnChallenge {
        public_key: PublicKeyRequest {
            challenge: b64.encode(&public_key.challenge),
            rp_id: public_key.rp_id,
            timeout: public_key.timeout_ms,
            user_verification: public_key.user_verification,
            extensions,
            allow_credentials: public_key
                .allow_credentials
                .into_iter()
                .map(|c| AllowedCredential {
                    id: b64.encode(&c.id),
                    kind: c.r#type,
                })
                .collect(),
        },
    })
}

fn sso_challenge_from_proto(
    s: mfa_proto::SsoChallenge,
    channel_id: String,
) -> Result<SsoChallenge, CodecError> {
    let device = s
        .device
        .ok_or(CodecError::Missing("sso_challenge.device"))?;

    Ok(SsoChallenge {
        // TS-compat quirk: the JSON's `channelId` is sourced from the
        // envelope, not from `SsoChallenge.request_id`. Don't switch these.
        channel_id,
        redirect_url: s.redirect_url,
        request_id: s.request_id,
        device: SsoDevice {
            connector_id: device.connector_id,
            connector_type: device.connector_type,
            display_name: device.display_name,
        },
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::messages::{
        ClipboardIn, MouseButton, MouseButtonKind, MouseMove, RdpResponsePdu, ScreenSpec,
    };
    use crate::tdpb::encode;

    /// Wraps a prost-encoded Envelope body in the 4-byte BE length prefix
    /// the decoder expects on the wire.
    fn frame(body: Vec<u8>) -> Bytes {
        let mut out = Vec::with_capacity(4 + body.len());
        out.extend_from_slice(&u32::try_from(body.len()).unwrap().to_be_bytes());
        out.extend_from_slice(&body);
        Bytes::from(out)
    }

    #[test]
    fn mouse_move_roundtrip() {
        let original = MouseMove { x: 100, y: 200 };
        let framed = encode::mouse_move(&original).expect("encode");
        let InboundMessage::MouseMove(decoded) = decode(Bytes::from(framed)).unwrap() else {
            panic!("wrong variant");
        };
        assert_eq!(decoded.x, 100);
        assert_eq!(decoded.y, 200);
    }

    #[test]
    fn mouse_button_enum_mapping_is_lossless() {
        // The encoder pivots on an explicit match between Rust and proto
        // enums; this test would have caught the old `+1` offset bug.
        for &kind in &[
            MouseButtonKind::Left,
            MouseButtonKind::Middle,
            MouseButtonKind::Right,
        ] {
            let framed = encode::mouse_button(&MouseButton {
                button: kind,
                pressed: true,
            })
            .expect("encode");
            let InboundMessage::MouseButton(d) = decode(Bytes::from(framed)).unwrap() else {
                panic!("wrong variant");
            };
            assert_eq!(d.button, kind);
            assert!(d.pressed);
        }
    }

    #[test]
    fn clipboard_string_roundtrip() {
        let original = ClipboardIn {
            data: "héllo".to_owned(),
        };
        let framed = encode::clipboard(&original).expect("encode");
        let InboundMessage::ClipboardIn(d) = decode(Bytes::from(framed)).unwrap() else {
            panic!("wrong variant");
        };
        assert_eq!(d.data, original.data);
    }

    #[test]
    fn rdp_response_pdu_uses_prost_bytes_mode() {
        // With prost's bytes mode the payload should round-trip through
        // `Bytes` without copying via Vec. The assertion is correctness; the
        // performance property is documented in the decoder.
        let original = RdpResponsePdu {
            response: Bytes::from_static(b"\x00\x01\x02opaque"),
        };
        let framed = encode::rdp_response(&original).expect("encode");
        let InboundMessage::RdpResponsePdu(d) = decode(Bytes::from(framed)).unwrap() else {
            panic!("wrong variant");
        };
        assert_eq!(d.response, original.response);
    }

    #[test]
    fn screen_spec_carries_scale() {
        // TDPB added `scale` after `width`/`height`; round-trip the new field.
        let original = ScreenSpec {
            width: 1920,
            height: 1080,
            scale: 200,
            monitors: Vec::new(),
        };
        let framed = encode::screen_spec(&original).expect("encode");
        let InboundMessage::ScreenSpec(d) = decode(Bytes::from(framed)).unwrap() else {
            panic!("wrong variant");
        };
        assert_eq!(d.width, 1920);
        assert_eq!(d.height, 1080);
        assert_eq!(d.scale, 200);
    }

    #[test]
    fn empty_envelope_payload_errors() {
        // A serialized Envelope with no payload must surface as Missing,
        // not panic, and not be silently treated as some default variant.
        let empty = teleport_proto::teleport::desktop::v1::Envelope { payload: None };
        let mut buf = Vec::new();
        teleport_proto::prost::Message::encode(&empty, &mut buf).unwrap();
        assert!(matches!(
            decode(frame(buf)),
            Err(CodecError::Missing("Envelope.payload"))
        ));
    }

    #[test]
    fn missing_length_prefix_errors() {
        // Fewer than 4 bytes can't carry a length header; must error cleanly.
        assert!(matches!(
            decode(Bytes::from_static(&[0x01, 0x02])),
            Err(CodecError::Missing("Envelope length prefix"))
        ));
    }

    #[test]
    fn garbage_body_surfaces_as_decode_error() {
        // 4-byte length prefix followed by bytes that don't parse as an
        // Envelope must be a clean decode error.
        assert!(matches!(
            decode(frame(vec![0xFF, 0xFF, 0xFF, 0xFF])),
            Err(CodecError::Decode(_))
        ));
    }

    #[test]
    fn mfa_with_unspecified_type_decodes_as_webauthn() {
        // The server's `NewTDPBMFAPrompt` populates the challenge but leaves
        // `Mfa.type` at the proto3 default (`MFA_TYPE_UNSPECIFIED`); we must
        // not reject it. Replicates that wire shape and asserts the consumer
        // gets a WebAuthn challenge with the populated channel id.
        use teleport_proto::prost::Message as _;
        use teleport_proto::teleport::desktop::v1::{envelope::Payload, Envelope, Mfa};
        use teleport_proto::teleport::mfa::v1::{AuthenticateChallenge, SsoChallenge};
        use teleport_proto::types::SsomfaDevice;

        let envelope = Envelope {
            payload: Some(Payload::Mfa(Box::new(Mfa {
                r#type: 0, // MFA_TYPE_UNSPECIFIED — server leaves this unset
                channel_id: "channel-42".to_owned(),
                challenge: Some(AuthenticateChallenge {
                    sso_challenge: Some(SsoChallenge {
                        redirect_url: "https://idp.example/auth".to_owned(),
                        request_id: "req-1".to_owned(),
                        device: Some(SsomfaDevice {
                            connector_id: "okta".to_owned(),
                            connector_type: "saml".to_owned(),
                            display_name: "Okta".to_owned(),
                        }),
                    }),
                    ..Default::default()
                }),
                authentication_response: None,
            }))),
        };
        let mut buf = Vec::new();
        envelope.encode(&mut buf).unwrap();

        let InboundMessage::MfaChallenge(c) = decode(frame(buf)).unwrap() else {
            panic!("wrong variant");
        };
        assert_eq!(c.kind, MfaKind::WebAuthn);
        let sso = c.challenge.sso_challenge.expect("sso_challenge present");
        assert_eq!(sso.channel_id, "channel-42");
        assert_eq!(sso.request_id, "req-1");
        assert_eq!(sso.device.connector_id, "okta");
    }

    #[test]
    fn mfa_with_unknown_nonzero_type_still_errors() {
        // The Unspecified default is the only value we infer; an unrecognised
        // non-zero value is a real wire-format violation and must propagate.
        use teleport_proto::prost::Message as _;
        use teleport_proto::teleport::desktop::v1::{envelope::Payload, Envelope, Mfa};
        use teleport_proto::teleport::mfa::v1::AuthenticateChallenge;

        let envelope = Envelope {
            payload: Some(Payload::Mfa(Box::new(Mfa {
                r#type: 999,
                channel_id: String::new(),
                challenge: Some(AuthenticateChallenge::default()),
                authentication_response: None,
            }))),
        };
        let mut buf = Vec::new();
        envelope.encode(&mut buf).unwrap();
        assert!(matches!(
            decode(frame(buf)),
            Err(CodecError::Invalid(crate::error::InvalidValue::MfaKind(999)))
        ));
    }
}
