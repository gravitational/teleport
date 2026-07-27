//! Per-stage timing for the live session hot path.
//!
//! Mirrors the source player's `FrameStats` log line so a side-by-side
//! comparison reads off the same column names, but tracks the finer-grained
//! stages we actually want to optimize:
//!
//!   codec_decode    prost-decode of one inbound Envelope
//!   ironrdp_process IronRDP FastPathProcessor::process (RFX decode + image writes)
//!   pixel_copy      wasm-memory pixels → cached Uint8ClampedArray slice
//!   put_image       ctx.putImageData with the dirty rect
//!   response_send   prost-encode + ws.send of the per-PDU RDP response (when non-empty)
//!   cursor_post     post_cursor() — ImageData alloc + structured-clone over postMessage
//!   mouse_encode    prost-encode of an outbound MouseMove
//!   mouse_send      Uint8Array::from + ws.send_with_array_buffer
//!
//! Each stage carries sum/max/count so we can derive mean and tail per flush.
//! A flush happens roughly once a second, posted to JS as a `Perf` message.

use std::cell::RefCell;

use js_sys::Reflect;
use wasm_bindgen::{JsCast, JsValue};
use web_sys::{DedicatedWorkerGlobalScope, Performance};

const FLUSH_INTERVAL_MS: f64 = 1000.0;

#[derive(Default, Debug, Clone, Copy)]
struct Stage {
    count: u32,
    sum_ms: f64,
    max_ms: f64,
}

impl Stage {
    fn record(&mut self, ms: f64) {
        self.count += 1;
        self.sum_ms += ms;
        if ms > self.max_ms {
            self.max_ms = ms;
        }
    }

    fn write(self, out: &js_sys::Object, prefix: &str) {
        let mean = if self.count == 0 {
            0.0
        } else {
            self.sum_ms / f64::from(self.count)
        };
        set_num(out, &format!("{prefix}_n"), f64::from(self.count));
        set_num(out, &format!("{prefix}_mean_ms"), mean);
        set_num(out, &format!("{prefix}_max_ms"), self.max_ms);
    }
}

/// Coarse classification of a FastPath PDU. Mirrors the IronRDP
/// `update_code` byte; "other" covers synchronize/palette/etc. that
/// aren't worth their own bucket.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PduClass {
    SurfaceCommands,
    Bitmap,
    Pointer,
    Other,
}

impl PduClass {
    fn as_str(self) -> &'static str {
        match self {
            PduClass::SurfaceCommands => "surface",
            PduClass::Bitmap => "bitmap",
            PduClass::Pointer => "pointer",
            PduClass::Other => "other",
        }
    }
}

#[derive(Default, Debug)]
pub struct PerfStats {
    start: f64,
    last_flush: f64,

    // Inbound stages.
    codec_decode: Stage,
    ironrdp_process: Stage,
    process_surface: Stage,
    process_bitmap: Stage,
    process_pointer: Stage,
    process_other: Stage,
    pixel_copy: Stage,
    put_image: Stage,
    response_send: Stage,
    cursor_post: Stage,

    // Misc counters. Outbound (mouse_*, m_queue) lives on main in TS
    // since A.2 — encode + send happen there with no worker hop.
    pdus: u32,
    paints: u32,
    dirty_pixels: u64, // sum of (dw*dh) per paint
    response_bytes: u64,
    // Total wall-clock ms spent inside `feed_bytes` (decode + apply) this
    // interval. busy_ms/elapsed_ms ≈ worker CPU utilization for inbound work —
    // distinguishes "client can't keep up" from "server only sends N fps".
    busy_ms: f64,
    // busy_ms split by op category, to find which EGFX op dominates the CPU.
    apply_clear_ms: f64,
    apply_c2s_ms: f64,
    apply_s2c_ms: f64,
    apply_rfx_ms: f64,
    apply_other_ms: f64,

    // Within-rfx breakdown (feature `rfx-stage-timing`): where the wasm decode
    // thread actually spends the rfx time — entropy+dequant (serial), inverse
    // DWT, YCbCr→RGB color convert, and the framebuffer blit. The input to the
    // "parallelize across tiles vs SIMD" decision.
    #[cfg(feature = "rfx-stage-timing")]
    rfx_entropy_ms: f64,
    #[cfg(feature = "rfx-stage-timing")]
    rfx_idwt_ms: f64,
    #[cfg(feature = "rfx-stage-timing")]
    rfx_color_ms: f64,
    #[cfg(feature = "rfx-stage-timing")]
    rfx_blit_ms: f64,
}

