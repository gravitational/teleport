use crate::messages::enums::{
    ButtonState, FileType, MfaKind, MouseButtonKind, ScrollAxis, Severity,
};
use bytes::Bytes;

/// One monitor's position and size within the RDP virtual desktop, relative
/// to the primary monitor's origin. `is_primary` marks the single primary.
#[derive(Debug, Clone, Copy)]
pub struct MonitorLayout {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
    pub is_primary: bool,
}

#[derive(Debug, Clone, Default)]
pub struct ScreenSpec {
    pub width: u32,
    pub height: u32,
    pub scale: u32,
    /// Per-monitor layout. Empty for single-monitor sessions; the proxy
    /// synthesizes a primary from `width`/`height` when this is empty.
    pub monitors: Vec<MonitorLayout>,
}

#[derive(Debug, Clone, Copy)]
pub struct Rect {
    pub left: u32,
    pub top: u32,
    pub right: u32,
    pub bottom: u32,
}

#[derive(Debug, Clone, Copy)]
pub struct ConnectionActivated {
    pub io_channel_id: u16,
    pub user_channel_id: u16,
    pub screen_width: u16,
    pub screen_height: u16,
}

#[derive(Debug, Clone)]
pub struct Fso {
    pub last_modified: u64,
    pub file_type: FileType,
    pub size: u64,
    pub is_empty: bool,
    pub path: String,
}

#[derive(Debug, Clone)]
pub struct ClientHello {
    pub username: String,
    pub screen_spec: ScreenSpec,
    pub keyboard_layout: u32,
}

#[derive(Debug, Clone)]
pub struct ServerHello {
    pub activation: ConnectionActivated,
    pub clipboard_enabled: bool,
    pub directory_remove_supported: bool,
    pub hidpi_supported: bool,
    /// True when the server can negotiate a multi-monitor virtual desktop
    /// via the RDP DisplayControl DVC. Mirrors the proto bit of the same
    /// name; clients must only emit a non-empty `ScreenSpec.monitors`
    /// after seeing this advertised.
    pub multi_monitor_supported: bool,
}

#[derive(Debug, Clone)]
pub struct SessionSelection {
    pub name: String,
}

#[derive(Debug, Clone)]
pub struct PngFrame {
    pub rect: Rect,
    pub png: Bytes,
}

#[derive(Debug, Clone)]
pub struct Png2Frame {
    pub rect: Rect,
    pub png: Bytes,
}

#[derive(Debug, Clone)]
pub struct FastPathPdu {
    pub pdu: Bytes,
}

/// Pre-decoded EGFX (MS-RDPEGFX Graphics Pipeline) bitmap update. EGFX
/// payloads (uncompressed, ClearCodec, RemoteFX progressive, AVC420,
/// AVC444) are decoded in the server-side IronRDP cgo client; the wasm
/// decoder only sees the resulting RGBA in desktop coordinates. Required
/// for Windows 11 multi-monitor, which sends secondary surface frames
/// exclusively over the EGFX DVC.
#[derive(Debug, Clone)]
pub struct EgfxBitmap {
    pub desktop_x: u32,
    pub desktop_y: u32,
    pub width: u32,
    pub height: u32,
    pub rgba: Bytes,
}

/// An EGFX AVC444/v2 frame forwarded from the server. The H.264 streams
/// are still encoded — the wasm side feeds them to the browser's WebCodecs
/// `VideoDecoder` and composes YUV444 → RGBA client-side.
#[derive(Debug, Clone)]
pub struct EgfxAvcFrame {
    pub desktop_x: u32,
    pub desktop_y: u32,
    pub dest_width: u32,
    pub dest_height: u32,
    pub surface_id: u32,
    /// EGFX codec ID: 0xe = Avc444, 0xf = Avc444v2.
    pub codec_id: u32,
    /// Stream presence flag: 0 = LUMA_AND_CHROMA, 1 = LUMA, 2 = CHROMA.
    pub encoding: u32,
    pub luma_h264: Bytes,
    pub chroma_h264: Bytes,
}

/// A raw EGFX ClearCodec PDU ([MS-RDPEGFX] 2.2.4.2 / `RFX_CLEAR_BITMAP_STREAM`)
/// forwarded from the server for wasm-side decode. The wasm decoder writes
/// in-place into the framebuffer image and only paints the pixels the wire
/// format actually covers, preserving existing content for un-painted regions
/// of the destination rectangle (which a server-side decode-to-RGBA pipeline
/// would otherwise stomp with opaque black).
#[derive(Debug, Clone)]
pub struct EgfxClearCodec {
    pub surface_id: u32,
    pub dest_x: i32,
    pub dest_y: i32,
    pub width: u32,
    pub height: u32,
    pub pdu_data: Bytes,
}

