//! Decode-only EGFX codec object for the Step 2 dedicated decode worker.
//!
//! Holds the stateful RFX-Progressive + ClearCodec decoders (moved off the
//! `Framebuffer`, which keeps the image + GL on the canvas worker) and turns
//! encoded EGFX PDUs into pixel buffers the canvas worker blits. No GL, no
//! framebuffer image — this object runs in a SECOND wasm instance whose only
//! job is decode, so its results cross to the canvas worker via `postMessage`
//! transfer (no SharedArrayBuffer; see RFX_POOL_STEP2_PLAN.md).
//!
//! Decode order matters: both decoders are stateful and prior-frame-relative
//! (RFX difference/upgrade chains keyed per `codec_context_id`; ClearCodec
//! glyph/V-bar caches connection-scoped), so PDUs for a surface must be fed in
//! wire order. The single decode worker processes its message queue FIFO, which
//! preserves that order.
//!
//! Wire format, decode worker -> canvas worker (all little-endian):
//! - ClearCodec: the raw decoded BGRA buffer. The dest rect is echoed by the
//!   dispatcher (it sent dest_x/y/w/h in the request), so it isn't in the blob.
//! - RFX WireToSurface2: a self-describing tile blob =
//!     u32 tile_count
//!     tile_count × { u16 x_idx, u16 y_idx, u16 clip_count,
//!                    clip_count × { u16 x, u16 y, u16 width, u16 height } }
//!     tile_count × 16384 bytes RGBA   (tile i's pixels at rgba_start + i*16384)
//!   `Framebuffer::blit_rfx_blob` parses this and composites with the same
//!   clip math as the inline `blit_rfx_tiles`.

use ironrdp_graphics::clearcodec::ClearCodecDecoder;
use ironrdp_graphics::progressive::{DecodedTile, ProgressiveDecoder, TilePartition};
use wasm_bindgen::prelude::*;

/// Bytes in one decoded 64x64 RGBA tile (matches `progressive::TILE_PIXELS_LEN`).
const TILE_PIXELS_LEN: usize = 64 * 64 * 4;

// Per-worker accumulator of RFX per-stage decode time (feature `rfx-stage-timing`),
// surfaced to the host's `L.perf()` via `take_rfx_stage_timings`. This measures the
// entropy:IDWT:color split on the OFFLOAD path — the inline path's timers in
// `framebuffer::apply_rfx_progressive` never run when decode is pooled, which is
// why the `[apply] rfx{...}` line reads zero during normal (offloaded) operation.
// Cell of (entropy_ms, idwt_ms, color_ms, tiles).
#[cfg(feature = "rfx-stage-timing")]
thread_local! {
    static RFX_STAGE_ACCUM: std::cell::Cell<(f64, f64, f64, u32)> =
        const { std::cell::Cell::new((0.0, 0.0, 0.0, 0)) };
    static RFX_STAGE_HOOK_SET: std::cell::Cell<bool> = const { std::cell::Cell::new(false) };
}

/// Decode-only owner of the stateful EGFX codec decoders. Constructed once per
/// decode worker; fed EGFX PDUs in wire order; returns pixel buffers/blobs.
#[wasm_bindgen]
pub struct EgfxDecoder {
    /// Dims of the surface the NEXT decode targets, handed off per-chunk via
    /// [`Self::set_surface`]. Tile grids are PER SURFACE: the reset-on-resize
    /// decision lives in [`Self::decode_wire_to_surface2`], scoped to the one
    /// surface whose dims changed.
    width: u16,
    height: u16,
    /// One shared ClearCodec decoder for the whole connection (caches are
    /// connection-scoped — see `Framebuffer::clearcodec_decoder`).
    clearcodec_decoder: ClearCodecDecoder,
    /// One stateful RFX-Progressive decoder per EGFX surface, tagged with the
    /// dims its tile grid was last decoded at.
    progressive_decoders: Vec<(u32, SurfaceState)>,
}

