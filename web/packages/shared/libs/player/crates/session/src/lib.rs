//! wasm-bindgen entry points.
//!
//! * [`MainCodec`] is the per-thread encoder used by anything that needs to
//!   produce outbound TDP/TDPB bytes (main thread for mouse/keyboard, the
//!   SharedSessionWorker for wrapping `RdpResponse`).
//! * [`WorkerDecoder`] owns the IronRDP processor and one [`Framebuffer`]
//!   that drives N registered canvases. Each call to `add_canvas` registers
//!   one viewport-shaped slice of the virtual desktop; all canvases share
//!   the same decoded image so there's no decode duplication when the
//!   session has multiple monitors.
//!
//! `WorkerDecoder` is host-scope agnostic: callers pass a JS function at
//! construction that the decoder invokes to deliver outbound events
//! (`rdpResponse`, `cursorBitmap`, etc.). This lets the same wasm binary
//! run in either a dedicated worker or a shared worker scope.

use std::cell::{Cell, RefCell};
use std::rc::Rc;

use bytes::Bytes;
use codec::incoming::InboundMessage;
use codec::messages::{
    ButtonState, ClientHello, ClipboardIn, KeyboardButton, MonitorLayout, MouseButton,
    MouseButtonKind, MouseMove, MouseWheel, RdpResponsePdu, RefreshRect, ScreenSpec, ScrollAxis,
    SyncKeys,
};
use codec::Codec;
use js_sys::{Function, Reflect};
use wasm_bindgen::closure::Closure;
use wasm_bindgen::prelude::*;
use wasm_bindgen::JsCast;
use web_sys::OffscreenCanvas;

mod egfx_decoder;
mod framebuffer;
mod gl;
mod paint_queue;
mod perf;
// Native integer references for the Phase-A GPU IDWT+color offload — the
// transliteration source for the WebGL2 shaders and their bit-exactness oracle.
#[cfg(test)]
mod gpu_ref;
// Host-only RFX Progressive vs FreeRDP oracle (test harness). Never part of the
// wasm bundle. Enable with `cargo test -p session --features freerdp-oracle`.
#[cfg(feature = "freerdp-oracle")]
mod progressive;

use framebuffer::{CursorEvent, Framebuffer, Viewport};
use paint_queue::{PaintQueue, PendingPaint};

// ── tracing setup ───────────────────────────────────────────────────────────

#[wasm_bindgen(js_name = initWasmLog)]
pub fn init_wasm_log(level: &str) {
    use tracing_subscriber::filter::LevelFilter;
    use tracing_subscriber::fmt::time::UtcTime;
    use tracing_subscriber::prelude::*;
    use tracing_web::MakeConsoleWriter;

    console_error_panic_hook::set_once();

    if let Ok(lvl) = level.parse::<tracing::Level>() {
        let fmt_layer = tracing_subscriber::fmt::layer()
            .with_ansi(false)
            .with_timer(UtcTime::rfc_3339())
            .with_writer(MakeConsoleWriter);

        let _ = tracing_subscriber::registry()
            .with(fmt_layer)
            .with(LevelFilter::from_level(lvl))
            .try_init();
    }
}

fn monitors_from_parallel_arrays(
    xs: &[i32],
    ys: &[i32],
    widths: &[u32],
    heights: &[u32],
    primaries: &[u8],
) -> Result<Vec<MonitorLayout>, JsError> {
    let n = xs.len();
    if n == 0 {
        return Err(JsError::new("monitors arrays are empty"));
    }
    if ys.len() != n || widths.len() != n || heights.len() != n || primaries.len() != n {
        return Err(JsError::new("monitors arrays have mismatched lengths"));
    }
    let mut primary_count = 0;
    let mut out = Vec::with_capacity(n);
    for i in 0..n {
        let is_primary = primaries[i] != 0;
        if is_primary {
            primary_count += 1;
        }
        out.push(MonitorLayout {
            x: xs[i],
            y: ys[i],
            width: widths[i],
            height: heights[i],
            is_primary,
        });
    }
    if primary_count != 1 {
        return Err(JsError::new(&format!(
            "exactly one primary monitor required, got {primary_count}"
        )));
    }
    Ok(out)
}

/// Map an inbound message to its perf category for the per-op time breakdown.
fn apply_category(m: &InboundMessage) -> perf::ApplyCat {
    use InboundMessage::*;
    match m {
        EgfxClearCodec(_) => perf::ApplyCat::Clear,
        EgfxCacheToSurface(_) => perf::ApplyCat::C2s,
        EgfxSurfaceToCache(_) => perf::ApplyCat::S2c,
        EgfxWireToSurface2(_) => perf::ApplyCat::Rfx,
        _ => perf::ApplyCat::Other,
    }
}

// ── MainCodec ──────────────────────────────────────────────────────────────

#[wasm_bindgen]
pub struct MainCodec {
    inner: RefCell<Codec>,
}

impl Default for MainCodec {
    fn default() -> Self {
        Self::new()
    }
}

#[wasm_bindgen]
impl MainCodec {
    #[wasm_bindgen(constructor)]
    #[must_use]
    pub fn new() -> Self {
        Self {
            inner: RefCell::new(Codec::new()),
        }
    }

    #[wasm_bindgen(js_name = upgradeToTdpb)]
    pub fn upgrade_to_tdpb(&self) {
        self.inner.borrow_mut().upgrade_to_tdpb();
    }

