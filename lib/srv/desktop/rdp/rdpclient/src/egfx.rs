// EGFX (MS-RDPEGFX Graphics Pipeline) client integration.
//
// Windows 11 negotiates EGFX whenever the server announces
// `DYNVC_GFX_PROTOCOL_SUPPORTED`, and on multi-monitor sessions it sends
// secondary monitor content exclusively over this channel. Without an
// EGFX handler registered on `Microsoft::Windows::RDS::Graphics`, the
// server refuses the create with `NO_LISTENER` and the secondary monitor
// stays blank.
//
// Architecture: IronRDP's `GraphicsPipelineClient` does all of the heavy
// lifting (capability negotiation, ZGFX decompression, AVC420/AVC444 H.264
// decode via bundled OpenH264, surface tracking, MapSurfaceToOutput
// resolution). We provide a [`TeleportEgfxHandler`] that receives each
// decoded `BitmapUpdate` and forwards it to Go (and from there to the
// browser) as a `tdpb::EgfxBitmap` message — desktop-coordinate RGBA that
// the wasm decoder can blit directly into its framebuffer image without
// knowing anything about EGFX surface IDs or codec internals.

use std::collections::HashMap;

use ironrdp_core::ReadCursor;
use ironrdp_egfx::client::{BitmapUpdate, GraphicsPipelineHandler, Surface};
use ironrdp_egfx::pdu::{
    Avc444BitmapStream, CacheToSurfacePdu, CapabilitiesV104Flags, CapabilitiesV107Flags,
    CapabilitiesV81Flags, CapabilitiesV8Flags, CapabilitySet, Codec1Type, Encoding,
    MapSurfaceToScaledOutputPdu, SolidFillPdu, SurfaceToCachePdu, SurfaceToSurfacePdu,
};
use ironrdp_pdu::geometry::Rectangle as _;
use ironrdp_pdu::Decode as _;
use log::{info, warn};

use crate::{
    cgo_handle_egfx_avc420, cgo_handle_egfx_avc_frame, cgo_handle_egfx_bitmap,
    cgo_handle_egfx_cache_to_surface, cgo_handle_egfx_clearcodec,
    cgo_handle_egfx_delete_encoding_context, cgo_handle_egfx_end_frame,
    cgo_handle_egfx_evict_cache_entry, cgo_handle_egfx_planar, cgo_handle_egfx_solid_fill,
    cgo_handle_egfx_surface_to_cache, cgo_handle_egfx_surface_to_surface,
    cgo_handle_egfx_uncompressed, cgo_handle_egfx_wire_to_surface2, cgo_handle_rdp_reset_graphics,
};

/// Log when the server sends H.264/AVC frames — the signal that the V10.4 AVC
/// capabilities took effect (vs the CPU-bound RFX-progressive path). Logs the
/// first frame and every 120th so it confirms sustained AVC without flooding
/// the server log. (Revert once the V10.4 experiment is settled.)
fn note_avc(codec: Codec1Type) {
    use std::sync::atomic::{AtomicU64, Ordering};
    static AVC_COUNT: AtomicU64 = AtomicU64::new(0);
    let n = AVC_COUNT.fetch_add(1, Ordering::Relaxed);
    if n == 0 || n % 120 == 0 {
        info!("[teleport][egfx][avc] AVC frame #{n} codec={codec:?} — H.264 is flowing (negotiated version logged at capabilities-confirmed)");
    }
}
use crate::{CGOErrCode, CgoHandle};

/// EGFX handler that forwards decoded bitmap updates to Go via cgo. We
/// track surface→output mappings here (rather than relying solely on
/// `GraphicsPipelineClient::get_surface`) so we can compute desktop
/// coordinates from a `BitmapUpdate` without re-querying the client.
pub struct TeleportEgfxHandler {
    cgo_handle: CgoHandle,
    /// surface_id -> (origin_x, origin_y) in desktop coordinates. Populated
    /// from MapSurfaceToOutput / MapSurfaceToScaledOutput PDUs.
    surface_origins: HashMap<u16, (u32, u32)>,
    /// One-shot probe — set after the first `on_bitmap_updated` callback so
    /// we can tell whether IronRDP is still decoding internally despite the
    /// `Avc420`/`Uncompressed` dispatch patch.
    seen_bitmap_updated: bool,
    /// DIAGNOSTIC (revert): surface ids for which we've already emitted the
    /// one-time `[origin-probe]` log from the cache/fill/copy forwarders, so
    /// we log each surface's origin once instead of per-PDU.
    logged_origin_surfaces: Vec<u16>,
}

impl TeleportEgfxHandler {
    pub fn new(cgo_handle: CgoHandle) -> Self {
        Self {
            cgo_handle,
            surface_origins: HashMap::new(),
            seen_bitmap_updated: false,
            logged_origin_surfaces: Vec::new(),
        }
    }