/// Per-surface decode state: the stateful RFX-Progressive decoder plus the
/// surface dims its accumulators were built for.
struct SurfaceState {
    width: u16,
    height: u16,
    decoder: ProgressiveDecoder,
}

#[wasm_bindgen]
impl EgfxDecoder {
    #[wasm_bindgen(constructor)]
    #[must_use]
    pub fn new() -> Self {
        Self {
            width: 0,
            height: 0,
            clearcodec_decoder: ClearCodecDecoder::new(),
            progressive_decoders: Vec::new(),
        }
    }

    /// Record the dimensions of the surface the NEXT decode targets (called
    /// before every chunk). Purely a parameter hand-off: a dim change resets
    /// only the resized surface's decoder, in `decode_wire_to_surface2`.
    /// Interleaved chunks from differently-sized surfaces must NOT clear each
    /// other's accumulators — the old global clear here was the 3-monitor
    /// drag-corruption / missing-rect bug.
    #[wasm_bindgen(js_name = setSurface)]
    pub fn set_surface(&mut self, width: u16, height: u16) {
        self.width = width;
        self.height = height;
    }

    /// Decode a ClearCodec PDU into a BGRA buffer (the dispatcher pairs it with
    /// the dest rect it sent). `w`/`h` are the destination size.
    #[wasm_bindgen(js_name = decodeClearCodec)]
    pub fn decode_clearcodec(&mut self, w: u16, h: u16, pdu: &[u8]) -> Result<Vec<u8>, JsValue> {
        if w == 0 || h == 0 {
            return Ok(Vec::new());
        }
        self.clearcodec_decoder
            .decode(pdu, w, h)
            .map_err(|e| JsValue::from_str(&format!("clearcodec decode failed: {e}")))
    }

    /// Decode an RFX WireToSurface2 PDU into the tile blob (see module docs).
    /// `surface_id`/`codec_context_id` select the per-surface/-context decoder
    /// state; surface dims come from the last [`Self::set_surface`].
    /// Decode this worker's partition of an RFX WireToSurface2 PDU. The whole PDU
    /// is broadcast to every worker; `worker_index`/`num_workers` select which
    /// tiles THIS worker owns (stable position hash). `num_workers == 1` decodes
    /// every tile (the Step 2 single-worker case). Returns the tile blob for the
    /// owned subset (see module docs).
    #[wasm_bindgen(js_name = decodeWireToSurface2)]
    pub fn decode_wire_to_surface2(
        &mut self,
        surface_id: u32,
        codec_context_id: u32,
        worker_index: u32,
        num_workers: u32,
        pdu: &[u8],
    ) -> Result<Vec<u8>, JsValue> {
        let (width, height) = (self.width, self.height);
        let partition = (num_workers > 1)
            .then_some(TilePartition { worker_index, num_workers, salt: surface_id });
        // Zero ironrdp-graphics' per-decode stage accumulator before decoding so
        // `take()` below reads only this PDU's entropy/IDWT/color (offload-path
        // measurement — see RFX_STAGE_ACCUM). The clock hook is installed once.
        #[cfg(feature = "rfx-stage-timing")]
        {
            RFX_STAGE_HOOK_SET.with(|c| {
                if !c.get() {
                    ironrdp_graphics::progressive::stage_timing::set_now_hook(
                        crate::perf::performance_now,
                    );
                    c.set(true);
                }
            });
            ironrdp_graphics::progressive::stage_timing::reset();
        }
        let tiles = {
            let state = lookup_or_create(&mut self.progressive_decoders, surface_id, width, height);
            if state.width != width || state.height != height {
                // THIS surface was resized (monitor resize / layout reflow):
                // its accumulators no longer match the tile grid, so restart
                // them — mirrors `Framebuffer::resize_preserving_canvases`,
                // scoped to the one surface. Other surfaces keep their state.
                state.decoder.reset();
                state.width = width;
                state.height = height;
            }
            match state.decoder.decode_bitmap_partitioned(codec_context_id, width, height, pdu, partition) {
                Ok(t) => t,
                Err(e) => {
                    // A mid-frame decode error can leave this surface's tile
                    // accumulators PARTIALLY advanced while NO pixels were blitted
                    // (the frame is dropped). Future Difference/Upgrade passes would
                    // then difference against inconsistent state and the artifact
                    // would ACCUMULATE forever (never self-heal). Reset the
                    // per-surface decoder so the next frame starts from a clean
                    // baseline — it resyncs on the server's next keyframe / First
                    // pass instead of poisoning the chain.
                    state.decoder.reset();
                    return Err(JsValue::from_str(&format!("rfx-progressive decode failed: {e}")));
                }
            }
        };
        // Fold this PDU's per-stage decode time into the worker accumulator.
        #[cfg(feature = "rfx-stage-timing")]
        {
            let st = ironrdp_graphics::progressive::stage_timing::take();
            RFX_STAGE_ACCUM.with(|c| {
                let (e, i, co, n) = c.get();
                c.set((
                    e + st.entropy_ms,
                    i + st.idwt_ms,
                    co + st.color_ms,
                    n + tiles.len() as u32,
                ));
            });
        }
        let blob = encode_tile_blob(&tiles);
        // Recycle the tile pixel buffers locally: the decoder stays in THIS
        // worker, so its pixel pool survives across frames (the transferred blob
        // is a separate copy, detached on the canvas worker).
        lookup_or_create(&mut self.progressive_decoders, surface_id, width, height)
            .decoder
            .reclaim(tiles);
        Ok(blob)
    }