thread_local! {
    static STATS: RefCell<PerfStats> = RefCell::new(PerfStats::new());
}

pub fn record_decode(ms: f64) {
    STATS.with(|s| s.borrow_mut().codec_decode.record(ms));
}

pub fn record_process(ms: f64, class: PduClass) {
    STATS.with(|s| {
        let mut s = s.borrow_mut();
        s.ironrdp_process.record(ms);
        s.pdus += 1;
        match class {
            PduClass::SurfaceCommands => s.process_surface.record(ms),
            PduClass::Bitmap => s.process_bitmap.record(ms),
            PduClass::Pointer => s.process_pointer.record(ms),
            PduClass::Other => s.process_other.record(ms),
        }
    });
}

/// Threshold (ms) above which a process call is logged with its update
/// code so the user can spot what causes the long-tail spikes.
const SLOW_PDU_THRESHOLD_MS: f64 = 8.0;

/// Post a `slow-pdu` notification to main when a process call exceeds
/// the threshold. Called once per slow PDU; not buffered, since slow
/// PDUs are by definition rare.
pub fn maybe_report_slow_pdu(ms: f64, class: PduClass, pdu_len: usize) {
    if ms < SLOW_PDU_THRESHOLD_MS {
        return;
    }
    let obj = js_sys::Object::new();
    let _ = Reflect::set(&obj, &"type".into(), &"slowPdu".into());
    let _ = Reflect::set(&obj, &"class".into(), &JsValue::from_str(class.as_str()));
    set_num(&obj, "ms", ms);
    set_num(&obj, "len", pdu_len as f64);
    let _ = worker_scope().post_message(&obj);
}

// `record_pixel_copy` was the canvas2D-era wasm→ImageData step. WebGL2's
// `texSubImage2D` rolls upload + draw into one timed region (`put_image`),
// so the stage now always reports zero for the new client. The field is
// still in the perf payload so the [perf-old] line aligns column-for-
// column (it's also always zero there — see tdp/perfLogger.ts).

pub fn record_put_image(ms: f64, dirty_w: u32, dirty_h: u32) {
    STATS.with(|s| {
        let mut s = s.borrow_mut();
        s.put_image.record(ms);
        s.paints += 1;
        s.dirty_pixels += u64::from(dirty_w) * u64::from(dirty_h);
    });
}

pub fn record_response_send(ms: f64, bytes: usize) {
    STATS.with(|s| {
        let mut s = s.borrow_mut();
        s.response_send.record(ms);
        // bytes can be large — accumulate as u64 to avoid overflow over 1s.
        s.response_bytes += bytes as u64;
    });
}

pub fn record_cursor_post(ms: f64) {
    STATS.with(|s| s.borrow_mut().cursor_post.record(ms));
}

/// Accumulate time spent processing one inbound wire message (decode + apply).
pub fn record_busy(ms: f64) {
    STATS.with(|s| s.borrow_mut().busy_ms += ms);
}

/// Which EGFX op a wire message maps to, for the per-category time breakdown.
#[derive(Debug, Clone, Copy)]
pub enum ApplyCat {
    Clear,
    C2s,
    S2c,
    Rfx,
    Other,
}

/// Attribute one message's processing time to its op category.
pub fn record_apply(cat: ApplyCat, ms: f64) {
    STATS.with(|s| {
        let mut s = s.borrow_mut();
        match cat {
            ApplyCat::Clear => s.apply_clear_ms += ms,
            ApplyCat::C2s => s.apply_c2s_ms += ms,
            ApplyCat::S2c => s.apply_s2c_ms += ms,
            ApplyCat::Rfx => s.apply_rfx_ms += ms,
            ApplyCat::Other => s.apply_other_ms += ms,
        }
    });
}

/// Attribute one RFX-Progressive PDU's per-stage decode breakdown (feature
/// `rfx-stage-timing`): entropy+dequant / inverse-DWT / YCbCr→RGB from the
/// ironrdp-graphics stage-timing hook, plus the framebuffer blit. These are a
/// breakdown WITHIN `apply_rfx_ms`, not a partition of it (decode dispatch +
/// dirty/probe overhead live outside the timed stages), so their sum is < rfx.
#[cfg(feature = "rfx-stage-timing")]
pub fn record_rfx_stages(entropy_ms: f64, idwt_ms: f64, color_ms: f64, blit_ms: f64) {
    STATS.with(|s| {
        let mut s = s.borrow_mut();
        s.rfx_entropy_ms += entropy_ms;
        s.rfx_idwt_ms += idwt_ms;
        s.rfx_color_ms += color_ms;
        s.rfx_blit_ms += blit_ms;
    });
}