    /// DIAGNOSTIC (revert): emit a one-time-per-surface log showing the
    /// MapSurfaceToOutput origin applied to a cache/fill/copy op, so we can
    /// confirm (in the server log) whether origins are actually non-zero and
    /// the translation is doing anything.
    fn maybe_log_origin(&mut self, op: &str, surface_id: u16, origin: (u32, u32), raw: (u32, u32)) {
        if self.logged_origin_surfaces.contains(&surface_id) {
            return;
        }
        self.logged_origin_surfaces.push(surface_id);
        warn!(
            "[teleport][egfx][origin-probe] op={} surface={} origin=({},{}) raw=({},{}) -> desktop=({},{})",
            op,
            surface_id,
            origin.0,
            origin.1,
            raw.0,
            raw.1,
            origin.0.saturating_add(raw.0),
            origin.1.saturating_add(raw.1),
        );
    }
}

impl GraphicsPipelineHandler for TeleportEgfxHandler {
    fn capabilities(&self) -> Vec<CapabilitySet> {
        // CURRENT STATE (RFX-only): AVC is DISABLED on every rung so the host
        // never promotes video regions to H.264 — we keep the sharper (if more
        // expensive) RFX-Progressive path. See the cap vec below for how to
        // re-enable region-split or full-screen thin-client AVC.
        //
        // EXPERIMENT (full-screen thin-client) — what AVC_THIN_CLIENT was for:
        // advertise V10.7 (AVC444v2) with AVC_THIN_CLIENT set → ask the server to
        // push ALL surface content through H.264. GOAL: determine whether this host
        // emits AVC444 (codecId 14/15) in full-screen mode. If it does, the path is
        // full-screen AVC444 + client-side chroma recombination (crisp text, ~0%
        // client CPU, no compositing seam). If it still emits AVC420 (codecId 11),
        // the host is 420-by-choice and the only crisp+smooth path is parallelizing
        // the RFX decode.
        //
        // RUN LOG (kept so findings aren't lost):
        // - region-split (V10.7, flags=empty): server CONFIRMED V10_7, but sent
        //   AVC420 (codecId 11) for the video region while RFX-progressive still
        //   pegged the wasm decode thread (~1000ms/s) — region-split does NOT offload
        //   video to AVC on this host, and the AVC/UI boundary showed the green seam.
        //   Since the host agreed to the AVC444v2 cap yet chose 420, the 420 is a
        //   Windows MODE decision, not an encoder limit → full-screen mode may yield
        //   444. OBSERVE [avc-probe] codecId on this run.
        // - thin-client softness root cause: the wasm client decodes only the H.264
        //   luma/main view and DROPS the AVC444 chroma auxiliary stream (which the
        //   proxy forwards). So a 444 stream renders at 4:2:0 until client-side
        //   chroma recombination is implemented.
        // - The green seam: the AVC fast path uploads straight to the GPU texture,
        //   bypassing the CPU framebuffer where ClearCodec/RFX composite. Full-screen
        //   AVC has no AVC/UI boundary, so it avoids the seam entirely.
        // - The durable, host-independent crisp-UI + smooth-video fix is parallelizing
        //   the RFX decode (see desktop-summaries/PARALLEL_RFX_DECODE_HANDOFF.md).
        //
        // All rungs kept so the server still negotiates the highest version it
        // supports (preserving V10.x surface/cache features) but with AVC turned
        // OFF, so video arrives as RFX-Progressive instead of H.264.
        // Note: CapabilitiesV81Flags has no AVC_DISABLED bit — dropping
        // AVC420_ENABLED is the only way to refuse AVC on the V8.1 rung.
        let caps = vec![
            // REGION-SPLIT AVC: V10.7/V10.4 advertised with empty flags (no
            // AVC_DISABLED, no AVC_THIN_CLIENT) so the host's adaptive video
            // detector promotes sustained-motion regions (video playback) to
            // H.264 while the UI stays RFX-Progressive. RFX-only video is
            // supply-bound (server encode + bandwidth — see the pacing notes
            // in commit a57a378d1b9): playback flooded the link with
            // full-region progressive deltas, ~4x worse at HiDPI. The two
            // blockers from the previous region-split run (RUN LOG above) are
            // resolved since: RFX decode is pooled, and the AVC green
            // seam/crop bugs are fixed in codecTestWorker. The AVC 4:2:0
            // quality tradeoff now applies only to motion regions, where it's
            // perceptually fine. To go back to RFX-only: V10.7/V10.4 →
            // AVC_DISABLED and drop AVC420_ENABLED from V8.1.
            CapabilitySet::V10_7 {
                flags: CapabilitiesV107Flags::empty(),
            },
            CapabilitySet::V10_4 {
                flags: CapabilitiesV104Flags::empty(),
            },
            CapabilitySet::V8_1 {
                flags: CapabilitiesV81Flags::SMALL_CACHE
                    | CapabilitiesV81Flags::AVC420_ENABLED,
            },
            CapabilitySet::V8 {
                flags: CapabilitiesV8Flags::SMALL_CACHE,
            },
        ];
        // DIAGNOSTIC: log exactly what we advertise. IronRDP never adds
        // AVC_DISABLED (a missing H264 decoder makes it DROP whole AVC rungs ->
        // V8 fallback), so if the server confirms V10_7 { AVC_DISABLED } this line
        // tells us whether we ourselves sent AVC_DISABLED (stale binary / old
        // code) vs sent empty() (then the server set it). If this line is ABSENT
        // from the node log, the running binary predates this build = stale.
        info!("[teleport][egfx] advertising capabilities: {:?}", caps);
        caps
    }