    /// Take and zero this worker's accumulated RFX per-stage decode time, as
    /// `[entropy_ms, idwt_ms, color_ms, tiles]`. The pool worker posts it to the
    /// host periodically so `L.perf()` shows where OFFLOADED decode spends time —
    /// the input to the SIMD-vs-GPU decode-offload decision. Zeros without the
    /// `rfx-stage-timing` feature.
    #[wasm_bindgen(js_name = takeRfxStageTimings)]
    #[must_use]
    pub fn take_rfx_stage_timings(&self) -> Vec<f64> {
        #[cfg(feature = "rfx-stage-timing")]
        {
            RFX_STAGE_ACCUM.with(|c| {
                let (e, i, co, n) = c.get();
                c.set((0.0, 0.0, 0.0, 0));
                vec![e, i, co, f64::from(n)]
            })
        }
        #[cfg(not(feature = "rfx-stage-timing"))]
        {
            vec![0.0, 0.0, 0.0, 0.0]
        }
    }

    /// Evict one `(surface_id, codec_context_id)` progressive context
    /// (EGFX `DeleteEncodingContext`). Must stay ordered with this surface's PDUs.
    ///
    /// `codec_context_id == u32::MAX` is the WHOLE-SURFACE sentinel: the
    /// server forwards it when Windows deletes the surface (lib/srv/desktop/
    /// rdp/rdpclient/src/egfx.rs `on_surface_deleted`). The surface's decoder
    /// is dropped entirely so a recreated surface with a reused id (common
    /// after the ResetGraphics from a monitor add/remove) starts clean
    /// instead of differencing against the dead surface's accumulators.
    #[wasm_bindgen(js_name = deleteContext)]
    pub fn delete_context(&mut self, surface_id: u32, codec_context_id: u32) {
        if codec_context_id == u32::MAX {
            self.progressive_decoders.retain(|(id, _)| *id != surface_id);
            return;
        }
        if let Some((_, state)) = self
            .progressive_decoders
            .iter_mut()
            .find(|(id, _)| *id == surface_id)
        {
            state.decoder.delete_context(codec_context_id);
        }
    }
}

impl Default for EgfxDecoder {
    fn default() -> Self {
        Self::new()
    }
}

