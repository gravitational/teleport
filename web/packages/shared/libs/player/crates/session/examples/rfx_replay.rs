//! Headless RFX-Progressive decode replay for per-stage timing (native).
//!
//! Replays the captured `WireToSurface2` fixtures (default
//! `/tmp/teleport-rfx-fixtures`, override with `TELEPORT_RFX_FIXTURES`) through
//! `ProgressiveDecoder::decode_bitmap` and reports the entropy+dequant /
//! inverse-DWT / YCbCr→RGB split via the `ironrdp-graphics` stage-timing hook.
//! No FreeRDP, no pixel diff — just the decode, so a profiler/flamegraph (or
//! these accumulators) isolate the RFX cost.
//!
//! This measures NATIVE codegen, not wasm. The wasm decode worker (the actually
//! pegged thread) does not auto-vectorize the IDWT/color loops the way native
//! release does, so treat this as a cross-check of the live `[apply]` rfx{...}
//! breakdown, not a substitute for it.
//!
//! Run:
//!   cargo run --release -p session --example rfx_replay --features rfx-stage-timing
//!   RFX_REPLAY_ITERS=200 TELEPORT_RFX_FIXTURES=/path cargo run --release ...
//!
//! Flamegraph cross-check (needs perf + `cargo install flamegraph`):
//!   cargo flamegraph -p session --example rfx_replay --features rfx-stage-timing

use std::fs;
use std::path::PathBuf;
use std::sync::OnceLock;
use std::time::Instant;

use ironrdp_graphics::progressive::{stage_timing, ProgressiveDecoder};

// Geometry of the capture (3200x660 @ 200% in the repro that produced the
// committed fixtures). Adjust to match your manifest if you recaptured.
const SURFACE_W: u16 = 3200;
const SURFACE_H: u16 = 660;

static START: OnceLock<Instant> = OnceLock::new();

/// Monotonic millisecond clock for the stage-timing hook (a plain `fn` so it
/// coerces to `fn() -> f64`; no captured state).
fn now_ms() -> f64 {
    START.get_or_init(Instant::now).elapsed().as_secs_f64() * 1000.0
}

struct Fixture {
    surface_id: u32,
    ctx_id: u32,
    payload: Vec<u8>,
}

fn load_fixtures(dir: &PathBuf) -> Vec<Fixture> {
    let manifest = fs::read_to_string(dir.join("manifest.tsv"))
        .unwrap_or_else(|e| panic!("read {}/manifest.tsv: {e}", dir.display()));
    let mut out = Vec::new();
    for line in manifest.lines() {
        // seq ts_ns surface codec_id ctx pixel_format origin_x origin_y bytes file
        let c: Vec<&str> = line.split('\t').collect();
        if c.len() < 10 {
            continue;
        }
        let surface_id: u32 = match c[2].parse() {
            Ok(v) => v,
            Err(_) => continue, // header row / malformed
        };
        let ctx_id: u32 = c[4].parse().unwrap_or(0);
        let payload = fs::read(dir.join(c[9])).unwrap_or_else(|e| panic!("read fixture {}: {e}", c[9]));
        out.push(Fixture { surface_id, ctx_id, payload });
    }
    out
}

fn main() {
    let dir = std::env::var_os("TELEPORT_RFX_FIXTURES")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/tmp/teleport-rfx-fixtures"));
    let iters: usize = std::env::var("RFX_REPLAY_ITERS")
        .ok()
        .and_then(|s| s.parse().ok())
        .unwrap_or(50);

    let all = load_fixtures(&dir);
    assert!(!all.is_empty(), "no fixtures in {}", dir.display());

    // Single-surface replay: drive off the first surface_id (the captures are a
    // single surface), so one stateful decoder matches live behavior.
    let surface_id = all[0].surface_id;
    let seq: Vec<&Fixture> = all.iter().filter(|f| f.surface_id == surface_id).collect();
    let total_bytes: usize = seq.iter().map(|f| f.payload.len()).sum();

    stage_timing::set_now_hook(now_ms);

    println!(
        "[rfx_replay] {} PDUs, surface={surface_id}, {SURFACE_W}x{SURFACE_H}, {iters} iters, {} KiB/iter",
        seq.len(),
        total_bytes / 1024,
    );

    // Replay the captured PDU sequence `iters` times, each from a fresh decoder
    // (faithful re-decode of the capture; the decoder is stateful across PDUs).
    stage_timing::reset();
    let mut tile_count = 0usize;
    let wall = Instant::now();
    for _ in 0..iters {
        let mut dec = ProgressiveDecoder::new();
        for f in &seq {
            match dec.decode_bitmap(f.ctx_id, SURFACE_W, SURFACE_H, &f.payload) {
                Ok(tiles) => {
                    tile_count += tiles.len();
                    dec.reclaim(tiles);
                }
                Err(e) => panic!("decode failed: {e:?}"),
            }
        }
    }
    let wall_ms = wall.elapsed().as_secs_f64() * 1000.0;
    let st = stage_timing::take();

    let staged = st.entropy_ms + st.idwt_ms + st.color_ms;
    let pct = |x: f64| if staged > 0.0 { x / staged * 100.0 } else { 0.0 };
    println!("[rfx_replay] wall={wall_ms:.1}ms, decoded {tile_count} tiles");
    println!("[rfx_replay]   entropy+dequant = {:7.1}ms  ({:4.1}%)", st.entropy_ms, pct(st.entropy_ms));
    println!("[rfx_replay]   inverse-DWT     = {:7.1}ms  ({:4.1}%)", st.idwt_ms, pct(st.idwt_ms));
    println!("[rfx_replay]   YCbCr->RGB      = {:7.1}ms  ({:4.1}%)", st.color_ms, pct(st.color_ms));
    println!(
        "[rfx_replay]   staged total    = {staged:7.1}ms  (untimed parse/accum/pool = {:.1}ms)",
        wall_ms - staged,
    );
    if staged > 0.0 {
        // Entropy (RLGR/SRL/raw-bit) is inherently serial / unvectorizable; the
        // IDWT + color stages are the SIMD-amenable fraction. This is the Amdahl
        // input the kickoff wants from Step 1.
        let serial = st.entropy_ms / staged;
        let simd_amenable = (st.idwt_ms + st.color_ms) / staged;
        println!(
            "[rfx_replay]   serial(entropy)={:.1}%  SIMD-amenable(idwt+color)={:.1}%",
            serial * 100.0,
            simd_amenable * 100.0,
        );
    }
}