    fn on_capabilities_confirmed(&mut self, caps: &CapabilitySet) {
        info!(
            "[teleport][egfx] capabilities confirmed by server: {:?}",
            caps
        );
    }

    fn on_reset_graphics(&mut self, width: u32, height: u32) {
        info!(
            "[teleport][egfx] reset_graphics: virtual desktop now {}x{}",
            width, height
        );
        // Forward the new virtual-desktop size to Go → browser so the wasm
        // decoder grows its framebuffer to the new bounding box. A
        // DisplayControl monitor-layout change (add/move/resize a monitor) does
        // NOT trigger a protocol reactivation, so this ResetGraphics is the only
        // signal carrying the new size. Without it the framebuffer stays at the
        // old size and graphics for the grown region land outside it and render
        // black ("tile out of surface bounds").
        let _ = unsafe { cgo_handle_rdp_reset_graphics(self.cgo_handle, width, height) };
        // Drop the per-surface origins: the vendored IronRDP client destroys
        // ALL surfaces on ResetGraphics (ironrdp-egfx client.rs, "ResetGraphics
        // implicitly destroys all surfaces") and Windows re-creates + re-maps
        // them afterwards. Keeping origins for old ids stamped STALE offsets
        // onto PDUs when Windows re-used a surface id after a mid-session
        // monitor add/move/resize — misplaced rects until the next remap. A
        // missing entry merely warns and assumes (0,0) until the fresh
        // MapSurfaceToOutput lands, which is strictly better.
        self.surface_origins.clear();
    }

    fn on_surface_created(&mut self, surface: &Surface) {
        info!(
            "[teleport][egfx] surface_created: id={} {}x{} {:?}",
            surface.id, surface.width, surface.height, surface.pixel_format
        );
    }

