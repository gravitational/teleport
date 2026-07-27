use crate::error::{CodecError, EncodeError, InvalidValue};
use crate::messages::*;
use teleport_proto::teleport::desktop::v1 as proto;

impl TryFrom<proto::ConnectionActivated> for ConnectionActivated {
    type Error = CodecError;

    fn try_from(p: proto::ConnectionActivated) -> Result<Self, CodecError> {
        fn narrow(field: &'static str, v: u32) -> Result<u16, CodecError> {
            u16::try_from(v).map_err(|_| {
                CodecError::Encode(EncodeError::ValueOverflow {
                    field,
                    value: i64::from(v),
                    target: "u16",
                })
            })
        }

        Ok(Self {
            io_channel_id: narrow("ConnectionActivated.io_channel_id", p.io_channel_id)?,
            user_channel_id: narrow("ConnectionActivated.user_channel_id", p.user_channel_id)?,
            screen_width: narrow("ConnectionActivated.screen_width", p.screen_width)?,
            screen_height: narrow("ConnectionActivated.screen_height", p.screen_height)?,
        })
    }
}

impl From<&ConnectionActivated> for proto::ConnectionActivated {
    fn from(d: &ConnectionActivated) -> Self {
        Self {
            io_channel_id: u32::from(d.io_channel_id),
            user_channel_id: u32::from(d.user_channel_id),
            screen_width: u32::from(d.screen_width),
            screen_height: u32::from(d.screen_height),
        }
    }
}

impl From<proto::Rectangle> for Rect {
    fn from(p: proto::Rectangle) -> Self {
        Self {
            left: p.left,
            top: p.top,
            right: p.right,
            bottom: p.bottom,
        }
    }
}

impl From<proto::MonitorLayout> for MonitorLayout {
    fn from(p: proto::MonitorLayout) -> Self {
        Self {
            x: p.x,
            y: p.y,
            width: p.width,
            height: p.height,
            is_primary: p.is_primary,
        }
    }
}

impl From<&MonitorLayout> for proto::MonitorLayout {
    fn from(m: &MonitorLayout) -> Self {
        Self {
            x: m.x,
            y: m.y,
            width: m.width,
            height: m.height,
            is_primary: m.is_primary,
        }
    }
}

impl From<proto::ClientScreenSpec> for ScreenSpec {
    fn from(p: proto::ClientScreenSpec) -> Self {
        Self {
            width: p.width,
            height: p.height,
            scale: p.scale,
            monitors: p.monitors.into_iter().map(Into::into).collect(),
        }
    }
}

impl From<&ScreenSpec> for proto::ClientScreenSpec {
    fn from(s: &ScreenSpec) -> Self {
        Self {
            width: s.width,
            height: s.height,
            scale: s.scale,
            monitors: s.monitors.iter().map(Into::into).collect(),
        }
    }
}

impl TryFrom<proto::FileSystemObject> for Fso {
    type Error = CodecError;

    fn try_from(p: proto::FileSystemObject) -> Result<Self, CodecError> {
        Ok(Self {
            last_modified: p.last_modified,
            file_type: FileType::try_from(p.file_type)?,
            size: p.size,
            is_empty: p.is_empty,
            path: p.path,
        })
    }
}

impl From<&Fso> for proto::FileSystemObject {
    fn from(d: &Fso) -> Self {
        Self {
            last_modified: d.last_modified,
            file_type: d.file_type as u32,
            size: d.size,
            is_empty: d.is_empty,
            path: d.path.clone(),
        }
    }
}

/// Generates a `TryFrom<i32>` that maps the listed proto-enum variants to a
/// Rust enum and rejects everything else (including the proto's `Unspecified`
/// zero value) with `InvalidValue::$invalid(v)`.
macro_rules! proto_enum {
    ($rust:ty, $proto:path, $invalid:ident, { $($from:ident => $to:ident),* $(,)? }) => {
        impl TryFrom<i32> for $rust {
            type Error = CodecError;

            fn try_from(v: i32) -> Result<Self, CodecError> {
                use $proto as P;
                match P::try_from(v) {
                    $( Ok(P::$from) => Ok(Self::$to), )*
                    _ => Err(CodecError::Invalid(InvalidValue::$invalid(v))),
                }
            }
        }
    };
}