    #[wasm_bindgen(js_name = encodeScreenSpec)]
    pub fn encode_screen_spec(
        &self,
        width: u32,
        height: u32,
        scale: u32,
    ) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .screen_spec(&ScreenSpec {
                width,
                height,
                scale,
                monitors: Vec::new(),
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeScreenSpecMulti)]
    pub fn encode_screen_spec_multi(
        &self,
        width: u32,
        height: u32,
        scale: u32,
        xs: Vec<i32>,
        ys: Vec<i32>,
        widths: Vec<u32>,
        heights: Vec<u32>,
        primaries: Vec<u8>,
    ) -> Result<Vec<u8>, JsError> {
        let monitors = monitors_from_parallel_arrays(&xs, &ys, &widths, &heights, &primaries)?;
        self.inner
            .borrow()
            .screen_spec(&ScreenSpec {
                width,
                height,
                scale,
                monitors,
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeClientHello)]
    pub fn encode_client_hello(
        &self,
        username: String,
        width: u32,
        height: u32,
        scale: u32,
        keyboard_layout: u32,
    ) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .client_hello(&ClientHello {
                username,
                screen_spec: ScreenSpec {
                    width,
                    height,
                    scale,
                    monitors: Vec::new(),
                },
                keyboard_layout,
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeClientHelloMulti)]
    pub fn encode_client_hello_multi(
        &self,
        username: String,
        width: u32,
        height: u32,
        scale: u32,
        keyboard_layout: u32,
        xs: Vec<i32>,
        ys: Vec<i32>,
        widths: Vec<u32>,
        heights: Vec<u32>,
        primaries: Vec<u8>,
    ) -> Result<Vec<u8>, JsError> {
        let monitors = monitors_from_parallel_arrays(&xs, &ys, &widths, &heights, &primaries)?;
        self.inner
            .borrow()
            .client_hello(&ClientHello {
                username,
                screen_spec: ScreenSpec {
                    width,
                    height,
                    scale,
                    monitors,
                },
                keyboard_layout,
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeMouseMove)]
    pub fn encode_mouse_move(&self, x: u32, y: u32) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .mouse_move(&MouseMove { x, y })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeMouseButton)]
    pub fn encode_mouse_button(&self, button: u8, pressed: bool) -> Result<Vec<u8>, JsError> {
        let kind = match button {
            0 => MouseButtonKind::Left,
            1 => MouseButtonKind::Middle,
            2 => MouseButtonKind::Right,
            n => return Err(JsError::new(&format!("invalid mouse button: {n}"))),
        };
        self.inner
            .borrow()
            .mouse_button(&MouseButton {
                button: kind,
                pressed,
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    /// `axis`: 0 = vertical, 1 = horizontal (mirrors the TS `ScrollAxis` enum).
    /// `delta` is in CSS pixels, sign-matching the browser `WheelEvent` after
    /// the caller has negated it for remote scroll direction. TDPB carries the
    /// full i32; the legacy wire form clamps to i16 (and errors on overflow).
    #[wasm_bindgen(js_name = encodeMouseWheel)]
    pub fn encode_mouse_wheel(&self, axis: u8, delta: i32) -> Result<Vec<u8>, JsError> {
        let axis = match axis {
            0 => ScrollAxis::Vertical,
            1 => ScrollAxis::Horizontal,
            n => return Err(JsError::new(&format!("invalid scroll axis: {n}"))),
        };
        self.inner
            .borrow()
            .mouse_wheel(&MouseWheel { axis, delta })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeKey)]
    pub fn encode_key(&self, key_code: u32, pressed: bool) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .keyboard_button(&KeyboardButton { key_code, pressed })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    /// Lock-key states come from the browser's `getModifierState`. Beyond
    /// syncing the four lock keys, the resulting RDP Synchronize Event
    /// "reset[s] the server key state to all keys up" (MS-RDPBCGR
    /// TS_SYNC_EVENT), releasing any key the server still believes is held —
    /// the recovery path for a modifier latched by a keyup the page never saw
    /// (Cmd chords that move focus, a popup closing mid-press). Windows
    /// sessions keep held keys latched across RDP reconnects, so this must be
    /// sent before the first key event of a session as well as on refocus.
    #[wasm_bindgen(js_name = encodeSyncKeys)]
    pub fn encode_sync_keys(
        &self,
        scroll_lock: bool,
        num_lock: bool,
        caps_lock: bool,
        kana_lock: bool,
    ) -> Result<Vec<u8>, JsError> {
        let state = |pressed: bool| {
            if pressed {
                ButtonState::Down
            } else {
                ButtonState::Up
            }
        };
        self.inner
            .borrow()
            .sync_keys(&SyncKeys {
                scroll_lock: state(scroll_lock),
                num_lock: state(num_lock),
                caps_lock: state(caps_lock),
                kana_lock: state(kana_lock),
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeRdpResponse)]
    pub fn encode_rdp_response(&self, response: Vec<u8>) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .rdp_response(&RdpResponsePdu {
                response: Bytes::from(response),
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    /// Send the host clipboard contents to the server, so a paste in the
    /// remote session yields this text.
    #[wasm_bindgen(js_name = encodeClipboard)]
    pub fn encode_clipboard(&self, data: String) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .clipboard(&ClipboardIn { data })
            .map_err(|e| JsError::new(&format!("{e}")))
    }

    #[wasm_bindgen(js_name = encodeRefreshRect)]
    pub fn encode_refresh_rect(
        &self,
        left: u32,
        top: u32,
        right: u32,
        bottom: u32,
    ) -> Result<Vec<u8>, JsError> {
        self.inner
            .borrow()
            .refresh_rect(&RefreshRect {
                left,
                top,
                right,
                bottom,
            })
            .map_err(|e| JsError::new(&format!("{e}")))
    }
}

// ── WorkerDecoder ──────────────────────────────────────────────────────────

/// Pending canvas registration recorded before the framebuffer was created
/// (because ServerHello hasn't arrived yet). Drained when `init_framebuffer`
/// runs.
struct PendingCanvas {
    id: u32,
    canvas: OffscreenCanvas,
    viewport: Viewport,
}

/// DIAGNOSTIC (revert): inbound graphics-message tallies. Answers the
/// pivotal "is this session EGFX or legacy fast-path?" question — EGFX
/// non-AVC ops decode silently, so without counting them the logs only show
/// `FastPathPdu`. Summarized via `post_log` so it surfaces in the same
/// channel the harness already renders.
#[derive(Default)]
struct Traffic {
    gfx_total: u64,
    fastpath: u64,
    egfx_bitmap: u64,
    egfx_clearcodec: u64,
    egfx_uncompressed: u64,
    egfx_planar: u64,
    egfx_avc: u64,
    egfx_solidfill: u64,
    egfx_s2c: u64,
    egfx_c2s: u64,
    egfx_evict: u64,
    egfx_s2s: u64,
    egfx_rfx: u64,
    egfx_other: u64,
    png: u64,
    logged_first_egfx: bool,
}

impl Traffic {
    fn summary(&self) -> String {
        format!(
            "[traffic] gfx={} fastpath={} egfx{{bitmap={} clear={} uncomp={} planar={} avc={} solid={} s2c={} c2s={} evict={} s2s={} rfx={} other={}}} png={}",
            self.gfx_total, self.fastpath, self.egfx_bitmap, self.egfx_clearcodec,
            self.egfx_uncompressed, self.egfx_planar, self.egfx_avc, self.egfx_solidfill,
            self.egfx_s2c, self.egfx_c2s, self.egfx_evict, self.egfx_s2s, self.egfx_rfx,
            self.egfx_other, self.png,
        )
    }
}

struct DecoderInner {
    codec: Codec,
    fb: Option<Framebuffer>,
    pending: Vec<PendingCanvas>,
    /// JS callback invoked with one argument — a plain JS object describing
    /// the outbound event. The host (a dedicated worker or shared worker)
    /// chooses what to do with each event type.
    post: Function,
    /// Reused buffer for `blit_avc_rgba` so each decoded AVC frame doesn't
    /// allocate a fresh `Vec` to copy the JS pixels into.
    avc_scratch: Vec<u8>,
}

#[wasm_bindgen]
pub struct WorkerDecoder {
    inner: Rc<RefCell<DecoderInner>>,
    /// Coalescing gate for `flush_render`. Set true when a `setTimeout(0)`
    /// is in flight; cleared at the start of the timeout callback. Lets
    /// many `mark_dirty_pending` calls within one wire burst schedule
    /// exactly one GL upload at the end of the burst.
    flush_scheduled: Rc<Cell<bool>>,
    /// EGFX presents on the frame's EndFrame boundary, not per wire-burst.
    /// `egfx_present_owed` is true once a frame's content has been applied but
    /// not yet presented (cleared when the EndFrame presents it). Presenting
    /// per wire-burst instead uploads half-applied frames (the per-frame bg
    /// fill before content lands) — the black-rectangle flicker.
    egfx_present_owed: Cell<bool>,
    /// Latches once the server starts sending EndFrame markers. Older servers
    /// never send them; until we see one we keep the legacy per-burst flush so
    /// the screen still updates.
    seen_end_frame: Cell<bool>,
    /// When true, RFX-Progressive decode is offloaded to the decode-worker pool.
    /// `apply_egfx_wire_to_surface2` emits an `rfxChunk` event (decoded tiles
    /// return via `stageRfxBlob` + `rfxSeqReady`) and EVERY paint op — RFX and
    /// the inline ones — is routed through the wire-order `paint_queue` rather
    /// than applied immediately; present is driven by the queue's `Present`
    /// marker (so the inline per-burst flush is suppressed). Opt-in
    /// (`setEgfxOffload`) so the proven inline path stays a zero-risk fallback
    /// (the `?offload=0` A-B toggle).
    egfx_offload: Cell<bool>,
    /// Wire-order apply queue for offloaded EGFX paint ops (see `paint_queue`).
    /// In offload mode EVERY paint op (RFX + the inline SurfaceToSurface /
    /// SurfaceToCache / CacheToSurface / SolidFill / ClearCodec / Uncompressed /
    /// Planar / Bitmap) is enqueued in wire order and drained strictly in `seq`
    /// order, so an inline op never reads/writes the framebuffer ahead of a
    /// prior in-flight RFX blit (the window-drag corruption). Empty / unused
    /// when offload is off.
    paint_queue: RefCell<PaintQueue>,
    /// DIAGNOSTIC (revert): inbound graphics-message tallies.
    traffic: RefCell<Traffic>,
    /// One persistent flush closure reused for every coalesced `setTimeout(0)`,
    /// instead of allocating a fresh one-shot closure per flush.
    flush_cb: Closure<dyn FnMut()>,
}

#[wasm_bindgen]
impl WorkerDecoder {
    /// Construct the decoder. `post_message` is a JS function that will be
    /// invoked with `(eventObject)` for every outbound event the decoder
    /// produces — see the message-type tags emitted by the helpers below.
    #[wasm_bindgen(constructor)]
    pub fn new(post_message: Function) -> Result<WorkerDecoder, JsError> {
        // DIAGNOSTIC (revert): muted — logged the [build:egfx-endframe-present-v1]
        // build stamp on construction; un-comment to re-enable.
        // web_sys::console::warn_1(&"[build:egfx-endframe-present-v1]".into());
        let inner = Rc::new(RefCell::new(DecoderInner {
            codec: Codec::new(),
            fb: None,
            pending: Vec::new(),
            post: post_message,
            avc_scratch: Vec::new(),
        }));
        let flush_scheduled = Rc::new(Cell::new(false));

        // Build the flush closure once. Each coalesced `setTimeout(0)` reuses it
        // (see `maybe_schedule_flush`) rather than allocating a one-shot closure.
        let flush_cb = {
            let inner = inner.clone();
            let gate = flush_scheduled.clone();
            Closure::wrap(Box::new(move || {
                gate.set(false);
                let mut p = inner.borrow_mut();
                if let Some(fb) = p.fb.as_mut() {
                    if let Err(e) = fb.flush_render() {
                        web_sys::console::warn_1(&format!("flush_render error: {e}").into());
                    }
                }
            }) as Box<dyn FnMut()>)
        };

        Ok(Self {
            inner,
            flush_scheduled,
            egfx_present_owed: Cell::new(false),
            seen_end_frame: Cell::new(false),
            egfx_offload: Cell::new(false),
            paint_queue: RefCell::new(PaintQueue::new()),
            traffic: RefCell::new(Traffic::default()),
            flush_cb,
        })
    }

    /// Register a canvas to receive renders of the given viewport. The id is
    /// caller-assigned and used in later `removeCanvas` / `updateViewport`
    /// calls. Safe to call before ServerHello — the registration is queued
    /// and applied once the framebuffer is created.
    #[wasm_bindgen(js_name = addCanvas)]
    pub fn add_canvas(
        &self,
        id: u32,
        canvas: OffscreenCanvas,
        x: u32,
        y: u32,
        width: u32,
        height: u32,
    ) -> Result<(), JsError> {
        let viewport = Viewport {
            x,
            y,
            width,
            height,
        };
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.add_canvas(id, &canvas, viewport)
                .map_err(|e| JsError::new(&format!("{e}")))?;
        } else {
            p.pending.push(PendingCanvas {
                id,
                canvas,
                viewport,
            });
        }
        Ok(())
    }

    #[wasm_bindgen(js_name = removeCanvas)]
    pub fn remove_canvas(&self, id: u32) {
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.remove_canvas(id);
        }
        p.pending.retain(|pc| pc.id != id);
    }

    #[wasm_bindgen(js_name = updateViewport)]
    pub fn update_viewport(
        &self,
        id: u32,
        canvas: OffscreenCanvas,
        x: u32,
        y: u32,
        width: u32,
        height: u32,
    ) -> Result<(), JsError> {
        let viewport = Viewport {
            x,
            y,
            width,
            height,
        };
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.update_viewport(id, &canvas, viewport)
                .map_err(|e| JsError::new(&format!("{e}")))?;
        } else {
            // No fb yet — replace any pending entry with the same id.
            p.pending.retain(|pc| pc.id != id);
            p.pending.push(PendingCanvas {
                id,
                canvas,
                viewport,
            });
        }
        Ok(())
    }

    /// Re-point an already-registered canvas at a new viewport WITHOUT the
    /// `OffscreenCanvas` (the caller transferred it away and no longer holds
    /// it). Used when a popup monitor closes and the surviving monitors reflow
    /// into new desktop positions. No-op if `id` isn't registered.
    #[wasm_bindgen(js_name = repositionViewport)]
    pub fn reposition_viewport(
        &self,
        id: u32,
        x: u32,
        y: u32,
        width: u32,
        height: u32,
    ) -> Result<(), JsError> {
        let viewport = Viewport {
            x,
            y,
            width,
            height,
        };
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.reposition_canvas(id, viewport)
                .map_err(|e| JsError::new(&format!("{e}")))?;
        } else if let Some(pc) = p.pending.iter_mut().find(|pc| pc.id == id) {
            pc.viewport = viewport;
        }
        Ok(())
    }