/// Linear lookup / lazy-create of the per-surface decode state (mirrors
/// `framebuffer::lookup_or_create_progressive`; N is tiny, ≤ 4 surfaces).
/// New entries are created at the given dims, so a fresh surface needs no
/// reset on its first decode.
fn lookup_or_create(
    decoders: &mut Vec<(u32, SurfaceState)>,
    surface_id: u32,
    width: u16,
    height: u16,
) -> &mut SurfaceState {
    let idx = match decoders.iter().position(|(id, _)| *id == surface_id) {
        Some(i) => i,
        None => {
            decoders.push((
                surface_id,
                SurfaceState { width, height, decoder: ProgressiveDecoder::new() },
            ));
            decoders.len() - 1
        }
    };
    &mut decoders[idx].1
}

/// Serialize decoded tiles into the self-describing blob documented above.
fn encode_tile_blob(tiles: &[DecodedTile]) -> Vec<u8> {
    let header_len = 4 + tiles.iter().map(|t| 6 + t.clip_rects.len() * 8).sum::<usize>();
    let total = header_len + tiles.len() * TILE_PIXELS_LEN;
    let mut blob = Vec::with_capacity(total);
    blob.extend_from_slice(&(tiles.len() as u32).to_le_bytes());
    for t in tiles {
        blob.extend_from_slice(&t.x_idx.to_le_bytes());
        blob.extend_from_slice(&t.y_idx.to_le_bytes());
        blob.extend_from_slice(&(t.clip_rects.len() as u16).to_le_bytes());
        for r in &t.clip_rects {
            blob.extend_from_slice(&r.x.to_le_bytes());
            blob.extend_from_slice(&r.y.to_le_bytes());
            blob.extend_from_slice(&r.width.to_le_bytes());
            blob.extend_from_slice(&r.height.to_le_bytes());
        }
    }
    for t in tiles {
        debug_assert_eq!(t.pixels.len(), TILE_PIXELS_LEN, "tile pixels must be 64x64 RGBA");
        blob.extend_from_slice(&t.pixels);
    }
    blob
}

#[cfg(test)]
mod tests {
    use super::*;

    /// `deleteContext` with the whole-surface sentinel (ctx = u32::MAX,
    /// forwarded by the server when Windows DELETES a surface) must drop that
    /// surface's decoder entirely — a recreated surface with a reused id and
    /// unchanged dims would otherwise decode its fresh RFX stream against the
    /// dead surface's stale accumulators (corrupt rects after adding a
    /// monitor) — while leaving other surfaces' state untouched.
    #[test]
    fn delete_context_sentinel_drops_only_that_surface() {
        let mut egfx = EgfxDecoder::new();
        egfx.set_surface(1024, 768);
        lookup_or_create(&mut egfx.progressive_decoders, 1, 1024, 768);
        lookup_or_create(&mut egfx.progressive_decoders, 2, 1024, 768);
        egfx.delete_context(1, u32::MAX);
        assert_eq!(egfx.progressive_decoders.len(), 1);
        assert_eq!(egfx.progressive_decoders[0].0, 2);
    }

    /// Interleaved chunks from differently-sized surfaces (the 3-monitor
    /// session shape: main window + popups with unequal inner sizes) must not
    /// wipe each other's RFX-Progressive accumulators — Difference/Upgrade
    /// passes decode against them, so a wipe shows up as window-drag border
    /// corruption and permanently missing rects (decode errors are replied
    /// bloblessly).
    #[test]
    fn dim_change_for_one_surface_keeps_other_surfaces_decoders() {
        let mut egfx = EgfxDecoder::new();
        // Two surfaces materialized at 1024x768, as the decode path would.
        egfx.set_surface(1024, 768);
        lookup_or_create(&mut egfx.progressive_decoders, 1, 1024, 768);
        lookup_or_create(&mut egfx.progressive_decoders, 2, 1024, 768);
        // A chunk for a differently-sized surface arrives.
        egfx.set_surface(1280, 800);
        assert_eq!(
            egfx.progressive_decoders.len(),
            2,
            "a surface-dim change must not clear other surfaces' decoder state"
        );
    }
}