proto_enum!(Severity,        proto::AlertSeverity,   Severity,    { Info => Info, Warning => Warning, Error => Error });
proto_enum!(MouseButtonKind, proto::MouseButtonType, MouseButton, { Left => Left, Middle => Middle, Right => Right });
proto_enum!(ScrollAxis,      proto::MouseWheelAxis,  ScrollAxis,  { Vertical => Vertical, Horizontal => Horizontal });
proto_enum!(MfaKind,         proto::MfaType,         MfaKind,     { Webauthn => WebAuthn, U2f => U2f });

impl TryFrom<proto::ClientHello> for ClientHello {
    type Error = CodecError;

    fn try_from(p: proto::ClientHello) -> Result<Self, CodecError> {
        Ok(Self {
            username: p.username,
            screen_spec: p
                .screen_spec
                .ok_or(CodecError::Missing("ClientHello.screen_spec"))?
                .into(),
            keyboard_layout: p.keyboard_layout,
        })
    }
}

impl TryFrom<proto::ServerHello> for ServerHello {
    type Error = CodecError;

    fn try_from(p: proto::ServerHello) -> Result<Self, CodecError> {
        Ok(Self {
            activation: p
                .activation_spec
                .ok_or(CodecError::Missing("ServerHello.activation_spec"))?
                .try_into()?,
            clipboard_enabled: p.clipboard_enabled,
            directory_remove_supported: p.directory_remove_supported,
            hidpi_supported: p.hidpi_supported,
            multi_monitor_supported: p.multi_monitor_supported,
        })
    }
}

impl TryFrom<proto::SessionSelection> for SessionSelection {
    type Error = CodecError;

    fn try_from(p: proto::SessionSelection) -> Result<Self, CodecError> {
        Ok(Self {
            name: p
                .session
                .ok_or(CodecError::Missing("SessionSelection.session"))?
                .name,
        })
    }
}

impl TryFrom<proto::PngFrame> for PngFrame {
    type Error = CodecError;

    fn try_from(p: proto::PngFrame) -> Result<Self, CodecError> {
        Ok(Self {
            rect: p
                .coordinates
                .ok_or(CodecError::Missing("PNGFrame.coordinates"))?
                .into(),
            png: p.data,
        })
    }
}

impl From<proto::FastPathPdu> for FastPathPdu {
    fn from(p: proto::FastPathPdu) -> Self {
        Self { pdu: p.pdu }
    }
}

impl From<proto::EgfxBitmap> for EgfxBitmap {
    fn from(p: proto::EgfxBitmap) -> Self {
        Self {
            desktop_x: p.desktop_x,
            desktop_y: p.desktop_y,
            width: p.width,
            height: p.height,
            rgba: p.rgba,
        }
    }
}

impl From<proto::EgfxAvcFrame> for EgfxAvcFrame {
    fn from(p: proto::EgfxAvcFrame) -> Self {
        Self {
            desktop_x: p.desktop_x,
            desktop_y: p.desktop_y,
            dest_width: p.dest_width,
            dest_height: p.dest_height,
            surface_id: p.surface_id,
            codec_id: p.codec_id,
            encoding: p.encoding,
            luma_h264: p.luma_h264,
            chroma_h264: p.chroma_h264,
        }
    }
}

impl From<proto::EgfxClearCodec> for EgfxClearCodec {
    fn from(p: proto::EgfxClearCodec) -> Self {
        Self {
            surface_id: p.surface_id,
            dest_x: p.dest_x,
            dest_y: p.dest_y,
            width: p.width,
            height: p.height,
            pdu_data: p.pdu_data,
        }
    }
}