    #[wasm_bindgen(js_name = feedBytes)]
    pub fn feed_bytes(&self, buffer: &js_sys::ArrayBuffer) {
        let bytes: Bytes = {
            let view = js_sys::Uint8Array::new(buffer);
            view.to_vec().into()
        };
        let busy_start = perf::performance_now();
        let decoded = {
            let _t = perf::Timer::new(perf::record_decode);
            self.inner.borrow().codec.decode(bytes)
        };
        let cat = match &decoded {
            Ok(m) => apply_category(m),
            Err(_) => perf::ApplyCat::Other,
        };
        match decoded {
            Ok(m) => self.handle_inbound(m),
            Err(e) => self.post_log(&format!("decode error: {e}")),
        }
        let busy = perf::performance_now() - busy_start;
        perf::record_busy(busy);
        perf::record_apply(cat, busy);
        // After the message has been dispatched, EGFX handlers will have
        // set `dirty_pending` instead of rendering inline. Coalesce: post
        // at most one `setTimeout(0)` flush per wire burst. Synchronous
        // PDUs that arrive while a flush is in flight (gate held true)
        // reuse the already-scheduled flush.
        self.maybe_schedule_flush();
        perf::maybe_flush();
    }

    /// Called by JS after a main-thread `VideoDecoder` produces a decoded
    /// AVC frame: blit the RGBA pixels into the framebuffer image at the
    /// given desktop coordinates and trigger a render. RGBA is provided as
    /// a Uint8Array so the caller doesn't have to copy through wasm-bindgen's
    /// `Vec<u8>` marshal — we copy once into a Rust-owned buffer here.
    #[wasm_bindgen(js_name = blitAvcRgba)]
    pub fn blit_avc_rgba(
        &self,
        desktop_x: u32,
        desktop_y: u32,
        width: u32,
        height: u32,
        rgba: &js_sys::Uint8Array,
    ) {
        let len = rgba.length() as usize;
        let render_result = {
            let mut p = self.inner.borrow_mut();
            // Copy the JS pixels into a reused scratch buffer (no per-frame Vec
            // alloc), then blit from it. `copy_to` requires dst.len() == src.len().
            if p.avc_scratch.len() < len {
                p.avc_scratch.resize(len, 0);
            }
            let DecoderInner {
                fb, avc_scratch, ..
            } = &mut *p;
            rgba.copy_to(&mut avc_scratch[..len]);
            let Some(fb) = fb.as_mut() else {
                return;
            };
            if fb
                .blit_rgba(desktop_x, desktop_y, width, height, &avc_scratch[..len])
                .is_err()
            {
                return;
            }
            fb.render()
        };
        if let Err(e) = render_result {
            self.post_log(&format!("avc blit render error: {e}"));
        }
    }

    /// AVC fast path: upload a decoded `VideoFrame` straight to the GPU texture
    /// (no `copyTo` readback, no CPU copies). Returns `true` if it was drawn;
    /// `false` means the caller should fall back to `blitAvcRgba` (e.g. the
    /// video spans a monitor edge, or there's no framebuffer yet).
    #[wasm_bindgen(js_name = blitAvcFrame)]
    pub fn blit_avc_frame(
        &self,
        desktop_x: u32,
        desktop_y: u32,
        frame: &web_sys::VideoFrame,
    ) -> bool {
        let result = {
            let p = self.inner.borrow();
            let Some(fb) = p.fb.as_ref() else {
                return false;
            };
            fb.blit_video_frame(desktop_x, desktop_y, frame)
        };
        match result {
            Ok(drawn) => drawn,
            Err(e) => {
                self.post_log(&format!("avc frame blit error: {e}"));
                false
            }
        }
    }

    /// Enable/disable offloading RFX-Progressive decode to the worker pool. The
    /// host enables this once the decode workers are up; while disabled, decode
    /// runs inline exactly as before (the fallback / `?offload=0` A-B path).
    ///
    /// On disable we drain+apply any ready ops and drop the rest of the queue:
    /// pending RFX can never complete once we stop offloading, so leaving them
    /// queued would wedge the drain (and the inline path presents synchronously
    /// anyway).
    #[wasm_bindgen(js_name = setEgfxOffload)]
    pub fn set_egfx_offload(&self, enabled: bool) {
        if !enabled {
            self.drain_paint_queue();
            self.paint_queue.borrow_mut().clear();
        }
        self.egfx_offload.set(enabled);
    }

    /// Offload return path: stash one decode worker's returned RFX tile blob for
    /// `seq`. With N position-partitioned workers a `seq` collects N partial
    /// blobs; they're composited together (in wire order vs the inline ops) once
    /// the host marks the `seq` complete via [`Self::rfx_seq_ready`]. Does not
    /// apply or present yet — the wire-order drain owns that.
    #[wasm_bindgen(js_name = stageRfxBlob)]
    pub fn stage_rfx_blob(&self, seq: u32, blob: &[u8]) {
        self.paint_queue.borrow_mut().stage_rfx_blob(seq, blob.to_vec());
    }

    /// Offload return path: the host counted every expected worker reply for
    /// `seq`, so all its tile blobs have arrived. Mark it ready and drain the
    /// queue in `seq` order — this composites the RFX tiles and then any inline
    /// ops / EndFrame present that were waiting behind it.
    #[wasm_bindgen(js_name = rfxSeqReady)]
    pub fn rfx_seq_ready(&self, seq: u32) {
        self.paint_queue.borrow_mut().mark_rfx_ready(seq);
        self.drain_paint_queue();
    }
}

/// Enable/disable the SIMD inverse-DWT at runtime. Default on. Thread-local to
/// this wasm instance (one per decode worker), so the toggle is broadcast to
/// every pool worker via `set-simd-idwt` messages.
#[wasm_bindgen(js_name = setSimdIdwt)]
pub fn set_simd_idwt(on: bool) {
    ironrdp_graphics::progressive::set_simd_idwt(on);
}

impl WorkerDecoder {
    fn post_obj(&self, obj: &js_sys::Object) {
        // Clone the JS callback and DROP the `inner` borrow BEFORE invoking it.
        // The callback runs synchronously and may re-enter wasm: e.g. an
        // `egfxEndFrame` event drives `noteEndFrame -> drainPresents ->
        // presentFrame`, which takes `inner.borrow_mut()`. Holding the borrow
        // across `call1` would panic with "RefCell already borrowed" (the
        // borrow on this line is dropped at the end of THIS statement, before
        // `call1` runs on the next line). `Function::clone` is a cheap JS handle
        // clone.
        let post = self.inner.borrow().post.clone();
        let _ = post.call1(&JsValue::NULL, &JsValue::from(obj));
    }

