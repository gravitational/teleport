use crate::messages::*;

pub enum InboundMessage {
    ClientHello(ClientHello),
    ServerHello(ServerHello),
    ConnectionActivated(ConnectionActivated),
    SessionSelection(SessionSelection),

    FastPathPdu(FastPathPdu),
    EgfxBitmap(EgfxBitmap),
    EgfxAvcFrame(EgfxAvcFrame),
    EgfxClearCodec(EgfxClearCodec),
    EgfxUncompressed(EgfxUncompressed),
    EgfxPlanar(EgfxPlanar),
    EgfxAvc420(EgfxAvc420),
    EgfxSolidFill(EgfxSolidFill),
    EgfxSurfaceToCache(EgfxSurfaceToCache),
    EgfxCacheToSurface(EgfxCacheToSurface),
    EgfxEvictCacheEntry(EgfxEvictCacheEntry),
    EgfxSurfaceToSurface(EgfxSurfaceToSurface),
    EgfxWireToSurface2(EgfxWireToSurface2),
    EgfxDeleteEncodingContext(EgfxDeleteEncodingContext),
    EgfxEndFrame(EgfxEndFrame),
    PngFrame(PngFrame),
    Png2Frame(Png2Frame),
    RdpResponsePdu(RdpResponsePdu),

    Alert(Alert),
    ClipboardIn(ClipboardIn),
    LatencyStats(LatencyStats),
    /// Boxed to keep `InboundMessage` compact: a typed MFA challenge carries
    /// several `String`s plus `Vec<AllowedCredential>` (~288 bytes), which
    /// would otherwise widen every variant — including the per-frame hot ones.
    MfaChallenge(Box<MfaChallenge>),
    Ping(Ping),
    ScreenSpec(ScreenSpec),

    KeyboardButton(KeyboardButton),
    MouseButton(MouseButton),
    MouseMove(MouseMove),
    MouseWheel(MouseWheel),
    SyncKeys(SyncKeys),

    /// Legacy-TDP-only sentinel signaling that the proxy is about to switch
    /// to TDPB framing. The driver should call `Codec::upgrade_to_tdpb`.
    TdpbUpgrade,

    ShareDirRequest(ShareDirRequest),
    ShareDirAck(ShareDirAck),
    ShareDirAnnounce(ShareDirAnnounce),
    ShareDirRemove(ShareDirRemove),
    ShareDirResponse(ShareDirResponse),

    /// Client→server only on the wire. Surfaced here so a server-side or
    /// session-recording consumer can identify the message; the live
    /// browser pipeline never receives this from the proxy.
    RefreshRect(RefreshRect),

    /// Legacy TDP only: an unknown or client-targeted type byte we can't
    /// project onto a typed variant. TDPB has no analogue — unrecogniszd
    /// envelope payloads are surfaced as `CodecError::Missing`.
    Unsupported(Unsupported),
}