impl From<proto::EgfxUncompressed> for EgfxUncompressed {
    fn from(p: proto::EgfxUncompressed) -> Self {
        Self {
            surface_id: p.surface_id,
            dest_x: p.dest_x,
            dest_y: p.dest_y,
            width: p.width,
            height: p.height,
            pixel_format: p.pixel_format,
            bitmap_data: p.bitmap_data,
        }
    }
}

impl From<proto::EgfxPlanar> for EgfxPlanar {
    fn from(p: proto::EgfxPlanar) -> Self {
        Self {
            surface_id: p.surface_id,
            dest_x: p.dest_x,
            dest_y: p.dest_y,
            width: p.width,
            height: p.height,
            pdu_data: p.pdu_data,
        }
    }
}

impl From<proto::EgfxAvc420> for EgfxAvc420 {
    fn from(p: proto::EgfxAvc420) -> Self {
        Self {
            surface_id: p.surface_id,
            dest_x: p.dest_x,
            dest_y: p.dest_y,
            width: p.width,
            height: p.height,
            pdu_data: p.pdu_data,
        }
    }
}

impl From<proto::EgfxRect> for EgfxRect {
    fn from(p: proto::EgfxRect) -> Self {
        Self {
            left: p.left,
            top: p.top,
            right: p.right,
            bottom: p.bottom,
        }
    }
}

impl From<proto::EgfxPoint> for EgfxPoint {
    fn from(p: proto::EgfxPoint) -> Self {
        Self { x: p.x, y: p.y }
    }
}

impl From<proto::EgfxSolidFill> for EgfxSolidFill {
    fn from(p: proto::EgfxSolidFill) -> Self {
        // u32 wire fields narrow to u8; saturating preserves the visible
        // color for any out-of-range value (the wire format actually only
        // ever produces 0..=255 since the field encodes one byte).
        Self {
            surface_id: p.surface_id,
            color_b: p.color_b.min(0xff) as u8,
            color_g: p.color_g.min(0xff) as u8,
            color_r: p.color_r.min(0xff) as u8,
            rects: p.rects.into_iter().map(Into::into).collect(),
        }
    }
}

impl From<proto::EgfxSurfaceToCache> for EgfxSurfaceToCache {
    fn from(p: proto::EgfxSurfaceToCache) -> Self {
        Self {
            surface_id: p.surface_id,
            cache_key: p.cache_key,
            cache_slot: p.cache_slot,
            source_rect: p.source_rect.unwrap_or(proto::EgfxRect::default()).into(),
        }
    }
}

impl From<proto::EgfxCacheToSurface> for EgfxCacheToSurface {
    fn from(p: proto::EgfxCacheToSurface) -> Self {
        Self {
            surface_id: p.surface_id,
            cache_slot: p.cache_slot,
            dest_points: p.dest_points.into_iter().map(Into::into).collect(),
        }
    }
}

impl From<proto::EgfxEvictCacheEntry> for EgfxEvictCacheEntry {
    fn from(p: proto::EgfxEvictCacheEntry) -> Self {
        Self {
            cache_slot: p.cache_slot,
        }
    }
}

impl From<proto::EgfxSurfaceToSurface> for EgfxSurfaceToSurface {
    fn from(p: proto::EgfxSurfaceToSurface) -> Self {
        Self {
            source_surface_id: p.source_surface_id,
            destination_surface_id: p.destination_surface_id,
            source_rect: p.source_rect.unwrap_or(proto::EgfxRect::default()).into(),
            dest_points: p.dest_points.into_iter().map(Into::into).collect(),
        }
    }
}

impl From<proto::EgfxEndFrame> for EgfxEndFrame {
    fn from(p: proto::EgfxEndFrame) -> Self {
        Self {
            frame_id: p.frame_id,
        }
    }
}

impl From<proto::EgfxWireToSurface2> for EgfxWireToSurface2 {
    fn from(p: proto::EgfxWireToSurface2) -> Self {
        Self {
            surface_id: p.surface_id,
            codec_id: p.codec_id,
            codec_context_id: p.codec_context_id,
            pixel_format: p.pixel_format,
            surface_origin_x: p.surface_origin_x,
            surface_origin_y: p.surface_origin_y,
            bitmap_data: p.bitmap_data,
        }
    }
}