pub fn maybe_flush() {
    let now = performance_now();
    let payload = STATS.with(|s| {
        let mut s = s.borrow_mut();
        if s.last_flush == 0.0 {
            s.start = now;
            s.last_flush = now;
            return None;
        }
        let elapsed_ms = now - s.last_flush;
        if elapsed_ms < FLUSH_INTERVAL_MS {
            return None;
        }
        let payload = s.snapshot(elapsed_ms);
        s.reset(now);
        Some(payload)
    });
    if let Some(obj) = payload {
        let _ = worker_scope().post_message(&obj);
    }
}

impl PerfStats {
    fn new() -> Self {
        Self::default()
    }

    fn snapshot(&self, elapsed_ms: f64) -> JsValue {
        let obj = js_sys::Object::new();
        let _ = Reflect::set(&obj, &"type".into(), &"perf".into());
        set_num(&obj, "elapsed_ms", elapsed_ms);
        set_num(&obj, "pdus", f64::from(self.pdus));
        set_num(&obj, "paints", f64::from(self.paints));
        // dirty_pixels / response_bytes may exceed u32; coerce through f64
        // which is what we serialize over postMessage anyway.
        #[allow(clippy::cast_precision_loss)]
        let dirty_pixels_f = self.dirty_pixels as f64;
        #[allow(clippy::cast_precision_loss)]
        let response_bytes_f = self.response_bytes as f64;
        set_num(&obj, "dirty_pixels", dirty_pixels_f);
        set_num(&obj, "response_bytes", response_bytes_f);
        set_num(&obj, "busy_ms", self.busy_ms);
        set_num(&obj, "apply_clear_ms", self.apply_clear_ms);
        set_num(&obj, "apply_c2s_ms", self.apply_c2s_ms);
        set_num(&obj, "apply_s2c_ms", self.apply_s2c_ms);
        set_num(&obj, "apply_rfx_ms", self.apply_rfx_ms);
        set_num(&obj, "apply_other_ms", self.apply_other_ms);
        #[cfg(feature = "rfx-stage-timing")]
        {
            set_num(&obj, "rfx_entropy_ms", self.rfx_entropy_ms);
            set_num(&obj, "rfx_idwt_ms", self.rfx_idwt_ms);
            set_num(&obj, "rfx_color_ms", self.rfx_color_ms);
            set_num(&obj, "rfx_blit_ms", self.rfx_blit_ms);
        }

        self.codec_decode.write(&obj, "codec_decode");
        self.ironrdp_process.write(&obj, "ironrdp_process");
        self.process_surface.write(&obj, "process_surface");
        self.process_bitmap.write(&obj, "process_bitmap");
        self.process_pointer.write(&obj, "process_pointer");
        self.process_other.write(&obj, "process_other");
        self.pixel_copy.write(&obj, "pixel_copy");
        self.put_image.write(&obj, "put_image");
        self.response_send.write(&obj, "response_send");
        self.cursor_post.write(&obj, "cursor_post");

        obj.into()
    }

    fn reset(&mut self, now: f64) {
        // Preserve `start` so consumers can compute uptime if they care.
        let start = self.start;
        *self = PerfStats {
            start,
            last_flush: now,
            ..PerfStats::default()
        };
    }
}

fn set_num(o: &js_sys::Object, key: &str, v: f64) {
    let _ = Reflect::set(o, &JsValue::from_str(key), &JsValue::from_f64(v));
}

thread_local! {
    // Resolve the worker's `Performance` once. Re-fetching
    // `global().performance()` on every call crosses the JS boundary ~3× and
    // this runs twice per `Timer` (new + drop) plus once per `feed_bytes`
    // (`maybe_flush`) — i.e. on the hottest path.
    static PERFORMANCE: Option<Performance> = worker_scope().performance();
}

pub fn performance_now() -> f64 {
    PERFORMANCE.with(|p| p.as_ref().map_or(0.0, Performance::now))
}

fn worker_scope() -> DedicatedWorkerGlobalScope {
    js_sys::global().unchecked_into()
}

/// RAII timer that records the elapsed ms when dropped.
pub struct Timer {
    start: f64,
    sink: fn(f64),
}

impl Timer {
    #[must_use]
    pub fn new(sink: fn(f64)) -> Self {
        Self {
            start: performance_now(),
            sink,
        }
    }
}

impl Drop for Timer {
    fn drop(&mut self) {
        (self.sink)(performance_now() - self.start);
    }
}