    fn on_surface_deleted(&mut self, surface_id: u16) {
        info!("[teleport][egfx] surface_deleted: id={}", surface_id);
        self.surface_origins.remove(&surface_id);
        // Tell the browser the surface is GONE, riding the existing (ordered)
        // DeleteEncodingContext plumbing with the whole-surface sentinel
        // (codec_context_id = u32::MAX). The wasm decoders keep per-surface
        // RFX-Progressive state that must survive ResetGraphics (Windows often
        // keeps surfaces and their streams across it) but must NOT survive a
        // real surface deletion: a recreated surface reusing this id would
        // difference its fresh stream against the dead surface's accumulators
        // — corrupt rects after every monitor add/remove.
        let err = unsafe {
            cgo_handle_egfx_delete_encoding_context(
                self.cgo_handle,
                u32::from(surface_id),
                u32::MAX,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] surface_deleted forward (sentinel) returned {:?}",
                err
            );
        }
    }

    fn on_surface_mapped(&mut self, surface_id: u16, origin_x: u32, origin_y: u32) {
        info!(
            "[teleport][egfx] surface_mapped: id={} -> desktop ({}, {})",
            surface_id, origin_x, origin_y
        );
        self.surface_origins
            .insert(surface_id, (origin_x, origin_y));
    }

    fn on_map_surface_to_scaled_output(&mut self, pdu: &MapSurfaceToScaledOutputPdu) {
        // Treat scaled mapping the same as unscaled for routing purposes:
        // we only need the origin to translate bitmap updates into desktop
        // coords. Any per-surface scaling is reflected by the surface's
        // declared dimensions vs. the BitmapUpdate width/height, which we
        // simply forward as-is.
        info!(
            "[teleport][egfx] surface_mapped_scaled: id={} -> desktop ({}, {})",
            pdu.surface_id, pdu.output_origin_x, pdu.output_origin_y
        );
        self.surface_origins
            .insert(pdu.surface_id, (pdu.output_origin_x, pdu.output_origin_y));
    }

    fn on_bitmap_updated(&mut self, update: &BitmapUpdate) {
        // One-shot probe: if anything reaches this callback after the
        // IronRDP-egfx dispatch patch, we have another internal decode
        // path we missed.
        if !self.seen_bitmap_updated {
            self.seen_bitmap_updated = true;
            warn!(
                "[teleport][egfx][bitmap-updated-probe] FIRST on_bitmap_updated received — codec={:?} surface={} dest={:?} bytes={}",
                update.codec_id, update.surface_id, update.destination_rectangle, update.data.len(),
            );
        }
        // Skip empty/skipped decodes (decoder absent, etc.) — the client
        // emits these so handlers can observe metadata even when no
        // pixels are available, but we have nothing useful to forward.
        if update.data.is_empty() {
            warn!(
                "[teleport][egfx] bitmap_updated: surface_id={} codec={:?} but data empty (decoder missing?)",
                update.surface_id, update.codec_id
            );
            return;
        }

        let (origin_x, origin_y) = self
            .surface_origins
            .get(&update.surface_id)
            .copied()
            .unwrap_or_else(|| {
                warn!(
                    "[teleport][egfx] bitmap_updated: surface_id={} has no MapSurfaceToOutput entry; assuming (0,0)",
                    update.surface_id
                );
                (0, 0)
            });

        let desktop_x = origin_x.saturating_add(update.destination_rectangle.left.into());
        let desktop_y = origin_y.saturating_add(update.destination_rectangle.top.into());

        let expected = (update.width as usize) * (update.height as usize) * 4;
        if update.data.len() != expected {
            warn!(
                "[teleport][egfx] bitmap_updated: size mismatch — expected {} bytes for {}x{}, got {}",
                expected,
                update.width,
                update.height,
                update.data.len()
            );
            return;
        }

        // SAFETY: Go copies the buffer before returning; no aliasing with
        // mutable Rust state. The cast to `*mut u8` is required by CGO's
        // C-style export signature (no `const` qualifier) — Go does not
        // mutate the buffer.
        let rgba_len: u32 = match update.data.len().try_into() {
            Ok(n) => n,
            Err(_) => {
                warn!("[teleport][egfx] bitmap_updated: rgba length exceeds u32 max");
                return;
            }
        };
        let err = unsafe {
            cgo_handle_egfx_bitmap(
                self.cgo_handle,
                desktop_x,
                desktop_y,
                update.width.into(),
                update.height.into(),
                update.data.as_ptr() as *mut u8,
                rgba_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!("[teleport][egfx] cgo_handle_egfx_bitmap returned {:?}", err);
        }
    }

    fn on_frame_complete(&mut self, frame_id: u32) {
        // Forward the frame boundary so the client presents only complete
        // frames. Without this the client falls back to presenting per
        // wire-burst, uploading half-applied frames (bg fill before content)
        // — the black-rectangle flicker.
        let err = unsafe { cgo_handle_egfx_end_frame(self.cgo_handle, frame_id) };
        // DIAGNOSTIC (revert to a single debug! line): info-level marker proves
        // the NEW Rust is linked AND that on_frame_complete actually fires and
        // forwards. `forward=ErrCodeSuccess` => the EndFrame reached the Go
        // layer; if this line is absent entirely, on_frame_complete isn't being
        // called (no EndFrame from Windows / handler not dispatched).
        if frame_id < 5 || frame_id % 120 == 0 {
            info!(
                "[teleport][egfx][endframe-fwd-v1] frame_complete id={} forward={:?}",
                frame_id, err
            );
        }
    }

    fn on_solid_fill(&mut self, pdu: &SolidFillPdu) {
        self.forward_solid_fill(pdu);
    }

    fn on_surface_to_surface(&mut self, pdu: &SurfaceToSurfacePdu) {
        self.forward_surface_to_surface(pdu);
    }

    fn on_surface_to_cache(&mut self, pdu: &SurfaceToCachePdu) {
        self.forward_surface_to_cache(pdu);
    }

    fn on_cache_to_surface(&mut self, pdu: &CacheToSurfacePdu) {
        self.forward_cache_to_surface(pdu);
    }

    fn on_cache_import_reply(&mut self, pdu: &ironrdp_egfx::pdu::CacheImportReplyPdu) {
        // Server-side response to a client-issued CacheImportOffer. We
        // only send the offer when we have ≥1 persistent cache entry to
        // offer (FreeRDP convention; Windows rejects offers with 0
        // entries). When we add IndexedDB persistence and offer real
        // keys, this is where we load the accepted entries back into
        // the bitmap_cache.
        info!(
            "[teleport][egfx] cache_import_reply: {} cache_slots accepted",
            pdu.cache_slots.len()
        );
    }

    fn on_evict_cache_entry(&mut self, pdu: &ironrdp_egfx::pdu::EvictCacheEntryPdu) {
        self.forward_evict_cache_entry(pdu);
    }

    fn on_unhandled_pdu(&mut self, pdu: &ironrdp_egfx::pdu::GfxPdu) {
        use ironrdp_egfx::pdu::GfxPdu;
        match pdu {
            GfxPdu::WireToSurface1(p) => match p.codec_id {
                Codec1Type::Avc444 | Codec1Type::Avc444v2 => {
                    note_avc(p.codec_id);
                    self.forward_avc_frame(p)
                }
                Codec1Type::ClearCodec => self.forward_clearcodec_frame(p),
                Codec1Type::Uncompressed => self.forward_uncompressed_frame(p),
                Codec1Type::Planar => self.forward_planar_frame(p),
                Codec1Type::Avc420 => {
                    note_avc(p.codec_id);
                    self.forward_avc420_frame(p)
                }
                _ => warn!(
                    "[teleport][egfx] UNHANDLED WireToSurface1: surface_id={} codec={:?}",
                    p.surface_id, p.codec_id
                ),
            },
            other => warn!("[teleport][egfx] unhandled PDU: {:?}", other),
        }
    }

    fn on_wire_to_surface2(&mut self, pdu: &ironrdp_egfx::pdu::WireToSurface2Pdu) {
        self.forward_progressive_frame(pdu);
    }

    fn on_delete_encoding_context(&mut self, pdu: &ironrdp_egfx::pdu::DeleteEncodingContextPdu) {
        self.forward_delete_encoding_context(pdu);
    }

    // RAIL window-mapping variants are still routed to the IronRDP defaults
    // (no-op). Standard desktop sessions don't use them; if we start
    // supporting RemoteApp we'll need to wire these up.
    fn on_map_surface_to_window(&mut self, pdu: &ironrdp_egfx::pdu::MapSurfaceToWindowPdu) {
        warn!(
            "[teleport][egfx][SILENT-DROP] MapSurfaceToWindow: surface_id={} window_id={}",
            pdu.surface_id, pdu.window_id,
        );
    }

    fn on_map_surface_to_scaled_window(
        &mut self,
        pdu: &ironrdp_egfx::pdu::MapSurfaceToScaledWindowPdu,
    ) {
        warn!(
            "[teleport][egfx][SILENT-DROP] MapSurfaceToScaledWindow: surface_id={} window_id={}",
            pdu.surface_id, pdu.window_id,
        );
    }
}