    fn post_log(&self, text: &str) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"log".into());
        let _ = Reflect::set(&obj, &"text".into(), &text.into());
        self.post_obj(&obj);
    }

    fn post_decoded(&self, text: &str) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"decoded".into());
        let _ = Reflect::set(&obj, &"text".into(), &text.into());
        self.post_obj(&obj);
    }

    fn post_resolution(&self, width: u16, height: u16) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"resolution".into());
        let _ = Reflect::set(&obj, &"width".into(), &JsValue::from(width));
        let _ = Reflect::set(&obj, &"height".into(), &JsValue::from(height));
        self.post_obj(&obj);
    }

    fn post_tdpb_upgrade(&self) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"tdpbUpgrade".into());
        self.post_obj(&obj);
    }

    fn post_response(&self, response: Vec<u8>) {
        let arr = js_sys::Uint8Array::from(response.as_slice());
        let buffer = arr.buffer();
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"rdpResponse".into());
        let _ = Reflect::set(&obj, &"buffer".into(), &buffer);
        self.post_obj(&obj);
    }

    fn post_cursor(&self, event: &CursorEvent) {
        let obj = js_sys::Object::new();
        match event {
            CursorEvent::Bitmap {
                rgba,
                width,
                height,
                hotspot_x,
                hotspot_y,
            } => {
                let _ = Reflect::set(&obj, &"type".into(), &"cursorBitmap".into());
                let _ = Reflect::set(&obj, &"width".into(), &JsValue::from(*width));
                let _ = Reflect::set(&obj, &"height".into(), &JsValue::from(*height));
                let _ = Reflect::set(&obj, &"hotspotX".into(), &JsValue::from(*hotspot_x));
                let _ = Reflect::set(&obj, &"hotspotY".into(), &JsValue::from(*hotspot_y));
                if let Ok(image_data) = web_sys::ImageData::new_with_u8_clamped_array_and_sh(
                    wasm_bindgen::Clamped(rgba),
                    u32::from(*width),
                    u32::from(*height),
                ) {
                    let _ = Reflect::set(&obj, &"imageData".into(), &image_data);
                }
            }
            CursorEvent::Hidden => {
                let _ = Reflect::set(&obj, &"type".into(), &"cursorHidden".into());
            }
            CursorEvent::Default => {
                let _ = Reflect::set(&obj, &"type".into(), &"cursorDefault".into());
            }
        }
        self.post_obj(&obj);
    }

    /// Present the accumulated frame at its EGFX EndFrame boundary. Coalescing
    /// to the frame boundary (rather than per wire-burst) guarantees only
    /// complete, composited frames reach the canvas — no half-applied frames,
    /// no black-rectangle flicker.
    fn present_frame(&self) {
        // DIAGNOSTIC (revert): muted — logged [endframe-rx] proving EgfxEndFrame
        // reached the client; un-comment to re-enable.
        // web_sys::console::warn_1(&"[endframe-rx]".into());
        self.seen_end_frame.set(true);
        self.egfx_present_owed.set(false);
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            if let Err(e) = fb.flush_render() {
                web_sys::console::warn_1(&format!("flush_render error: {e}").into());
            }
        }
    }

    /// Apply the ready prefix of the offload paint queue to the framebuffer in
    /// strict `seq` (wire) order, stopping at the first still-pending RFX op.
    /// This is the wire-order fix: an inline op (SurfaceToSurface/SurfaceToCache
    /// etc.) only runs once every lower-`seq` RFX blit has landed, and a
    /// `Present` marker only flushes once its whole frame has applied.
    ///
    /// Called after every inline-op / `Present` enqueue and on every
    /// `rfxSeqReady`. The `post_log`/`post_presented` events are sent only after
    /// the `inner` borrow is dropped (they re-enter via `post_obj`).
    fn drain_paint_queue(&self) {
        let ops = self.paint_queue.borrow_mut().drain_ready();
        if ops.is_empty() {
            return;
        }
        let mut errors: Vec<String> = Vec::new();
        // Count Present markers applied — a single drain can flush several frames
        // when pipelined RFX seqs complete together, and the `[pacing]` present
        // histogram should see each one.
        let mut presents = 0usize;
        {
            let mut p = self.inner.borrow_mut();
            let Some(fb) = p.fb.as_mut() else {
                // No framebuffer yet (pre-activation) — drop the drained ops,
                // matching the inline path which drops pre-init PDUs.
                return;
            };
            for op in ops {
                apply_pending(fb, op, &mut errors, &mut presents);
            }
        }
        for e in errors {
            self.post_log(&e);
        }
        for _ in 0..presents {
            self.post_presented();
        }
    }

    /// Offload: tell the host a frame was just presented (flushed to GL), so its
    /// `[pacing]` diagnostic can track present cadence now that present is
    /// driven by the wasm wire-order drain rather than the JS barrier.
    fn post_presented(&self) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"egfxPresented".into());
        self.post_obj(&obj);
    }

    /// Offload: emit an `rfxChunk` event so the host broadcasts the PDU to the
    /// decode pool. Carries the surface dims (so workers size the tile grid /
    /// reset on resize) and the wire-order `seq` assigned by the paint queue —
    /// the host echoes `seq` on each returned blob (`stageRfxBlob`) and signals
    /// `rfxSeqReady(seq)` once all partition replies are in. Modeled on
    /// `apply_egfx_avc_frame`'s `avcChunk`.
    fn emit_rfx_chunk(
        &self,
        w: &codec::messages::EgfxWireToSurface2,
        surface_width: u16,
        surface_height: u16,
        seq: u32,
    ) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"rfxChunk".into());
        let _ = Reflect::set(&obj, &"seq".into(), &JsValue::from(seq));
        let _ = Reflect::set(&obj, &"surfaceId".into(), &JsValue::from(w.surface_id));
        let _ = Reflect::set(&obj, &"ctxId".into(), &JsValue::from(w.codec_context_id));
        let _ = Reflect::set(&obj, &"originX".into(), &JsValue::from(w.surface_origin_x));
        let _ = Reflect::set(&obj, &"originY".into(), &JsValue::from(w.surface_origin_y));
        let _ = Reflect::set(&obj, &"surfaceWidth".into(), &JsValue::from(surface_width));
        let _ = Reflect::set(&obj, &"surfaceHeight".into(), &JsValue::from(surface_height));
        let payload = js_sys::Uint8Array::new_with_length(w.bitmap_data.len() as u32);
        payload.copy_from(&w.bitmap_data);
        let _ = Reflect::set(&obj, &"payload".into(), &payload);
        self.post_obj(&obj);
    }

    /// Step 2 (offload): emit an `egfxDeleteContext` event so the decode worker
    /// evicts the per-context progressive state (ordered with the surface's PDUs).
    fn emit_delete_context(&self, surface_id: u32, codec_context_id: u32) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"egfxDeleteContext".into());
        let _ = Reflect::set(&obj, &"surfaceId".into(), &JsValue::from(surface_id));
        let _ = Reflect::set(&obj, &"ctxId".into(), &JsValue::from(codec_context_id));
        self.post_obj(&obj);
    }

    /// Offload: emit an `egfxEndFrame` event marking the frame boundary. Present
    /// itself is now driven by the wasm wire-order drain (a `Present` marker is
    /// enqueued at this boundary and flushes once the whole frame has applied);
    /// this event only feeds the host's `[pacing]` arrival-cadence diagnostic.
    fn emit_end_frame(&self) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"egfxEndFrame".into());
        self.post_obj(&obj);
    }

    /// If the framebuffer has pending dirty marks and no flush is already
    /// scheduled, post a `setTimeout(0)` that will run after the current
    /// JS task drains and call `Framebuffer::flush_render`. The cell-guard
    /// makes this a no-op if a previous PDU already scheduled the flush —
    /// many EGFX PDUs in one wire burst coalesce to a single GL upload.
    fn maybe_schedule_flush(&self) {
        // In EGFX-offload mode the present is barrier-gated by the host
        // (`presentFrame`), so the inline per-burst coalescing must not fire — it
        // would flush a frame before the decode worker's tiles land. FastPath
        // still renders inline (it calls `render` directly, not this path).
        if self.egfx_offload.get() {
            return;
        }
        let dirty = {
            let p = self.inner.borrow();
            p.fb.as_ref().map_or(false, |fb| fb.dirty_pending())
        };
        if !dirty || self.flush_scheduled.get() {
            return;
        }
        // EGFX presents at its EndFrame boundary (present_frame), not per wire-
        // burst — otherwise half-applied frames (bg fill before content) reach
        // the canvas and the screen flickers. Defer while a frame's content is
        // owed a present. Gated on `seen_end_frame` so a server that never sends
        // EndFrame still updates via the legacy per-burst path.
        if self.seen_end_frame.get() && self.egfx_present_owed.get() {
            return;
        }
        self.flush_scheduled.set(true);

        // Reuse the persistent flush closure built in `new` — no per-flush
        // closure allocation. The gate is cleared at the start of the callback.
        let global = js_sys::global().unchecked_into::<web_sys::DedicatedWorkerGlobalScope>();
        if let Err(e) = global.set_timeout_with_callback_and_timeout_and_arguments_0(
            self.flush_cb.as_ref().unchecked_ref(),
            0,
        ) {
            // Scheduling failed — clear the gate so the next PDU retries.
            self.flush_scheduled.set(false);
            self.post_log(&format!("setTimeout schedule failed: {e:?}"));
        }
    }

    /// DIAGNOSTIC (revert): tally inbound graphics messages and emit a
    /// `post_log` summary on the first message, the first EGFX message, and
    /// every 120 graphics messages thereafter.
    fn bump_traffic(&self, msg: &InboundMessage) {
        use InboundMessage::*;
        let summary = {
            let mut t = self.traffic.borrow_mut();
            match msg {
                FastPathPdu(_) => t.fastpath += 1,
                EgfxBitmap(_) => t.egfx_bitmap += 1,
                EgfxClearCodec(_) => t.egfx_clearcodec += 1,
                EgfxUncompressed(_) => t.egfx_uncompressed += 1,
                EgfxPlanar(_) => t.egfx_planar += 1,
                EgfxAvcFrame(_) | EgfxAvc420(_) => t.egfx_avc += 1,
                EgfxSolidFill(_) => t.egfx_solidfill += 1,
                EgfxSurfaceToCache(_) => t.egfx_s2c += 1,
                EgfxCacheToSurface(_) => t.egfx_c2s += 1,
                EgfxEvictCacheEntry(_) => t.egfx_evict += 1,
                EgfxSurfaceToSurface(_) => t.egfx_s2s += 1,
                EgfxWireToSurface2(_) => t.egfx_rfx += 1,
                EgfxDeleteEncodingContext(_) => t.egfx_other += 1,
                PngFrame(_) | Png2Frame(_) => t.png += 1,
                _ => return, // non-graphics (latency/ping/cursor/etc.): ignore
            }
            t.gfx_total += 1;
            let is_egfx = matches!(
                msg,
                EgfxBitmap(_)
                    | EgfxClearCodec(_)
                    | EgfxUncompressed(_)
                    | EgfxPlanar(_)
                    | EgfxAvcFrame(_)
                    | EgfxAvc420(_)
                    | EgfxSolidFill(_)
                    | EgfxSurfaceToCache(_)
                    | EgfxCacheToSurface(_)
                    | EgfxEvictCacheEntry(_)
                    | EgfxSurfaceToSurface(_)
                    | EgfxWireToSurface2(_)
                    | EgfxDeleteEncodingContext(_)
            );
            // Mark this frame's content as awaiting an EndFrame present, so the
            // per-burst flush defers (see maybe_schedule_flush / present_frame).
            if is_egfx {
                self.egfx_present_owed.set(true);
            }
            let first_egfx = is_egfx && !t.logged_first_egfx;
            if first_egfx {
                t.logged_first_egfx = true;
            }
            // DIAGNOSTIC (revert): muted — codec-mix [traffic] summary (counts
            // fastpath vs egfx{clear/avc/rfx/...}). Un-comment to re-enable:
            //   if first_egfx || t.gfx_total == 1 || t.gfx_total % 120 == 0 {
            //       Some(t.summary())
            //   } else { None }
            let _ = first_egfx;
            None::<String>
        };
        // DIAGNOSTIC (revert): muted — emitted the [traffic] summary via post_log.
        // if let Some(s) = summary { self.post_log(&s); }
        let _ = summary;
    }

    fn handle_inbound(&self, msg: InboundMessage) {
        self.bump_traffic(&msg);
        match msg {
            InboundMessage::TdpbUpgrade => {
                self.inner.borrow_mut().codec.upgrade_to_tdpb();
                self.post_tdpb_upgrade();
            }
            InboundMessage::ServerHello(h) => {
                self.post_decoded(&format!(
                    "[multimon-marker] ServerHello(clipboard={}, dir_remove={}, hidpi={}, multi_monitor={}, activation={}x{})",
                    h.clipboard_enabled,
                    h.directory_remove_supported,
                    h.hidpi_supported,
                    h.multi_monitor_supported,
                    h.activation.screen_width,
                    h.activation.screen_height,
                ));
                self.init_framebuffer(
                    h.activation.io_channel_id,
                    h.activation.user_channel_id,
                    h.activation.screen_width,
                    h.activation.screen_height,
                );
            }
            InboundMessage::ConnectionActivated(c) => {
                self.post_decoded(&format!(
                    "ConnectionActivated(io={}, user={}, {}x{})",
                    c.io_channel_id, c.user_channel_id, c.screen_width, c.screen_height,
                ));
                self.init_framebuffer(
                    c.io_channel_id,
                    c.user_channel_id,
                    c.screen_width,
                    c.screen_height,
                );
            }
            InboundMessage::FastPathPdu(pdu) => {
                self.post_log(&format!(
                    "[multimon-marker][legacy-fastpath] inbound FastPathPdu: {} bytes",
                    pdu.pdu.len()
                ));
                self.apply_fast_path(&pdu.pdu);
            }
            InboundMessage::EgfxBitmap(b) => {
                self.apply_egfx_bitmap(&b);
            }
            InboundMessage::EgfxAvcFrame(f) => {
                self.apply_egfx_avc_frame(f);
            }
            InboundMessage::EgfxClearCodec(c) => {
                self.apply_egfx_clearcodec(c);
            }
            InboundMessage::EgfxUncompressed(u) => {
                self.apply_egfx_uncompressed(u);
            }
            InboundMessage::EgfxPlanar(p) => {
                self.apply_egfx_planar(p);
            }
            InboundMessage::EgfxAvc420(a) => {
                self.apply_egfx_avc420(a);
            }
            InboundMessage::EgfxSolidFill(s) => {
                self.apply_egfx_solid_fill(s);
            }
            InboundMessage::EgfxSurfaceToCache(c) => {
                self.apply_egfx_surface_to_cache(c);
            }
            InboundMessage::EgfxCacheToSurface(c) => {
                self.apply_egfx_cache_to_surface(c);
            }
            InboundMessage::EgfxEvictCacheEntry(e) => {
                self.apply_egfx_evict_cache_entry(e);
            }
            InboundMessage::EgfxSurfaceToSurface(s) => {
                self.apply_egfx_surface_to_surface(s);
            }
            InboundMessage::EgfxWireToSurface2(w) => {
                self.apply_egfx_wire_to_surface2(w);
            }
            InboundMessage::EgfxDeleteEncodingContext(d) => {
                self.apply_egfx_delete_encoding_context(d);
            }
            InboundMessage::EgfxEndFrame(_) => {
                // Offload: the frame's tiles may still be decoding in the pool, so
                // enqueue a Present marker in wire order. The drain flushes it once
                // every paint op before this boundary has applied (so only complete
                // frames reach the canvas — no tearing). `emit_end_frame` is just
                // the pacing-arrival marker now. Inline mode presents immediately
                // (decode already happened synchronously).
                if self.egfx_offload.get() {
                    self.seen_end_frame.set(true);
                    self.paint_queue.borrow_mut().enqueue_ready(PendingPaint::Present);
                    self.emit_end_frame();
                    self.drain_paint_queue();
                } else {
                    self.present_frame();
                }
            }
            other => self.post_decoded(&summarize(&other)),
        }
    }

    fn init_framebuffer(&self, io_channel_id: u16, user_channel_id: u16, width: u16, height: u16) {
        if let Err(e) = framebuffer::require_nonzero_dims(width, height) {
            self.post_log(&format!("framebuffer init skipped: {e}"));
            return;
        }

        // Borrow once to inspect existing dims, drop, then re-borrow mut.
        let existing = {
            let p = self.inner.borrow();
            p.fb.as_ref().map(|fb| (fb.width(), fb.height()))
        };

        match existing {
            // Same dims — nothing to do.
            Some((w, h)) if w == width && h == height => {}
            // Mid-session resize/reactivation (e.g. a popup monitor closed and
            // main sent a smaller ScreenSpec). Resize IN PLACE so the registered
            // canvases (main + surviving popups) survive — recreating the
            // framebuffer would drop every canvas and blank the screen.
            Some(_) => {
                // Drop the offload wire-order queue across a resize: captured
                // inline ops reference the old surface dims (their replies for
                // old-dim seqs become no-ops). Per-surface RFX decode state is
                // NOT cleared here — Windows keeps surfaces and their streams
                // across a ResetGraphics-driven resize; surfaces that really
                // die arrive as whole-surface DeleteEncodingContext sentinels
                // (ctx = u32::MAX) and are dropped individually. Apply whatever
                // is already ready against the current framebuffer first, then
                // clear.
                self.drain_paint_queue();
                self.paint_queue.borrow_mut().clear();
                let mut p = self.inner.borrow_mut();
                let res = p
                    .fb
                    .as_mut()
                    .map(|fb| fb.resize_preserving_canvases(width, height));
                drop(p);
                match res {
                    Some(Ok(())) => {
                        self.post_log(&format!(
                            "framebuffer resized: {width}x{height}, io={io_channel_id}, user={user_channel_id} (canvases preserved)"
                        ));
                        self.post_resolution(width, height);
                    }
                    Some(Err(e)) => {
                        self.post_log(&format!("framebuffer resize error: {e:#}"));
                    }
                    None => {}
                }
            }
            // First activation: build fresh and drain pending registrations.
            None => match Framebuffer::new(io_channel_id, user_channel_id, width, height) {
                Ok(fb) => {
                    let mut p = self.inner.borrow_mut();
                    p.fb = Some(fb);
                    // Drain pending canvas registrations.
                    let pending = std::mem::take(&mut p.pending);
                    let fb = p.fb.as_mut().unwrap();
                    for pc in pending {
                        if let Err(e) = fb.add_canvas(pc.id, &pc.canvas, pc.viewport) {
                            // Can't call self.post_log here without dropping the
                            // borrow first; just drop the error for now.
                            let _ = e;
                        }
                    }
                    drop(p);
                    self.post_log(&format!(
                        "framebuffer initialized: {width}x{height}, io={io_channel_id}, user={user_channel_id}"
                    ));
                    self.post_resolution(width, height);
                }
                Err(e) => self.post_log(&format!("framebuffer init error: {e:#}")),
            },
        }
    }

    fn apply_fast_path(&self, pdu: &[u8]) {
        // In EGFX-offload mode FastPath must NOT present inline. `fb.render()`
        // does `mem::take(self.dirty)`, so a FastPath PDU arriving mid-drag would
        // flush a PARTIAL EGFX frame — e.g. a SurfaceToSurface scroll already
        // applied while this frame's RFX repaint is still pending in the paint
        // queue — tearing thin stale seams along the drag path. Instead we apply
        // the FastPath pixels now (mutating the image) but DEFER the present to a
        // queued `Present` marker, drained in wire order behind any pending EGFX
        // ops (and immediately if none are pending). See the EgfxEndFrame handler
        // and RFX_POOL_CORRUPTION_HANDOFF.md.
        //
        // (FastPath pixel mutation still happens synchronously here; FastPath and
        // EGFX target disjoint regions in practice, so the rare write-order
        // inversion vs a pending RFX is acceptable — the fix targets the visible
        // partial-frame PRESENT.)
        let offload = self.egfx_offload.get();
        // Process the PDU first (mutable borrow scoped tight so post_* calls
        // afterwards can also borrow).
        let (response, cursors, render_result) = {
            let mut p = self.inner.borrow_mut();
            let Some(fb) = p.fb.as_mut() else {
                drop(p);
                self.post_log("← FastPathPdu before framebuffer initialized; dropping");
                return;
            };
            match fb.apply_fast_path(pdu) {
                Ok(out) => {
                    // Inline mode renders now; offload defers (see above).
                    let render_result = if offload { Ok(()) } else { fb.render() };
                    (out.response, out.cursors, render_result)
                }
                Err(e) => {
                    drop(p);
                    self.post_log(&format!("fastpath error: {e}"));
                    return;
                }
            }
        };

        if offload {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::Present);
            self.drain_paint_queue();
        }

        if !response.is_empty() {
            let len = response.len();
            let send_start = perf::performance_now();
            self.post_response(response);
            perf::record_response_send(perf::performance_now() - send_start, len);
        }
        for cursor in &cursors {
            let _t = perf::Timer::new(perf::record_cursor_post);
            self.post_cursor(cursor);
        }
        if let Err(e) = render_result {
            self.post_log(&format!("render error: {e}"));
        }
    }

    /// Forward an inbound EGFX AVC444/v2 frame to the main thread for
    /// browser-side H.264 decode. WebCodecs `VideoDecoder` is not exposed
    /// in the SharedWorker scope in current Chrome (only DedicatedWorker /
    /// Window), so we marshal the H.264 NAL units across the port boundary,
    /// let each main thread decode with its own `VideoDecoder`, and accept
    /// the resulting RGBA back via `blit_avc_rgba`.
    // WIRE-ORDER NOTE: AVC frames are intentionally NOT routed through the
    // `paint_queue`. The H.264 stream is decoded asynchronously by a per-surface
    // browser `VideoDecoder` with its own frame cadence (independent of the EGFX
    // EndFrame boundary), and its result is a GPU `VideoFrame` (uploaded straight
    // to the texture) rather than stageable CPU pixels — so it can't slot into
    // the seq-ordered drain the way RFX/inline ops do. This is safe because AVC
    // and RFX/inline target DISJOINT regions in practice (AVC = the full-motion
    // video tile; RFX/ClearCodec/SurfaceToSurface = the surrounding desktop
    // chrome). If a server is ever observed overlapping an AVC region with
    // same-frame EGFX paints, the AVC blit would need to flow through the queue's
    // Present path too. (AVC was async before the wire-order fix, so this is not
    // a regression — see RFX_POOL_CORRUPTION_HANDOFF.md.)
    fn apply_egfx_avc_frame(&self, f: codec::messages::EgfxAvcFrame) {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"avcChunk".into());
        let _ = Reflect::set(&obj, &"surface".into(), &JsValue::from(f.surface_id));
        let _ = Reflect::set(&obj, &"desktopX".into(), &JsValue::from(f.desktop_x));
        let _ = Reflect::set(&obj, &"desktopY".into(), &JsValue::from(f.desktop_y));
        let _ = Reflect::set(&obj, &"destWidth".into(), &JsValue::from(f.dest_width));
        let _ = Reflect::set(&obj, &"destHeight".into(), &JsValue::from(f.dest_height));
        let _ = Reflect::set(&obj, &"codecId".into(), &JsValue::from(f.codec_id));
        let _ = Reflect::set(&obj, &"encoding".into(), &JsValue::from(f.encoding));
        // Pass the H.264 bytes as a Uint8Array (zero-copy into JS heap).
        let luma_arr = js_sys::Uint8Array::new_with_length(f.luma_h264.len() as u32);
        luma_arr.copy_from(&f.luma_h264);
        let _ = Reflect::set(&obj, &"luma".into(), &luma_arr);
        // chroma_h264 is empty in Phase 1 (LUMA encoding) but include for
        // forward-compat with LUMA_AND_CHROMA frames once Phase 2 lands.
        if !f.chroma_h264.is_empty() {
            let chroma_arr = js_sys::Uint8Array::new_with_length(f.chroma_h264.len() as u32);
            chroma_arr.copy_from(&f.chroma_h264);
            let _ = Reflect::set(&obj, &"chroma".into(), &chroma_arr);
        }
        self.post_obj(&obj);
    }

    fn apply_egfx_solid_fill(&self, s: codec::messages::EgfxSolidFill) {
        // Offload: capture + enqueue in wire order instead of painting now, so
        // this fill applies after any prior in-flight RFX (a SolidFill over a
        // just-RFX'd region must not jump ahead of that RFX blit).
        if self.egfx_offload.get() {
            let rects = s
                .rects
                .iter()
                .map(|r| (r.left, r.top, r.right, r.bottom))
                .collect();
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::SolidFill {
                    surface_id: s.surface_id,
                    r: s.color_r,
                    g: s.color_g,
                    b: s.color_b,
                    rects,
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxSolidFill before framebuffer initialized; dropping");
            return;
        };
        let rects: Vec<(u32, u32, u32, u32)> = s
            .rects
            .iter()
            .map(|r| (r.left, r.top, r.right, r.bottom))
            .collect();
        if let Err(e) = fb.apply_solid_fill(s.surface_id, s.color_r, s.color_g, s.color_b, &rects) {
            drop(p);
            self.post_log(&format!("solid_fill error: {e}"));
            return;
        }
        fb.mark_dirty_pending();
    }

    fn apply_egfx_surface_to_cache(&self, c: codec::messages::EgfxSurfaceToCache) {
        // Offload: enqueue in wire order. This op READS the framebuffer into the
        // bitmap cache — if it ran ahead of a prior RFX blit it would latch
        // stale pixels that a later CacheToSurface replays (the accumulating
        // corruption), so it must wait behind in-flight RFX.
        if self.egfx_offload.get() {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::SurfaceToCache {
                    surface_id: c.surface_id,
                    cache_slot: c.cache_slot,
                    src: (
                        c.source_rect.left,
                        c.source_rect.top,
                        c.source_rect.right,
                        c.source_rect.bottom,
                    ),
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxSurfaceToCache before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.apply_surface_to_cache(
            c.surface_id,
            c.cache_slot,
            c.source_rect.left,
            c.source_rect.top,
            c.source_rect.right,
            c.source_rect.bottom,
        ) {
            drop(p);
            self.post_log(&format!("surface_to_cache error: {e}"));
        }
    }

    fn apply_egfx_cache_to_surface(&self, c: codec::messages::EgfxCacheToSurface) {
        // Offload: enqueue in wire order (a cached blit must land in the right
        // slot of the op stream relative to RFX / surface-copies).
        if self.egfx_offload.get() {
            let points = c.dest_points.iter().map(|p| (p.x, p.y)).collect();
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::CacheToSurface {
                    surface_id: c.surface_id,
                    cache_slot: c.cache_slot,
                    points,
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxCacheToSurface before framebuffer initialized; dropping");
            return;
        };
        let points: Vec<(u32, u32)> = c.dest_points.iter().map(|p| (p.x, p.y)).collect();
        if let Err(e) = fb.apply_cache_to_surface(c.surface_id, c.cache_slot, &points) {
            drop(p);
            self.post_log(&format!("cache_to_surface error: {e}"));
            return;
        }
        fb.mark_dirty_pending();
    }

    fn apply_egfx_evict_cache_entry(&self, e: codec::messages::EgfxEvictCacheEntry) {
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.apply_evict_cache_entry(e.cache_slot);
        }
    }

    fn apply_egfx_surface_to_surface(&self, s: codec::messages::EgfxSurfaceToSurface) {
        // Offload: enqueue in wire order. This is THE corruption op — a window
        // drag/scroll copies a framebuffer region to a new position by READING
        // self.image. Running it ahead of a prior RFX blit copies stale pixels
        // and re-propagates them each frame. Deferring it behind in-flight RFX
        // is the core of the fix.
        if self.egfx_offload.get() {
            let points = s.dest_points.iter().map(|p| (p.x, p.y)).collect();
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::SurfaceToSurface {
                    src_surface_id: s.source_surface_id,
                    dst_surface_id: s.destination_surface_id,
                    src: (
                        s.source_rect.left,
                        s.source_rect.top,
                        s.source_rect.right,
                        s.source_rect.bottom,
                    ),
                    points,
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxSurfaceToSurface before framebuffer initialized; dropping");
            return;
        };
        let points: Vec<(u32, u32)> = s.dest_points.iter().map(|p| (p.x, p.y)).collect();
        if let Err(e) = fb.apply_surface_to_surface(
            s.source_surface_id,
            s.destination_surface_id,
            s.source_rect.left,
            s.source_rect.top,
            s.source_rect.right,
            s.source_rect.bottom,
            &points,
        ) {
            drop(p);
            self.post_log(&format!("surface_to_surface error: {e}"));
            return;
        }
        fb.mark_dirty_pending();
    }

    /// Apply an inbound EGFX WireToSurface2 PDU (RFX Progressive). Decodes
    /// the payload into one or more 64×64 RGBA tiles and blits each into
    /// the framebuffer at `(surface_origin + tile.dest)` in desktop coords,
    /// then re-renders.
    fn apply_egfx_wire_to_surface2(&self, w: codec::messages::EgfxWireToSurface2) {
        // Offload: ship the PDU to the decode pool instead of decoding inline,
        // and enqueue a PENDING RFX entry in wire order. The decoded tile blobs
        // return via `stageRfxBlob(seq)` and apply (in order, vs the inline ops)
        // once the host calls `rfxSeqReady(seq)`. Drops pre-framebuffer PDUs
        // exactly like the inline path below.
        if self.egfx_offload.get() {
            let dims = self.inner.borrow().fb.as_ref().map(Framebuffer::dims);
            let Some((sw, sh)) = dims else {
                self.post_log("← EgfxWireToSurface2 before framebuffer initialized; dropping");
                return;
            };
            let seq = self
                .paint_queue
                .borrow_mut()
                .enqueue_rfx(w.surface_origin_x, w.surface_origin_y);
            self.emit_rfx_chunk(&w, sw, sh, seq);
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxWireToSurface2 before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.apply_rfx_progressive(
            w.surface_id,
            w.codec_context_id,
            w.surface_origin_x,
            w.surface_origin_y,
            &w.bitmap_data,
        ) {
            drop(p);
            self.post_log(&format!(
                "[blackrect][rfx-progressive] decode error surface={} ctx={} origin=({},{}) bytes={}: {e}",
                w.surface_id, w.codec_context_id, w.surface_origin_x, w.surface_origin_y, w.bitmap_data.len()
            ));
            return;
        }
        fb.mark_dirty_pending();
    }

    /// Apply an inbound EGFX DeleteEncodingContext (RFX Progressive teardown).
    /// Evicts the per-context tile state inside the surface's progressive
    /// decoder; the decoder itself stays (the surface may immediately open
    /// a new codec_context_id).
    fn apply_egfx_delete_encoding_context(&self, d: codec::messages::EgfxDeleteEncodingContext) {
        // Step 2 (offload): the progressive decoders live in the decode worker, so
        // route the eviction there (ordered with this surface's PDUs).
        if self.egfx_offload.get() {
            self.emit_delete_context(d.surface_id, d.codec_context_id);
            return;
        }
        let mut p = self.inner.borrow_mut();
        if let Some(fb) = p.fb.as_mut() {
            fb.delete_progressive_context(d.surface_id, d.codec_context_id);
        }
    }

    /// Apply an EGFX Planar PDU. Server forwards the raw RDP 6.0 bitmap
    /// stream bytes; the framebuffer decodes via
    /// `ironrdp_graphics::rdp6::bitmap_stream` and blits as opaque RGB.
    fn apply_egfx_planar(&self, p: codec::messages::EgfxPlanar) {
        if self.egfx_offload.get() {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::Planar {
                    surface_id: p.surface_id,
                    dest_x: p.dest_x,
                    dest_y: p.dest_y,
                    width: p.width,
                    height: p.height,
                    pdu: p.pdu_data.to_vec(),
                });
            self.drain_paint_queue();
            return;
        }
        let mut g = self.inner.borrow_mut();
        let Some(fb) = g.fb.as_mut() else {
            drop(g);
            self.post_log("← EgfxPlanar before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.apply_planar(p.surface_id, p.dest_x, p.dest_y, p.width, p.height, &p.pdu_data) {
            drop(g);
            self.post_log(&format!(
                "[blackrect][planar] decode error surface={} dest=({},{}) {}x{} bytes={}: {e}",
                p.surface_id, p.dest_x, p.dest_y, p.width, p.height, p.pdu_data.len()
            ));
            return;
        }
        fb.mark_dirty_pending();
    }

    /// Apply an EGFX Avc420 PDU. Strips the `RFX_AVC420_BITMAP_STREAM` region
    /// header and feeds the H.264 NAL units to the existing browser
    /// `VideoDecoder` pipeline (same one used for AVC444's luma stream).
    fn apply_egfx_avc420(&self, a: codec::messages::EgfxAvc420) {
        // Strip the RFX_AVC420_BITMAP_STREAM region header
        // ([MS-RDPEGFX] 2.2.4.4.1) to reach the H.264 payload. See
        // `avc420_h264_offset` for the exact layout.
        let header_size = match avc420_h264_offset(&a.pdu_data) {
            Ok(n) => n,
            Err(e) => {
                self.post_log(&e);
                return;
            }
        };
        let h264 = &a.pdu_data[header_size..];

        // Surface the H.264 frame through the same JS bridge AVC444's luma
        // stream uses. We synthesize an EgfxAvcFrame with empty chroma —
        // Avc420 is single-stream, the VideoDecoder treats this as a normal
        // H.264 frame.
        let frame = codec::messages::EgfxAvcFrame {
            surface_id: a.surface_id,
            desktop_x: a.dest_x.max(0) as u32,
            desktop_y: a.dest_y.max(0) as u32,
            dest_width: a.width,
            dest_height: a.height,
            codec_id: 0x0b, // CODEC_AVC420 for downstream JS routing
            encoding: 1,    // LUMA — single-stream
            luma_h264: bytes::Bytes::copy_from_slice(h264),
            chroma_h264: bytes::Bytes::new(),
        };
        self.apply_egfx_avc_frame(frame);
    }

    /// Apply an inbound EGFX Uncompressed PDU. Server forwards raw bitmap
    /// bytes verbatim with the source `pixel_format` byte; the framebuffer
    /// handles channel reorder and source-over alpha composite.
    fn apply_egfx_uncompressed(&self, u: codec::messages::EgfxUncompressed) {
        if self.egfx_offload.get() {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::Uncompressed {
                    surface_id: u.surface_id,
                    dest_x: u.dest_x,
                    dest_y: u.dest_y,
                    width: u.width,
                    height: u.height,
                    pixel_format: u.pixel_format,
                    bitmap: u.bitmap_data.to_vec(),
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxUncompressed before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.apply_uncompressed(
            u.surface_id, u.dest_x, u.dest_y, u.width, u.height, u.pixel_format, &u.bitmap_data,
        ) {
            drop(p);
            self.post_log(&format!(
                "[blackrect][uncompressed] error surface={} dest=({},{}) {}x{} pixfmt={} bytes={}: {e}",
                u.surface_id, u.dest_x, u.dest_y, u.width, u.height, u.pixel_format, u.bitmap_data.len()
            ));
            return;
        }
        fb.mark_dirty_pending();
    }

    /// Apply an inbound EGFX ClearCodec PDU. The server forwards the raw
    /// `RFX_CLEAR_BITMAP_STREAM` bytes and the destination rectangle from
    /// the surrounding `WireToSurface1Pdu`; the wasm-side decoder writes
    /// directly into the framebuffer image, preserving existing pixels for
    /// sub-regions the wire format doesn't paint (see `Framebuffer::
    /// apply_clearcodec` for the rationale).
    fn apply_egfx_clearcodec(&self, c: codec::messages::EgfxClearCodec) {
        // Offload: ClearCodec is still DECODED on the main thread (low-volume
        // crisp-UI path; connection-scoped glyph/V-bar caches aren't tile-
        // partitionable), but it must APPLY through the same wire-order queue as
        // RFX/SurfaceToSurface. A window drag is a surface-copy + ClearCodec
        // repaint; applying it synchronously while RFX is in flight is exactly
        // the intra-frame inversion this fix removes. We capture the raw PDU and
        // decode+blit it at drain time (still on this thread, in wire order).
        if self.egfx_offload.get() {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::ClearCodec {
                    surface_id: c.surface_id,
                    dest_x: c.dest_x,
                    dest_y: c.dest_y,
                    width: c.width,
                    height: c.height,
                    pdu: c.pdu_data.to_vec(),
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxClearCodec before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.apply_clearcodec(c.surface_id, c.dest_x, c.dest_y, c.width, c.height, &c.pdu_data) {
            drop(p);
            self.post_log(&format!(
                "[blackrect][clearcodec] decode error surface={} dest=({},{}) {}x{} bytes={}: {e}",
                c.surface_id, c.dest_x, c.dest_y, c.width, c.height, c.pdu_data.len()
            ));
            return;
        }
        fb.mark_dirty_pending();
    }

    /// Apply a pre-decoded EGFX bitmap update from the server-side IronRDP
    /// (Microsoft::Windows::RDS::Graphics DVC). The RGBA bytes are already
    /// in desktop coordinates so we blit them into the framebuffer image
    /// and mark that region dirty for the next render.
    fn apply_egfx_bitmap(&self, b: &codec::messages::EgfxBitmap) {
        if self.egfx_offload.get() {
            self.paint_queue
                .borrow_mut()
                .enqueue_ready(PendingPaint::Bitmap {
                    desktop_x: b.desktop_x,
                    desktop_y: b.desktop_y,
                    width: b.width,
                    height: b.height,
                    rgba: b.rgba.to_vec(),
                });
            self.drain_paint_queue();
            return;
        }
        let mut p = self.inner.borrow_mut();
        let Some(fb) = p.fb.as_mut() else {
            drop(p);
            self.post_log("← EgfxBitmap before framebuffer initialized; dropping");
            return;
        };
        if let Err(e) = fb.blit_rgba(b.desktop_x, b.desktop_y, b.width, b.height, &b.rgba) {
            drop(p);
            self.post_log(&format!("egfx blit error: {e}"));
            return;
        }
        fb.mark_dirty_pending();
    }
}

/// Apply one drained [`PendingPaint`] to the framebuffer. This mirrors the
/// inline `apply_egfx_*` handlers exactly (same `apply_*` calls, same
/// `mark_dirty_pending` policy, same error log tags) — it's the deferred replay
/// of an op that was captured at PDU-parse time and is now running in wire
/// order. Errors are collected (the caller logs them after dropping the `inner`
/// borrow) and never abort the drain. `presented` is set when a `Present`
/// marker flushed, so the caller can emit the pacing event.
fn apply_pending(
    fb: &mut Framebuffer,
    op: PendingPaint,
    errors: &mut Vec<String>,
    presents: &mut usize,
) {
    match op {
        PendingPaint::Rfx {
            origin_x,
            origin_y,
            blobs,
        } => {
            // Each partition's blob composites independently (disjoint tiles);
            // mark dirty once after the whole set.
            for blob in &blobs {
                fb.blit_rfx_blob(origin_x, origin_y, blob);
            }
            fb.mark_dirty_pending();
        }
        PendingPaint::SolidFill {
            surface_id,
            r,
            g,
            b,
            rects,
        } => {
            if let Err(e) = fb.apply_solid_fill(surface_id, r, g, b, &rects) {
                errors.push(format!("solid_fill error: {e}"));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::SurfaceToCache {
            surface_id,
            cache_slot,
            src,
        } => {
            // Snapshot into the bitmap cache only — no dirty mark (mirrors the
            // inline `apply_egfx_surface_to_cache`, which paints nothing).
            if let Err(e) =
                fb.apply_surface_to_cache(surface_id, cache_slot, src.0, src.1, src.2, src.3)
            {
                errors.push(format!("surface_to_cache error: {e}"));
            }
        }
        PendingPaint::CacheToSurface {
            surface_id,
            cache_slot,
            points,
        } => {
            if let Err(e) = fb.apply_cache_to_surface(surface_id, cache_slot, &points) {
                errors.push(format!("cache_to_surface error: {e}"));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::SurfaceToSurface {
            src_surface_id,
            dst_surface_id,
            src,
            points,
        } => {
            if let Err(e) = fb.apply_surface_to_surface(
                src_surface_id,
                dst_surface_id,
                src.0,
                src.1,
                src.2,
                src.3,
                &points,
            ) {
                errors.push(format!("surface_to_surface error: {e}"));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::ClearCodec {
            surface_id,
            dest_x,
            dest_y,
            width,
            height,
            pdu,
        } => {
            if let Err(e) = fb.apply_clearcodec(surface_id, dest_x, dest_y, width, height, &pdu) {
                errors.push(format!(
                    "[blackrect][clearcodec] decode error surface={surface_id} dest=({dest_x},{dest_y}) {width}x{height} bytes={}: {e}",
                    pdu.len()
                ));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::Uncompressed {
            surface_id,
            dest_x,
            dest_y,
            width,
            height,
            pixel_format,
            bitmap,
        } => {
            if let Err(e) =
                fb.apply_uncompressed(surface_id, dest_x, dest_y, width, height, pixel_format, &bitmap)
            {
                errors.push(format!(
                    "[blackrect][uncompressed] error surface={surface_id} dest=({dest_x},{dest_y}) {width}x{height} pixfmt={pixel_format} bytes={}: {e}",
                    bitmap.len()
                ));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::Planar {
            surface_id,
            dest_x,
            dest_y,
            width,
            height,
            pdu,
        } => {
            if let Err(e) = fb.apply_planar(surface_id, dest_x, dest_y, width, height, &pdu) {
                errors.push(format!(
                    "[blackrect][planar] decode error surface={surface_id} dest=({dest_x},{dest_y}) {width}x{height} bytes={}: {e}",
                    pdu.len()
                ));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::Bitmap {
            desktop_x,
            desktop_y,
            width,
            height,
            rgba,
        } => {
            if let Err(e) = fb.blit_rgba(desktop_x, desktop_y, width, height, &rgba) {
                errors.push(format!("egfx blit error: {e}"));
            } else {
                fb.mark_dirty_pending();
            }
        }
        PendingPaint::Present => {
            if let Err(e) = fb.flush_render() {
                errors.push(format!("flush_render error: {e}"));
            }
            *presents += 1;
        }
    }
}

fn summarize(msg: &InboundMessage) -> String {
    use InboundMessage::*;
    match msg {
        FastPathPdu(p) => format!("FastPathPdu({} bytes)", p.pdu.len()),
        EgfxBitmap(b) => format!(
            "EgfxBitmap({}+{}x{} @ {},{}; {} bytes)",
            b.width,
            b.height,
            b.height,
            b.desktop_x,
            b.desktop_y,
            b.rgba.len()
        ),
        EgfxAvcFrame(f) => format!(
            "EgfxAvcFrame(surface={} {}x{} @ {},{}; codec={:#x} enc={} luma={}B chroma={}B)",
            f.surface_id,
            f.dest_width,
            f.dest_height,
            f.desktop_x,
            f.desktop_y,
            f.codec_id,
            f.encoding,
            f.luma_h264.len(),
            f.chroma_h264.len()
        ),
        EgfxClearCodec(c) => format!(
            "EgfxClearCodec(surface={} {}x{} @ {},{}; pdu={}B)",
            c.surface_id,
            c.width,
            c.height,
            c.dest_x,
            c.dest_y,
            c.pdu_data.len()
        ),
        EgfxUncompressed(u) => format!(
            "EgfxUncompressed(surface={} {}x{} @ {},{}; pixfmt={:#x} bytes={})",
            u.surface_id, u.width, u.height, u.dest_x, u.dest_y,
            u.pixel_format, u.bitmap_data.len(),
        ),
        EgfxPlanar(p) => format!(
            "EgfxPlanar(surface={} {}x{} @ {},{}; pdu={}B)",
            p.surface_id, p.width, p.height, p.dest_x, p.dest_y, p.pdu_data.len(),
        ),
        EgfxAvc420(a) => format!(
            "EgfxAvc420(surface={} {}x{} @ {},{}; pdu={}B)",
            a.surface_id, a.width, a.height, a.dest_x, a.dest_y, a.pdu_data.len(),
        ),
        EgfxSolidFill(s) => format!(
            "EgfxSolidFill(surface={} color=({},{},{}) rects={})",
            s.surface_id, s.color_r, s.color_g, s.color_b, s.rects.len(),
        ),
        EgfxSurfaceToCache(c) => format!(
            "EgfxSurfaceToCache(surface={} slot={} key={:#x} src=({},{},{},{}))",
            c.surface_id, c.cache_slot, c.cache_key,
            c.source_rect.left, c.source_rect.top, c.source_rect.right, c.source_rect.bottom,
        ),
        EgfxCacheToSurface(c) => format!(
            "EgfxCacheToSurface(surface={} slot={} points={})",
            c.surface_id, c.cache_slot, c.dest_points.len(),
        ),
        EgfxEvictCacheEntry(e) => format!("EgfxEvictCacheEntry(slot={})", e.cache_slot),
        EgfxSurfaceToSurface(s) => format!(
            "EgfxSurfaceToSurface(src={} dst={} src_rect=({},{},{},{}) points={})",
            s.source_surface_id, s.destination_surface_id,
            s.source_rect.left, s.source_rect.top, s.source_rect.right, s.source_rect.bottom,
            s.dest_points.len(),
        ),
        EgfxWireToSurface2(w) => format!(
            "EgfxWireToSurface2(surface={} ctx={} codec={:#x} pixfmt={} origin=({},{}) bytes={})",
            w.surface_id, w.codec_context_id, w.codec_id, w.pixel_format,
            w.surface_origin_x, w.surface_origin_y, w.bitmap_data.len(),
        ),
        EgfxDeleteEncodingContext(d) => format!(
            "EgfxDeleteEncodingContext(surface={} ctx={})",
            d.surface_id, d.codec_context_id,
        ),
        PngFrame(p) => format!(
            "PngFrame({},{} -> {},{}; {} bytes)",
            p.rect.left,
            p.rect.top,
            p.rect.right,
            p.rect.bottom,
            p.png.len()
        ),
        Png2Frame(p) => format!(
            "Png2Frame({},{} -> {},{}; {} bytes)",
            p.rect.left,
            p.rect.top,
            p.rect.right,
            p.rect.bottom,
            p.png.len()
        ),
        RdpResponsePdu(r) => format!("RdpResponsePdu({} bytes)", r.response.len()),
        ConnectionActivated(c) => format!(
            "ConnectionActivated(io={}, user={}, {}x{})",
            c.io_channel_id, c.user_channel_id, c.screen_width, c.screen_height
        ),
        ServerHello(h) => format!(
            "ServerHello(clipboard={}, dir_remove={}, hidpi={}, multi_monitor={}, activation={}x{})",
            h.clipboard_enabled,
            h.directory_remove_supported,
            h.hidpi_supported,
            h.multi_monitor_supported,
            h.activation.screen_width,
            h.activation.screen_height
        ),
        Alert(a) => format!("Alert({:?}, {:?})", a.severity, a.message),
        ClipboardIn(c) => format!("ClipboardIn({} chars)", c.data.chars().count()),
        LatencyStats(l) => format!(
            "LatencyStats(client={}ms, server={}ms)",
            l.client_ms, l.server_ms
        ),
        Ping(p) => format!("Ping({} bytes)", p.uuid.len()),
        ScreenSpec(s) => format!("ScreenSpec({}x{} @ scale={})", s.width, s.height, s.scale),
        MfaChallenge(_) => "MfaChallenge(..)".to_owned(),
        MouseMove(m) => format!("MouseMove({}, {})", m.x, m.y),
        MouseButton(m) => format!("MouseButton({:?}, pressed={})", m.button, m.pressed),
        MouseWheel(m) => format!("MouseWheel({:?}, delta={})", m.axis, m.delta),
        KeyboardButton(k) => format!("KeyboardButton(code={}, pressed={})", k.key_code, k.pressed),
        SyncKeys(_) => "SyncKeys(..)".to_owned(),
        ShareDirRequest(_) => "ShareDirRequest(..)".to_owned(),
        ShareDirResponse(_) => "ShareDirResponse(..)".to_owned(),
        ShareDirAck(_) => "ShareDirAck(..)".to_owned(),
        ShareDirAnnounce(_) => "ShareDirAnnounce(..)".to_owned(),
        ShareDirRemove(_) => "ShareDirRemove(..)".to_owned(),
        SessionSelection(_) => "SessionSelection(..)".to_owned(),
        ClientHello(_) => "ClientHello(..)".to_owned(),
        TdpbUpgrade => "TdpbUpgrade".to_owned(),
        RefreshRect(r) => format!(
            "RefreshRect({},{} -> {},{})",
            r.left, r.top, r.right, r.bottom
        ),
        EgfxEndFrame(e) => format!("EgfxEndFrame(frame={})", e.frame_id),
        Unsupported(u) => format!("Unsupported(tdp_type={})", u.tdp_type),
    }
}

/// Byte offset of the H.264 payload inside an Avc420 `RFX_AVC420_BITMAP_STREAM`
/// ([MS-RDPEGFX] 2.2.4.4.1). The stream opens with a region header:
///   numRegionRects      u32 LE
///   regionRects[N]       N * 8 bytes  (RDPGFX_RECT16: left/top/right/bottom, u16 each)
///   quantQualityVals[N]  N * 2 bytes  (RDPGFX_AVC420_QUANT_QUALITY: qpVal + qualityVal)
/// then the H.264 bitstream. Returns the payload offset, or an error string
/// describing why the header is unusable.
fn avc420_h264_offset(pdu_data: &[u8]) -> Result<usize, String> {
    if pdu_data.len() < 4 {
        return Err("avc420 PDU too short for header".to_string());
    }
    let num_rects = u32::from_le_bytes([pdu_data[0], pdu_data[1], pdu_data[2], pdu_data[3]]);
    if num_rects == 0 || num_rects > 16 {
        return Err(format!("avc420 numRegionRects out of range: {num_rects}"));
    }
    let header_size = 4 + (num_rects as usize) * (8 /* RDPGFX_RECT16 */ + 2 /* AVC420_QUANT_QUALITY */);
    if pdu_data.len() <= header_size {
        return Err(format!(
            "avc420 PDU truncated: {} bytes, need more than {} for header",
            pdu_data.len(),
            header_size
        ));
    }
    Ok(header_size)
}

#[cfg(test)]
mod avc_tests {
    use super::avc420_h264_offset;

    #[test]
    fn avc420_offset_single_region_skips_rect_plus_2byte_quant() {
        // N=1: 4 (numRegionRects) + 8 (RDPGFX_RECT16) + 2 (AVC420_QUANT_QUALITY) = 14.
        // RDPGFX_AVC420_QUANT_QUALITY is 2 bytes (qpVal + qualityVal), NOT 4 — see
        // ironrdp-egfx QuantQuality::FIXED_PART_SIZE = 1 + 1.
        let mut data = Vec::new();
        data.extend_from_slice(&1u32.to_le_bytes()); // numRegionRects = 1
        data.extend_from_slice(&[0x00, 0x00, 0x00, 0x00, 0x40, 0x06, 0x40, 0x02]); // one RDPGFX_RECT16
        data.extend_from_slice(&[0x33, 0x00]); // one AVC420_QUANT_QUALITY (qpVal, qualityVal)
        let h264 = [0x00u8, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1f]; // Annex-B SPS NAL
        data.extend_from_slice(&h264);

        let off = avc420_h264_offset(&data).expect("valid avc420 header");
        assert_eq!(off, 14, "header must be 4 + 1*(8+2) = 14 bytes, not 16");
        assert_eq!(
            &data[off..],
            &h264,
            "payload must start exactly at the H.264 NAL, not 2 bytes into it"
        );
    }

    #[test]
    fn avc420_offset_rejects_truncated() {
        // Only numRegionRects=1, no rect/quant/payload bytes.
        let data = 1u32.to_le_bytes().to_vec();
        assert!(avc420_h264_offset(&data).is_err());
    }

    #[test]
    fn avc420_offset_rejects_zero_rects() {
        let mut data = 0u32.to_le_bytes().to_vec();
        data.extend_from_slice(&[0xFF; 16]);
        assert!(avc420_h264_offset(&data).is_err());
    }
}

#[cfg(test)]
mod sync_keys_tests {
    use super::MainCodec;

    #[test]
    fn encode_sync_keys_legacy_wire_bytes() {
        // Legacy TDP SyncKeys: | type (32) | scroll | num | caps | kana |,
        // one byte per lock key, 1 = down (mirrors tdp/codec.ts SYNC_KEYS).
        let codec = MainCodec::new();
        let bytes = match codec.encode_sync_keys(false, true, true, false) {
            Ok(b) => b,
            Err(_) => panic!("legacy encode_sync_keys failed"),
        };
        assert_eq!(bytes, vec![32, 0, 1, 1, 0]);
    }

    #[test]
    fn encode_sync_keys_tdpb_matches_codec_crate() {
        use codec::messages::{ButtonState, SyncKeys};

        let main = MainCodec::new();
        main.upgrade_to_tdpb();
        let bytes = match main.encode_sync_keys(false, false, true, false) {
            Ok(b) => b,
            Err(_) => panic!("tdpb encode_sync_keys failed"),
        };
        let mut reference = codec::Codec::new();
        reference.upgrade_to_tdpb();
        let expected = reference
            .sync_keys(&SyncKeys {
                scroll_lock: ButtonState::Up,
                num_lock: ButtonState::Up,
                caps_lock: ButtonState::Down,
                kana_lock: ButtonState::Up,
            })
            .expect("reference tdpb encode");
        assert!(!bytes.is_empty());
        assert_eq!(bytes, expected);
    }
}