/// A raw `Codec1Type::Planar` ([MS-RDPEGDI] 2.2.9.1.0.2 RDP 6.0 bitmap
/// stream) PDU forwarded for wasm-side decode. Decoded with
/// `ironrdp_graphics::rdp6::bitmap_stream`.
#[derive(Debug, Clone)]
pub struct EgfxPlanar {
    pub surface_id: u32,
    pub dest_x: i32,
    pub dest_y: i32,
    pub width: u32,
    pub height: u32,
    pub pdu_data: Bytes,
}

/// A raw `Codec1Type::Avc420` ([MS-RDPEGFX] 2.2.4.3) PDU. `pdu_data` is
/// the Avc420EncapsulatedBitmapStream: 12-byte header + concatenated H.264
/// NAL units. Decoded via the browser's WebCodecs `VideoDecoder`.
#[derive(Debug, Clone)]
pub struct EgfxAvc420 {
    pub surface_id: u32,
    pub dest_x: i32,
    pub dest_y: i32,
    pub width: u32,
    pub height: u32,
    pub pdu_data: Bytes,
}

/// A raw `Uncompressed` ([MS-RDPEGFX] 2.2.4.2, `Codec1Type::Uncompressed`)
/// WireToSurface1 PDU forwarded for wasm-side blit. Windows ships this for
/// small UI overlays — tooltips, popup chrome, hover shadows — and
/// frequently with an alpha channel (`PIXEL_FORMAT_ARGB_8888 = 0x21`),
/// which must be source-over composited against the existing framebuffer.
/// The server forwards the raw bytes without any byte-order or alpha
/// processing.
#[derive(Debug, Clone)]
pub struct EgfxUncompressed {
    pub surface_id: u32,
    pub dest_x: i32,
    pub dest_y: i32,
    pub width: u32,
    pub height: u32,
    /// MS-RDPEGFX pixel format byte: 0x20 = `PIXEL_FORMAT_XRGB_8888`
    /// (alpha byte is padding, treat as opaque), 0x21 =
    /// `PIXEL_FORMAT_ARGB_8888` (alpha byte is meaningful).
    pub pixel_format: u32,
    pub bitmap_data: Bytes,
}

/// Exclusive rectangle from EGFX (`right`/`bottom` are one-past-end).
#[derive(Debug, Clone, Copy)]
pub struct EgfxRect {
    pub left: u32,
    pub top: u32,
    pub right: u32,
    pub bottom: u32,
}

#[derive(Debug, Clone, Copy)]
pub struct EgfxPoint {
    pub x: u32,
    pub y: u32,
}

/// Solid color fill of one or more rectangles on a surface
/// ([MS-RDPEGFX] 2.2.2.4). The alpha component is "alpha-ignored" per spec
/// and the renderer treats every fill as opaque.
#[derive(Debug, Clone)]
pub struct EgfxSolidFill {
    pub surface_id: u32,
    pub color_b: u8,
    pub color_g: u8,
    pub color_r: u8,
    pub rects: Vec<EgfxRect>,
}

/// Copy a region of a surface into the bitmap cache at the given slot
/// ([MS-RDPEGFX] 2.2.2.6). Used together with `EgfxCacheToSurface` so the
/// server can tile a decoded image at many positions without re-encoding.
#[derive(Debug, Clone, Copy)]
pub struct EgfxSurfaceToCache {
    pub surface_id: u32,
    pub cache_key: u64,
    pub cache_slot: u32,
    pub source_rect: EgfxRect,
}

/// Blit a previously-cached region onto a surface at each of the given
/// destination points ([MS-RDPEGFX] 2.2.2.7).
#[derive(Debug, Clone)]
pub struct EgfxCacheToSurface {
    pub surface_id: u32,
    pub cache_slot: u32,
    pub dest_points: Vec<EgfxPoint>,
}

/// Drop a cached entry ([MS-RDPEGFX] 2.2.2.8).
#[derive(Debug, Clone, Copy)]
pub struct EgfxEvictCacheEntry {
    pub cache_slot: u32,
}

/// Copy a region between (or within) surfaces ([MS-RDPEGFX] 2.2.2.5).
/// Windows uses this for taskbar item moves, scroll, drag preview.
#[derive(Debug, Clone)]
pub struct EgfxSurfaceToSurface {
    pub source_surface_id: u32,
    pub destination_surface_id: u32,
    pub source_rect: EgfxRect,
    pub dest_points: Vec<EgfxPoint>,
}