impl From<proto::EgfxDeleteEncodingContext> for EgfxDeleteEncodingContext {
    fn from(p: proto::EgfxDeleteEncodingContext) -> Self {
        Self {
            surface_id: p.surface_id,
            codec_context_id: p.codec_context_id,
        }
    }
}

impl From<proto::RdpResponsePdu> for RdpResponsePdu {
    fn from(p: proto::RdpResponsePdu) -> Self {
        Self {
            response: p.response,
        }
    }
}

impl TryFrom<proto::Alert> for Alert {
    type Error = CodecError;

    fn try_from(p: proto::Alert) -> Result<Self, CodecError> {
        Ok(Self {
            severity: p.severity.try_into()?,
            message: p.message,
        })
    }
}

impl TryFrom<proto::ClipboardData> for ClipboardIn {
    type Error = CodecError;

    fn try_from(p: proto::ClipboardData) -> Result<Self, CodecError> {
        Ok(Self {
            data: std::str::from_utf8(&p.data)
                .map(str::to_owned)
                .map_err(|_| CodecError::Invalid(InvalidValue::Utf8))?,
        })
    }
}

impl From<proto::LatencyStats> for LatencyStats {
    fn from(p: proto::LatencyStats) -> Self {
        Self {
            client_ms: p.client_latency_ms,
            server_ms: p.server_latency_ms,
        }
    }
}

impl From<proto::Ping> for Ping {
    fn from(p: proto::Ping) -> Self {
        Self { uuid: p.uuid }
    }
}

impl From<proto::MouseMove> for MouseMove {
    fn from(p: proto::MouseMove) -> Self {
        Self { x: p.x, y: p.y }
    }
}

impl TryFrom<proto::MouseButton> for MouseButton {
    type Error = CodecError;

    fn try_from(p: proto::MouseButton) -> Result<Self, CodecError> {
        Ok(Self {
            button: MouseButtonKind::try_from(p.button)?,
            pressed: p.pressed,
        })
    }
}

impl TryFrom<proto::MouseWheel> for MouseWheel {
    type Error = CodecError;

    fn try_from(p: proto::MouseWheel) -> Result<Self, CodecError> {
        Ok(Self {
            axis: ScrollAxis::try_from(p.axis)?,
            delta: p.delta,
        })
    }
}

impl From<proto::KeyboardButton> for KeyboardButton {
    fn from(p: proto::KeyboardButton) -> Self {
        Self {
            key_code: p.key_code,
            pressed: p.pressed,
        }
    }
}

impl From<proto::SyncKeys> for SyncKeys {
    fn from(p: proto::SyncKeys) -> Self {
        use ButtonState::{Down, Up};

        let b = |on: bool| if on { Down } else { Up };

        Self {
            scroll_lock: b(p.scroll_lock_pressed),
            num_lock: b(p.num_lock_state),
            caps_lock: b(p.caps_lock_state),
            kana_lock: b(p.kana_lock_state),
        }
    }
}

impl From<&SyncKeys> for proto::SyncKeys {
    fn from(k: &SyncKeys) -> Self {
        Self {
            scroll_lock_pressed: matches!(k.scroll_lock, ButtonState::Down),
            num_lock_state: matches!(k.num_lock, ButtonState::Down),
            caps_lock_state: matches!(k.caps_lock, ButtonState::Down),
            kana_lock_state: matches!(k.kana_lock, ButtonState::Down),
        }
    }
}

impl From<proto::SharedDirectoryAnnounce> for ShareDirAnnounce {
    fn from(p: proto::SharedDirectoryAnnounce) -> Self {
        Self {
            directory_id: p.directory_id,
            name: p.name,
        }
    }
}

impl From<proto::SharedDirectoryRemove> for ShareDirRemove {
    fn from(p: proto::SharedDirectoryRemove) -> Self {
        Self {
            directory_id: p.directory_id,
        }
    }
}