impl TeleportEgfxHandler {
    /// Parse an AVC444/v2 WireToSurface1 PDU and forward the inner H.264
    /// streams to Go for browser-side decode. We don't decode H.264 here;
    /// the wasm client uses the browser's WebCodecs VideoDecoder for that.
    fn forward_avc_frame(&self, p: &ironrdp_egfx::pdu::WireToSurface1Pdu) {
        let mut cursor = ReadCursor::new(&p.bitmap_data);
        let stream = match Avc444BitmapStream::decode(&mut cursor) {
            Ok(s) => s,
            Err(e) => {
                warn!(
                    "[teleport][egfx][avc444] PDU decode failed: surface_id={} codec={:?} err={:?}",
                    p.surface_id, p.codec_id, e
                );
                return;
            }
        };

        let (origin_x, origin_y) = self
            .surface_origins
            .get(&p.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let desktop_x = origin_x.saturating_add(p.destination_rectangle.left.into());
        let desktop_y = origin_y.saturating_add(p.destination_rectangle.top.into());
        let dest_w: u32 = p.destination_rectangle.width().into();
        let dest_h: u32 = p.destination_rectangle.height().into();

        // Encoding flag determines which sub-stream the bytes belong to.
        // LUMA_AND_CHROMA = 0x00, LUMA = 0x01, CHROMA = 0x02.
        let encoding_bits: u32 = stream.encoding.bits().into();
        let codec_u32: u32 = u16::from(p.codec_id).into();

        // Per [MS-RDPEGFX] 2.2.4.4 / 2.2.4.4.1: when encoding == CHROMA,
        // `stream1` holds the chroma data (luma stream is absent).
        // Otherwise `stream1` is the luma stream and `stream2` (if present)
        // is the chroma stream.
        let (luma, chroma): (&[u8], &[u8]) = match stream.encoding {
            Encoding::CHROMA => (&[], stream.stream1.data),
            _ => (
                stream.stream1.data,
                stream.stream2.as_ref().map(|s| s.data).unwrap_or(&[]),
            ),
        };

        // DIAGNOSTIC (revert): per-frame AVC444 sub-stream tally + raw streamInfo
        // decode. `Avc444BitmapStream::decode` (avc.rs:260-266) splits at the
        // declared stream1 length and DISCARDS stream2 unless LC==LUMA_AND_CHROMA,
        // so to tell "host emits no aux" from "parser dropped an aux that's on the
        // wire" we manually decode the 32-bit streamInfo: bits 0..30 = stream1 byte
        // length, bits 30..32 = LC. `trailing` = bytes after stream1 = a stream2 the
        // LUMA path discarded. trailing≈0 ⇒ genuine luma-only (host emits no 4:4:4,
        // soft text is fundamental); trailing≫0 ⇒ the aux IS on the wire and the
        // parser dropped it (LC/parse issue → recombination viable). `raw_lc` should
        // equal `parser_lc`; if not, the u32 read disagrees with the wire.
        {
            use std::sync::atomic::{AtomicU64, Ordering};
            static N: AtomicU64 = AtomicU64::new(0);
            let n = N.fetch_add(1, Ordering::Relaxed);
            if n == 0 || n % 120 == 0 {
                let (raw_info, raw_lc, raw_s1len) = if p.bitmap_data.len() >= 4 {
                    let info = u32::from_le_bytes([
                        p.bitmap_data[0],
                        p.bitmap_data[1],
                        p.bitmap_data[2],
                        p.bitmap_data[3],
                    ]);
                    (info, info >> 30, (info & 0x3FFF_FFFF) as usize)
                } else {
                    (0u32, 9u32, 0usize)
                };
                let trailing = if raw_s1len == 0 {
                    0
                } else {
                    p.bitmap_data.len().saturating_sub(4 + raw_s1len)
                };
                info!(
                    "[teleport][egfx][avc444-streams] frame#{n} codec={codec_u32:#x} parser_lc={encoding_bits} \
                     streamInfo={raw_info:#010x} raw_lc={raw_lc} raw_stream1len={raw_s1len} trailing={trailing}B \
                     rects={} h264={}B stream2={} pdu={}B",
                    stream.stream1.rectangles.len(),
                    stream.stream1.data.len(),
                    stream
                        .stream2
                        .as_ref()
                        .map(|s| format!("{}B", s.data.len()))
                        .unwrap_or_else(|| "absent".to_owned()),
                    p.bitmap_data.len(),
                );
            }
        }

        // CGO export signature uses *mut u8 with no const qualifier; Go
        // copies into a Go-owned slice before any mutation could matter.
        let luma_len: u32 = luma.len().try_into().unwrap_or(u32::MAX);
        let chroma_len: u32 = chroma.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_avc_frame(
                self.cgo_handle,
                p.surface_id.into(),
                desktop_x,
                desktop_y,
                dest_w,
                dest_h,
                codec_u32,
                encoding_bits,
                luma.as_ptr() as *mut u8,
                luma_len,
                chroma.as_ptr() as *mut u8,
                chroma_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][avc444] cgo_handle_egfx_avc_frame returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX SolidFill PDU ([MS-RDPEGFX] 2.2.2.4) to the wasm
    /// client. The wasm side maintains its own framebuffer image and applies
    /// the fill directly there.
    fn forward_solid_fill(&mut self, pdu: &SolidFillPdu) {
        // Translate the fill rects from surface-relative to desktop coords by
        // the surface's MapSurfaceToOutput origin — the wasm framebuffer is a
        // single virtual-desktop image and the paint ops (ClearCodec/RFX/…)
        // already speak desktop coords, so fills must too.
        let (ox, oy) = self
            .surface_origins
            .get(&pdu.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let raw = pdu
            .rectangles
            .first()
            .map(|r| (u32::from(r.left), u32::from(r.top)))
            .unwrap_or((0, 0));
        self.maybe_log_origin("solid_fill", pdu.surface_id, (ox, oy), raw);
        // Each rect: 4 × u32 (left, top, right, bottom) — packed into one
        // flat buffer the cgo side can iterate without a per-rect allocation
        // dance. Empty rectangles list is a no-op (cgo handler short-circuits).
        let mut rects: Vec<u32> = Vec::with_capacity(pdu.rectangles.len() * 4);
        for r in &pdu.rectangles {
            rects.push(ox.saturating_add(u32::from(r.left)));
            rects.push(oy.saturating_add(u32::from(r.top)));
            rects.push(ox.saturating_add(u32::from(r.right)));
            rects.push(oy.saturating_add(u32::from(r.bottom)));
        }
        let rect_count: u32 = pdu.rectangles.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_solid_fill(
                self.cgo_handle,
                u32::from(pdu.surface_id),
                u32::from(pdu.fill_pixel.b),
                u32::from(pdu.fill_pixel.g),
                u32::from(pdu.fill_pixel.r),
                rect_count,
                rects.as_ptr() as *mut u32,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] cgo_handle_egfx_solid_fill returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX SurfaceToCache PDU ([MS-RDPEGFX] 2.2.2.6).
    fn forward_surface_to_cache(&mut self, pdu: &SurfaceToCachePdu) {
        // The source rect is surface-relative; the wasm framebuffer snapshots
        // from the single desktop image, so translate by the source surface's
        // origin to read the region the paint ops actually wrote.
        let (ox, oy) = self
            .surface_origins
            .get(&pdu.surface_id)
            .copied()
            .unwrap_or((0, 0));
        self.maybe_log_origin(
            "surface_to_cache",
            pdu.surface_id,
            (ox, oy),
            (
                u32::from(pdu.source_rectangle.left),
                u32::from(pdu.source_rectangle.top),
            ),
        );
        let err = unsafe {
            cgo_handle_egfx_surface_to_cache(
                self.cgo_handle,
                u32::from(pdu.surface_id),
                pdu.cache_key,
                u32::from(pdu.cache_slot),
                ox.saturating_add(u32::from(pdu.source_rectangle.left)),
                oy.saturating_add(u32::from(pdu.source_rectangle.top)),
                ox.saturating_add(u32::from(pdu.source_rectangle.right)),
                oy.saturating_add(u32::from(pdu.source_rectangle.bottom)),
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] cgo_handle_egfx_surface_to_cache returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX CacheToSurface PDU ([MS-RDPEGFX] 2.2.2.7). The
    /// destination points are packed as alternating u32 (x, y) pairs.
    fn forward_cache_to_surface(&mut self, pdu: &CacheToSurfacePdu) {
        // Destination points are surface-relative; translate by the
        // destination surface's origin so the cached tile is stamped at the
        // correct desktop location (this is the op that dominates the wire,
        // so getting its origin wrong blacks out / smears most of the screen).
        let (ox, oy) = self
            .surface_origins
            .get(&pdu.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let raw = pdu
            .destination_points
            .first()
            .map(|p| (u32::from(p.x), u32::from(p.y)))
            .unwrap_or((0, 0));
        self.maybe_log_origin("cache_to_surface", pdu.surface_id, (ox, oy), raw);
        let mut points: Vec<u32> = Vec::with_capacity(pdu.destination_points.len() * 2);
        for p in &pdu.destination_points {
            points.push(ox.saturating_add(u32::from(p.x)));
            points.push(oy.saturating_add(u32::from(p.y)));
        }
        let point_count: u32 = pdu.destination_points.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_cache_to_surface(
                self.cgo_handle,
                u32::from(pdu.surface_id),
                u32::from(pdu.cache_slot),
                point_count,
                points.as_ptr() as *mut u32,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] cgo_handle_egfx_cache_to_surface returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX SurfaceToSurface PDU ([MS-RDPEGFX] 2.2.2.5).
    fn forward_surface_to_surface(&mut self, pdu: &SurfaceToSurfacePdu) {
        // Source rect and destination points are each surface-relative to
        // their own surface — and source/destination can be DIFFERENT
        // surfaces with different origins, which is exactly why a single-image
        // client needs both translated independently.
        let (sox, soy) = self
            .surface_origins
            .get(&pdu.source_surface_id)
            .copied()
            .unwrap_or((0, 0));
        let (dox, doy) = self
            .surface_origins
            .get(&pdu.destination_surface_id)
            .copied()
            .unwrap_or((0, 0));
        self.maybe_log_origin(
            "surface_to_surface.src",
            pdu.source_surface_id,
            (sox, soy),
            (
                u32::from(pdu.source_rectangle.left),
                u32::from(pdu.source_rectangle.top),
            ),
        );
        let mut points: Vec<u32> = Vec::with_capacity(pdu.destination_points.len() * 2);
        for p in &pdu.destination_points {
            points.push(dox.saturating_add(u32::from(p.x)));
            points.push(doy.saturating_add(u32::from(p.y)));
        }
        let point_count: u32 = pdu.destination_points.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_surface_to_surface(
                self.cgo_handle,
                u32::from(pdu.source_surface_id),
                u32::from(pdu.destination_surface_id),
                sox.saturating_add(u32::from(pdu.source_rectangle.left)),
                soy.saturating_add(u32::from(pdu.source_rectangle.top)),
                sox.saturating_add(u32::from(pdu.source_rectangle.right)),
                soy.saturating_add(u32::from(pdu.source_rectangle.bottom)),
                point_count,
                points.as_ptr() as *mut u32,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] cgo_handle_egfx_surface_to_surface returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX EvictCacheEntry PDU ([MS-RDPEGFX] 2.2.2.8).
    fn forward_evict_cache_entry(&self, pdu: &ironrdp_egfx::pdu::EvictCacheEntryPdu) {
        let err = unsafe {
            cgo_handle_egfx_evict_cache_entry(self.cgo_handle, u32::from(pdu.cache_slot))
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx] cgo_handle_egfx_evict_cache_entry returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX WireToSurface2 PDU (RFX Progressive) to the wasm
    /// client untouched. Decoding lives in the wasm crate (`session::
    /// progressive`) so the per-tile sub-band state and per-(surface,
    /// codec_context_id) decoder state can write directly into the
    /// framebuffer without round-tripping decoded RGBA across the cgo /
    /// websocket boundary.
    fn forward_progressive_frame(&self, pdu: &ironrdp_egfx::pdu::WireToSurface2Pdu) {
        let (origin_x, origin_y) = self
            .surface_origins
            .get(&pdu.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let codec_id: u32 = u16::from(pdu.codec_id).into();
        let pixel_format: u32 = u8::from(pdu.pixel_format).into();
        let data_len: u32 = pdu.bitmap_data.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_wire_to_surface2(
                self.cgo_handle,
                u32::from(pdu.surface_id),
                codec_id,
                pdu.codec_context_id,
                pixel_format,
                origin_x,
                origin_y,
                pdu.bitmap_data.as_ptr() as *mut u8,
                data_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][rfx-progressive] cgo_handle_egfx_wire_to_surface2 returned {:?}",
                err
            );
        }
    }

    /// Forward an EGFX DeleteEncodingContext PDU to the wasm client. Pairs
    /// with `forward_progressive_frame` — the wasm decoder evicts the
    /// per-(surface, codec_context_id) state on receipt.
    fn forward_delete_encoding_context(&self, pdu: &ironrdp_egfx::pdu::DeleteEncodingContextPdu) {
        let err = unsafe {
            cgo_handle_egfx_delete_encoding_context(
                self.cgo_handle,
                u32::from(pdu.surface_id),
                pdu.codec_context_id,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][rfx-progressive] cgo_handle_egfx_delete_encoding_context returned {:?}",
                err
            );
        }
    }

    /// Forward a ClearCodec WireToSurface1 PDU to the wasm client untouched.
    /// The decoder lives in the session crate (wasm side) so it can write
    /// in-place into the framebuffer and preserve existing pixels for PDUs
    /// whose `residual_bytes==0` paints only a sub-region of the destination
    /// rectangle. Decoding here and shipping a full destination-rect RGBA
    /// buffer (as the previous path did) overwrites un-painted pixels with
    /// opaque black, producing visible black-tile artifacts in icons / text.
    fn forward_clearcodec_frame(&self, p: &ironrdp_egfx::pdu::WireToSurface1Pdu) {
        let dest_w = p.destination_rectangle.width();
        let dest_h = p.destination_rectangle.height();
        let (origin_x, origin_y) = self
            .surface_origins
            .get(&p.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let desktop_x: i32 = origin_x
            .saturating_add(p.destination_rectangle.left.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let desktop_y: i32 = origin_y
            .saturating_add(p.destination_rectangle.top.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let pdu_len: u32 = p.bitmap_data.len().try_into().unwrap_or(u32::MAX);

        let err = unsafe {
            cgo_handle_egfx_clearcodec(
                self.cgo_handle,
                u32::from(p.surface_id),
                desktop_x,
                desktop_y,
                u32::from(dest_w),
                u32::from(dest_h),
                p.bitmap_data.as_ptr() as *mut u8,
                pdu_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][clearcodec] cgo_handle_egfx_clearcodec returned {:?}",
                err
            );
        }
    }

    /// Forward a `Codec1Type::Planar` (codec_id 0x0a) WireToSurface1 PDU
    /// verbatim. RDP 6.0 bitmap stream — wasm decodes via
    /// `ironrdp_graphics::rdp6::bitmap_stream`.
    fn forward_planar_frame(&self, p: &ironrdp_egfx::pdu::WireToSurface1Pdu) {
        let dest_w = p.destination_rectangle.width();
        let dest_h = p.destination_rectangle.height();
        if dest_w == 0 || dest_h == 0 {
            return;
        }
        let (origin_x, origin_y) = self
            .surface_origins
            .get(&p.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let desktop_x: i32 = origin_x
            .saturating_add(p.destination_rectangle.left.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let desktop_y: i32 = origin_y
            .saturating_add(p.destination_rectangle.top.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let pdu_len: u32 = p.bitmap_data.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_planar(
                self.cgo_handle,
                u32::from(p.surface_id),
                desktop_x,
                desktop_y,
                u32::from(dest_w),
                u32::from(dest_h),
                p.bitmap_data.as_ptr() as *mut u8,
                pdu_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][planar] cgo_handle_egfx_planar returned {:?}",
                err
            );
        }
    }

    /// Forward a `Codec1Type::Avc420` (codec_id 0x0b) WireToSurface1 PDU
    /// verbatim. The `Avc420EncapsulatedBitmapStream` envelope and the
    /// H.264 bytes inside are decoded entirely on the wasm side (browser
    /// `VideoDecoder`).
    fn forward_avc420_frame(&self, p: &ironrdp_egfx::pdu::WireToSurface1Pdu) {
        let dest_w = p.destination_rectangle.width();
        let dest_h = p.destination_rectangle.height();
        if dest_w == 0 || dest_h == 0 {
            return;
        }
        let (origin_x, origin_y) = self
            .surface_origins
            .get(&p.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let desktop_x: i32 = origin_x
            .saturating_add(p.destination_rectangle.left.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let desktop_y: i32 = origin_y
            .saturating_add(p.destination_rectangle.top.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let pdu_len: u32 = p.bitmap_data.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_avc420(
                self.cgo_handle,
                u32::from(p.surface_id),
                desktop_x,
                desktop_y,
                u32::from(dest_w),
                u32::from(dest_h),
                p.bitmap_data.as_ptr() as *mut u8,
                pdu_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][avc420] cgo_handle_egfx_avc420 returned {:?}",
                err
            );
        }
    }

    /// Forward an `Uncompressed` (Codec1Type::Uncompressed, codec_id 0x0)
    /// WireToSurface1 PDU verbatim to wasm. Windows uses this for small UI
    /// overlays — tooltips, hover shadows, popup chrome — where the
    /// per-frame setup cost of a compressed codec outweighs the bandwidth
    /// savings, and frequently with an alpha channel (PIXEL_FORMAT_ARGB).
    ///
    /// No byte reordering or alpha handling on the server side; the wasm
    /// framebuffer is the only place that owns the destination buffer, so
    /// it's the only place that should do source-over compositing.
    fn forward_uncompressed_frame(&self, p: &ironrdp_egfx::pdu::WireToSurface1Pdu) {
        let dest_w = p.destination_rectangle.width();
        let dest_h = p.destination_rectangle.height();
        if dest_w == 0 || dest_h == 0 {
            return;
        }
        let (origin_x, origin_y) = self
            .surface_origins
            .get(&p.surface_id)
            .copied()
            .unwrap_or((0, 0));
        let desktop_x: i32 = origin_x
            .saturating_add(p.destination_rectangle.left.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let desktop_y: i32 = origin_y
            .saturating_add(p.destination_rectangle.top.into())
            .try_into()
            .unwrap_or(i32::MAX);
        let pixel_format: u32 = u8::from(p.pixel_format).into();
        let pdu_len: u32 = p.bitmap_data.len().try_into().unwrap_or(u32::MAX);
        let err = unsafe {
            cgo_handle_egfx_uncompressed(
                self.cgo_handle,
                u32::from(p.surface_id),
                desktop_x,
                desktop_y,
                u32::from(dest_w),
                u32::from(dest_h),
                pixel_format,
                p.bitmap_data.as_ptr() as *mut u8,
                pdu_len,
            )
        };
        if !matches!(err, CGOErrCode::ErrCodeSuccess) {
            warn!(
                "[teleport][egfx][uncompressed] cgo_handle_egfx_uncompressed returned {:?}",
                err
            );
        }
    }
}