/// A raw RFX Progressive payload ([MS-RDPEGFX] 2.2.2.2 + [MS-RDPRFX])
/// forwarded from the server. The wasm decoder maintains a stateful
/// per-(surface, codec_context_id) decoder; per-tile sub-band coefficient
/// state persists across PDUs and is evicted by [`EgfxDeleteEncodingContext`].
///
/// The destination rectangles live inside the Region block of the payload,
/// so unlike `EgfxClearCodec` the server cannot pre-translate a single dest
/// rect to desktop coords. We carry `surface_origin_x/y` and apply per-tile.
#[derive(Debug, Clone)]
pub struct EgfxWireToSurface2 {
    pub surface_id: u32,
    /// Codec identifier — `0x0009` (CAPROGRESSIVE_V1).
    pub codec_id: u32,
    pub codec_context_id: u32,
    /// 0 = XRgb, 1 = ARgb. Both are treated as opaque RGBA by the decoder.
    pub pixel_format: u32,
    pub surface_origin_x: u32,
    pub surface_origin_y: u32,
    pub bitmap_data: Bytes,
}

/// Evict the per-(surface, codec_context_id) progressive decoder state
/// ([MS-RDPEGFX] 2.2.2.3).
#[derive(Debug, Clone, Copy)]
pub struct EgfxDeleteEncodingContext {
    pub surface_id: u32,
    pub codec_context_id: u32,
}

/// The server finished a logical EGFX frame ([MS-RDPEGFX] 2.2.2.15). The
/// session presents on this boundary so only fully-composited frames reach the
/// canvas (presenting per wire-burst shows the bg fill before content =
/// black-rectangle flicker).
#[derive(Debug, Clone, Copy)]
pub struct EgfxEndFrame {
    pub frame_id: u32,
}

#[derive(Debug, Clone)]
pub struct RdpResponsePdu {
    pub response: Bytes,
}

#[derive(Debug, Clone)]
pub struct Alert {
    pub severity: Severity,
    pub message: String,
}

#[derive(Debug, Clone)]
pub struct ClipboardIn {
    pub data: String,
}

#[derive(Debug, Clone, Copy)]
pub struct LatencyStats {
    pub client_ms: u32,
    pub server_ms: u32,
}

#[derive(Debug, Clone)]
pub struct Ping {
    pub uuid: Bytes,
}

/// Typed MFA challenge. Legacy TDP parses the wire's JSON bytes into
/// [`MfaChallengeJson`]; TDPB lifts it from `AuthenticateChallenge`. Both
/// modes deliver the same Rust shape so consumers don't need to branch.
#[derive(Debug, Clone)]
pub struct MfaChallenge {
    pub kind: MfaKind,
    pub challenge: crate::messages::MfaChallengeJson,
}

#[derive(Debug, Clone, Copy)]
pub struct MouseMove {
    pub x: u32,
    pub y: u32,
}

#[derive(Debug, Clone, Copy)]
pub struct MouseButton {
    pub button: MouseButtonKind,
    pub pressed: bool,
}

#[derive(Debug, Clone, Copy)]
pub struct MouseWheel {
    pub axis: ScrollAxis,
    pub delta: i32,
}

#[derive(Debug, Clone, Copy)]
pub struct KeyboardButton {
    pub key_code: u32,
    pub pressed: bool,
}

#[derive(Debug, Clone, Copy)]
pub struct SyncKeys {
    pub scroll_lock: ButtonState,
    pub num_lock: ButtonState,
    pub caps_lock: ButtonState,
    pub kana_lock: ButtonState,
}

/// Ask the RDP server to repaint a region. Used to recover from RFX
/// decoder drift after a drag — the trailing window-border pixels stay
/// in the framebuffer until the server is asked to repaint over them.
/// Sent over TDPB only (TDP has no equivalent message).
#[derive(Debug, Clone, Copy)]
pub struct RefreshRect {
    pub left: u32,
    pub top: u32,
    pub right: u32,
    pub bottom: u32,
}

/// Legacy-TDP escape hatch: the type byte parsed but no decoder is wired up
/// (e.g. a server-targeted message reflected in a recording). TDPB has no
/// equivalent — unknown envelope payloads surface as `CodecError::Missing`.
#[derive(Debug, Clone, Copy)]
pub struct Unsupported {
    pub tdp_type: u8,
}