impl TryFrom<proto::SharedDirectoryAcknowledge> for ShareDirAck {
    type Error = CodecError;

    fn try_from(p: proto::SharedDirectoryAcknowledge) -> Result<Self, CodecError> {
        Ok(Self {
            err: SharedDirErrCode::try_from(p.error_code)?,
            directory_id: p.directory_id,
        })
    }
}

impl TryFrom<proto::SharedDirectoryRequest> for ShareDirRequest {
    type Error = CodecError;

    fn try_from(p: proto::SharedDirectoryRequest) -> Result<Self, CodecError> {
        use proto::shared_directory_request::Operation as Op;
        use ShareDirRequest as R;

        let completion_id = p.completion_id;
        let directory_id = p.directory_id;
        let op = p
            .operation
            .ok_or(CodecError::Missing("SharedDirectoryRequest.operation"))?;

        Ok(match op {
            Op::Info(i) => R::Info {
                completion_id,
                directory_id,
                path: i.path,
            },
            Op::Create(c) => R::Create {
                completion_id,
                directory_id,
                file_type: FileType::try_from(c.file_type)?,
                path: c.path,
            },
            Op::Delete(d) => R::Delete {
                completion_id,
                directory_id,
                path: d.path,
            },
            Op::List(l) => R::List {
                completion_id,
                directory_id,
                path: l.path,
            },
            Op::Read(r) => R::Read {
                completion_id,
                directory_id,
                path: r.path,
                offset: r.offset,
                length: r.length,
            },
            Op::Write(w) => R::Write {
                completion_id,
                directory_id,
                path: w.path,
                offset: w.offset,
                data: w.data,
            },
            Op::Move(m) => R::Move {
                completion_id,
                directory_id,
                original: m.original_path,
                new: m.new_path,
            },
            Op::Truncate(t) => R::Truncate {
                completion_id,
                directory_id,
                path: t.path,
                size: t.size,
            },
        })
    }
}

impl TryFrom<proto::SharedDirectoryResponse> for ShareDirResponse {
    type Error = CodecError;

    fn try_from(p: proto::SharedDirectoryResponse) -> Result<Self, CodecError> {
        use proto::shared_directory_response::Operation as Op;

        let completion_id = p.completion_id;
        let err = SharedDirErrCode::try_from(p.error_code)?;
        let op = p
            .operation
            .ok_or(CodecError::Missing("SharedDirectoryResponse.operation"))?;

        Ok(match op {
            Op::Info(i) => ShareDirResponse::Info(SharedDirInfoResponse {
                completion_id,
                err,
                fso: i
                    .fso
                    .ok_or(CodecError::Missing("SharedDirectoryResponse.Info.fso"))?
                    .try_into()?,
            }),
            Op::Create(c) => ShareDirResponse::Create(SharedDirCreateResponse {
                completion_id,
                err,
                fso: c
                    .fso
                    .ok_or(CodecError::Missing("SharedDirectoryResponse.Create.fso"))?
                    .try_into()?,
            }),
            Op::Delete(_) => {
                ShareDirResponse::Delete(SharedDirDeleteResponse { completion_id, err })
            }
            Op::List(l) => ShareDirResponse::List(SharedDirListResponse {
                completion_id,
                err,
                fsos: l
                    .fso_list
                    .into_iter()
                    .map(Fso::try_from)
                    .collect::<Result<_, _>>()?,
            }),
            Op::Read(r) => ShareDirResponse::Read(SharedDirReadResponse {
                completion_id,
                err,
                data: r.data,
            }),
            Op::Write(w) => ShareDirResponse::Write(SharedDirWriteResponse {
                completion_id,
                err,
                bytes_written: w.bytes_written,
            }),
            Op::Move(_) => {
                ShareDirResponse::Move(SharedDirMoveResponse { completion_id, err })
            }
            Op::Truncate(_) => {
                ShareDirResponse::Truncate(SharedDirTruncateResponse { completion_id, err })
            }
        })
    }
}
