# RFX-Progressive decode ceiling — CPU-SIMD vs GPU bake-off (HANDOFF)

**Goal:** make RFX-Progressive video decode "as good as possible, even with
multiple monitors/videos." On the test host AVC/H.264 is unavailable (see §1), so
full-motion video arrives as **RFX-Progressive** and the client-side decode is the
ceiling. This doc hands off a **head-to-head bake-off** between the two candidate
ceiling-raisers, with the measurement tooling already wired so you decide on real
numbers, not estimates.

> TL;DR decision state: the release build already took us from ~5fps → ~45fps for
> one video. The remaining question is *multi-video scaling*. A GPU IDWT-offload
> microbench showed it's **upload-bound** (~1.6 videos, ~2.4 with R16I) — only
> modestly ahead of an estimated CPU **SIMD-IDWT** (~1.5–2 videos) at far higher
> cost/risk. So: **benchmark both for real, then pick.** This doc tells you how.

---

## MEASURED RESULTS (2026-06-17) — both levers benchmarked on this 16" MBP (DPR 2)

**Lever B (GPU offload), `gpuBench` R16I:** upload is linear ~0.14 ms/MiB. Full-screen
2730-tile frame ≈ 11.5 ms → **~236k tiles/s** plumbing; windowed 510/240-tile → 8.5 / 11.2
frames-per-22ms. Upload-bound for full-motion full-screen (~2-video ceiling); shines for
windowed. Realizing it needs the per-surface GPU-composite architecture (Increment 2, §9.2)
+ a pool→main coefficient transfer; the number is idle-GPU-optimistic.

**Lever A (CPU SIMD inverse-DWT), measured via `perfAB` (release, same-session A/B):**
idwt **49.0 → 37.7 µs/tile (1.30×)**, per-tile 86.3 → 74.5 µs (1.16×) → 8-worker pool
**~93k → ~107k tiles/s (+16%)**. Bit-exact (8 oracle buffers + 30,002 randomized tiles,
`max|Δ|==0`) and visually clean (wasm v128 path confirmed). The 1.30× (vs the 2.6–4×
literature hope) is v128 gather/scatter overhead from vectorizing across strided rows/cols.
SHIPPED: zero architecture change, drops into the existing decode pool, composes with the §5
levers. Toggle live via `setSimdIdwt(bool)`; A/B via `await perfAB()`.

**Bottleneck caveat (decisive):** the single-video `perfAB` run was **arrival-bound**
(`inflight=1`, `present.p50 ≈ arrival ≈ 49 ms / ~20 fps`) — decode was NOT the limiter, so
neither lever moved on-screen fps. The decode-bound (multi-video) regime, where either lever
matters, was not reached. Per this doc's own rule (§2a), arrival≈present ⇒ stop optimizing
decode until proven decode-bound. Note: neither pool capacity (93k or 107k) sustains even ONE
2730-tile "More Space" full-screen video at 45fps (needs ~123k tiles/s).

**Verdict:** Keep Lever A (free, safe, bit-exact +16%; stacks with everything). Lever B has a
~2.2× raw-throughput edge but is gated behind the Increment-2 architecture and is upload-bound
for full-screen full-motion — NOT a slam-dunk. **Decide GPU-vs-not only after the multi-video
scale test** confirms decode (not arrival/network, not the all-N barrier §5) is the real
ceiling. If decode-bound → GPU's edge justifies the architecture work, with SIMD as the cheap
interim. If arrival-bound or the §5 barrier-kill / work-stealing / pool>8 levers fix it → the
GPU rewrite is premature.

**SERVER-SIDE ROOT CAUSE — CONFIRMED via `docker logs rdpclient-windows-1` (2026-06-17):**
The full-screen ceiling is the **RFX-Progressive supply rate**, not client decode. Logs:
- `capabilities confirmed by server: V10_7 { flags: AVC_DISABLED }` — host refuses AVC because
  the surface is **6400×2560** (≫ the ~4096 H.264 width cap). Video is RFX-Progressive (4000 tiles/frame).
- `frame_complete` cadence: 120 frames per ~6–8 s → **~17 fps steady** — Windows encodes/sends
  the giant RFX surface at ~17fps; the client present (~20fps, `inflight=1`) just tracks it.

The wall is **Windows RFX encode + bandwidth at a 6400×2560 HiDPI surface** (the "flooded the link
with full-region progressive deltas, ~4× worse at HiDPI" that `lib/srv/desktop/rdp/rdpclient/src/egfx.rs:151`
already warned about). Arriving tile rate ≈ 4000 × 17 ≈ **68k tiles/s** — under both decode ceilings
(SIMD 107k, GPU 236k), hence `inflight=1`. **No decode lever can raise this** — frames don't arrive
faster. The lever for full-screen-video fps is **AVC/H.264 for the video**, which on this host needs
sub-4096 surfaces → **multi-monitor each ≤4096** (§10). Lever A (SIMD) stays a genuine win for
decode-bound regimes (windowed video; or once AVC unclogs the pipe and the pool is the bottleneck again).

---

## 0. Two parallel tracks (read this first)

There are **two** independent bodies of work. Don't conflate them.

- **Track A — RFX decode performance (ACTIVE).** Make RFX video decode fast/scalable.
  This is §1–§7 (the bake-off). It applies *today* because video is RFX on this host.
- **Track B — native-parity rendering / AVC (PAUSED, gated).** Match how mstsc/native
  composites mixed AVC+RFX (one present clock, per-surface textures, AVC444 chroma,
  frame-ack pacing). This is §9. It only *matters once AVC actually flows*, which is
  blocked by the 4112-px width gate (§10). Increment 1 of it was tried and **reverted**
  (stash@{0}). Do **not** resume Track B until AVC is enabled (§10) — until then there's
  no AVC to unify or recombine, and the "boundary artifacts" it fixes don't occur.

Current working-tree state (what's done / in-flight / stashed) is §8.

---

## 1. Background you must not re-discover

- **AVC is off because of resolution, not a bug.** The Windows host gates AVC on
  monitor width: our single surface is **4112 px wide** (`window.innerWidth ×
  devicePixelRatio`, the codec-test hard-codes DPR scale — `DesktopSessionCodecTest.tsx:54`),
  which exceeds the host H.264 encoder's ~4096/stream cap, so the host confirms
  `V10_7 { AVC_DISABLED }` and sends **RFX for everything, including video**.
  - Verify on the node: `docker logs rdpclient-windows-1 2>&1 | grep -E 'advertising|confirmed'`.
    `rdpclient-windows-1` is the Teleport **Windows Desktop Service** (runs the Rust
    RDP client, logs `[teleport][egfx]`); it connects to a real Windows machine.
  - To get AVC back (out of scope here): multi-monitor each ≤4096, or software-AVC
    GPO (`HKLM\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services\AVC444ModePreferred=1`,
    forces software AVC, host CPU cost), or HEVC. Not the focus — we're optimizing RFX.

- **The single biggest win was already shipped: a RELEASE wasm build.** The dev
  loop was shipping an **unoptimized `opt-level=0` debug** wasm (`build-release-wasm.sh`
  *defaulted to debug*). Release (`-O3` + fat-LTO + wasm-opt + simd128) was ~8×:
  present p50 **~170ms (5fps) → ~22ms (45fps)**.
  - **ALWAYS perf-test the release build.** `./build-release-wasm.sh release`
    (now falls back gracefully if `wasm-opt`/binaryen chokes). The script still
    *defaults to debug* for fast iteration — do not measure perf on it.

- **Per-tile decode stage split (release, real offload path)** — the number that
  decides what to optimize:

  | stage | µs/tile | share | where it can go |
  |---|---|---|---|
  | **inverse-DWT** | 48.1 | **56%** | CPU SIMD, or GPU |
  | entropy (RLGR + dequant) | 30.7 | 36% | CPU only (serial; GPU-hostile) |
  | color (YCbCr→RGB) | 7.0 | 8% | CPU SIMD, or GPU (free ride) |

  One full-screen video ≈ 1600 tiles/frame (4112×1600 = 64×25 tiles); 8-worker pool
  ≈ 93k tiles/s capacity vs ~72k demand → **one video keeps up, a second saturates.**

- **GPU offload de-risk bench result (the pivot point):**
  `gpuBench()` → `total=13.49ms/frame (upload≈8.97, passes≈4.52), 75MiB/frame (R32I),
  framesPer22ms=1.63`. **Upload is the wall (66%), compute is cheap.** This is what
  the stage-split estimate could not tell us, and it's why we're now comparing
  against CPU SIMD instead of just building the moonshot.

---

## 2. The measurement toolkit — `L.*()` and `gpuBench()`

All of this is **already wired**. Use it to baseline before and measure after each
change. The logsink (`logsink.ts`) patches `console` in both main and the worker and
exposes a global `L` on main that round-trips queries to the worker.

### 2a. `await L.perf()` — live RFX decode dashboard (main console)
Reports (per `perfSnapshot()` in `codecTestWorker.ts`):
```
rfx{ workers=8 inflight=0
     arrival[p50=.. p95=..]      ← server delivery cadence (ms between frames)
     present[p50=.. p95=..]      ← actual on-screen cadence (ms); ~22ms = 45fps
     barrier[p50=.. p95=..]      ← EndFrame→present wait (decode-pool latency)
     stages{ tiles=N us/tile: entropy=.. idwt=.. color=.. } }   ← THE stage split
   avc{ ... }  heapMB=-1 }
```
- `inflight` = pool backlog (in-flight RFX seqs awaiting all-N replies); climbing ⇒ pool can't keep up.
- `present ≫ arrival` ⇒ pool-bound. `arrival` jittery ⇒ network-bound (stop optimizing decode).
- **This is your primary before/after metric** for the CPU-SIMD lever: watch `stages.idwt` drop and `present`/`inflight` improve, and find how many videos before `inflight` climbs.

**How it's wired (to extend it):**
- Pool workers (`rfxDecodeWorker.ts`) accumulate per-stage time in the wasm via
  `EgfxDecoder::take_rfx_stage_timings()` (`egfx_decoder.rs`, gated on the
  `rfx-stage-timing` cargo feature, which the build enables) and post a `rfxStats`
  message every 60 decodes.
- `codecTestWorker.ts` aggregates them into the module-level `rfxStages` and
  `perfSnapshot()` formats the line; `setLogPerfProvider(perfSnapshot)` registers it.
- `logsink.ts` `reduceLocal('perf')` → the provider; `L.perf()` (both `makeLocalL`
  and `makeAsyncL`) calls it. The stage timing relies on ironrdp-graphics'
  `progressive::stage_timing` hook (feature `stage-timing`), reset/taken around the
  decode in `egfx_decoder::decode_wire_to_surface2`.

### 2b. `await gpuBench(numTiles?, passesPerComponent?, iters?)` — GPU plumbing bench
(`gpuPlumbingBench.ts`, triggered by a global in `DesktopSessionTestMulti.tsx` that
messages `codecTestWorker.ts` case `'gpuBench'`, which runs it on its **own
OffscreenCanvas** in the worker and replies + logs `[gpubench] …`.)
- Defaults: 1600 tiles, 12×3 passes, 20 iters.
- Reports `totalMsPerFrame`, `uploadMsPerFrame` vs `passesMsPerFrame`,
  `coeffMiBPerFrame`, `framesPer22ms` (how many 1600-tile frames fit a 22ms budget).
- Composite pass is the **real color GLSL** (matches `gpu_ref::color_pass`), so it
  also smoke-tests color on the GPU.
- **This is your primary metric for the GPU lever.** `framesPer22ms ≥ 2` ⇒ scales
  to multiple videos; `< 1` ⇒ can't sustain one; upload-dominant ⇒ attack upload.

### 2c. Other logsink commands (read-only, main console)
- `await L.summary()` — tag histogram of all logs (instantly shows AVC vs RFX traffic).
- `await L.distinct('[apply]')` / `await L.distinct('[gpubench]')` — dedup'd lines.
- `await L.tail(30)`, `await L.grep(/.../)`.
- `[apply]` line (formatted in `DesktopSessionTestMulti.tsx`) shows per-second
  `clear/c2s/s2c/rfx/other ms (busy/elapsed) | rfx{entropy idwt color blit}` — note
  the `rfx{...}` reads **0 in offload mode** (decode is in the pool, not main);
  it's non-zero only with `?offload=0` (inline decode on main).

### 2d. Build / iterate
- Release wasm (perf): `cd web/packages/shared/libs/player && ./build-release-wasm.sh release` → regenerates `pkg/session/`.
- Debug wasm (fast, **not for perf**): `./build-release-wasm.sh debug`.
- Native Rust tests (the diff-test gate): `cargo test -p session --lib <name>` (host; no GPU/browser).
- TS changes: reload the codec-test tab (Vite rebundles; no wasm rebuild).
- `?offload=0` on the page URL forces inline (single-thread) RFX decode — useful to
  read the per-stage split via the `[apply] rfx{...}` line without the pool.

---

## 3. The bake-off — run BOTH, then decide

Baseline first (release build, one full-screen video, ~15s): record `L.perf()`
(`stages.idwt`, `present.p50`, `inflight`) and `gpuBench()` (`framesPer22ms`).

### Lever A — CPU SIMD inverse-DWT (recommended; lower risk)
**Hypothesis:** SIMD the IDWT (~2.6–4× per literature/FreeRDP) drops per-tile
85.8µs → ~54µs ≈ **1.6× pool capacity** (~1.5–2 videos), entirely on-die (no upload),
no GPU exactness risk.

Steps (each gated by `max|Δ|==0` diff-test — see §4 scaffold):
1. **A2 (done-able now):** `gpu_ref::idwt_legacy` — transliterate `dwt::decode`
   (`third_party/.../dwt.rs:111`) using the **even/odd two-sub-pass** structure (the
   read-your-own-output hazard at `inverse_vertical` `dwt.rs:197-198`), diff vs
   `dwt::decode` (it's `pub`) on synthetic + edge-pattern coefficient buffers.
   Also `dwt_extrapolate::decode` (`dwt_extrapolate.rs:99`) for the reduce-extrapolate
   variant (`TileState::use_reduce_extrapolate`).
2. **Implement the real SIMD IDWT** in `dwt.rs`/`dwt_extrapolate.rs` (wasm `core::arch::wasm32` v128,
   vectorized **across independent rows/cols** — the pattern `simd_color` already uses
   at `progressive.rs:1062-1130`). Keep `i32_to_i16_possible_truncation` (`as i16` wrap)
   at every narrow site. simd128 is already on in `.cargo/config.toml`.
3. **Gate:** the A2 diff-test must stay `max|Δ|==0` for BOTH variants (a mismatch
   permanently poisons the difference/upgrade chain → black rectangles).
4. **Bench:** release build, one video → `L.perf()`: `stages.idwt` should fall
   sharply. Then **add videos** (multiple browser windows / popups) until `inflight`
   climbs / `present.p50` exceeds ~22ms. Report **max videos @ 45fps**.

### Lever B — GPU IDWT+color offload, upload-reduced
**Hypothesis:** the math is cheap (4.52ms); if we cut the 75MiB/frame upload we can
beat CPU SIMD on scaling. The de-risk says we must attack upload, in order:
1. **R16I instead of R32I** (coefficients are i16): in `gpuPlumbingBench.ts` change
   `gl.R32I/RED_INTEGER/INT` → `gl.R16I/RED_INTEGER/SHORT` and the data to `Int16Array`.
   Re-run `gpuBench()` — expect upload ~halved (~4.5ms), `framesPer22ms` ~2.4. **Do this first; it's the cheapest 2×.**
2. **Sub-region passes:** the bench runs every pass over the full 64×64; the real IDWT
   works on shrinking level regions (legacy: 16/32/64; extrapolate: 17/33/64). Scissor
   each pass to its level region — passes get cheaper than 4.52ms.
3. **Upload only changed tiles:** RFX already sends only changed tiles. Atlas only the
   dirty tiles per frame (windowed video ⇒ far fewer than 1600). Add a `numTiles`
   sweep to `gpuBench()` to model windowed vs full-screen.
4. **(stretch) Persistent GPU coefficient texture + delta upload:** keep prior
   coefficients in a GPU texture, upload only the per-frame delta. Largest upload win,
   most complexity (GPU becomes coefficient-stateful).
5. **Bench:** `gpuBench()` `framesPer22ms` after each step. To make it a *real* decode
   (not plumbing), do A3 (§4): real GLSL IDWT, `readPixels` vs CPU `DecodedTile`,
   `max|Δ|==0`, then time it.

### Decision criteria (head-to-head)
Compare **max simultaneous full-screen videos at 45fps**:
- If **CPU SIMD ≥ 2** and GPU (reduced) does **not clearly exceed** it → **ship CPU SIMD** (far simpler/safer; composes with raising the pool >8 and the barrier fix in §5).
- If **GPU (reduced) clearly scales beyond** CPU SIMD (e.g. ≥4 videos) and upload is tamed → invest in the GPU pipeline (Phase B).
- If `L.perf().arrival` is jittery (network-bound) at the target video count → neither decode lever helps; stop.

---

## 4. Phase-A scaffold (GPU correctness path) — what's already built/verified

Built so the GPU path (and the SIMD reference) is diff-test-gated against the CPU oracle.

- **A1 DONE + VERIFIED:** `gpu_ref::color_pass` (`crates/session/src/gpu_ref.rs`,
  `#[cfg(test)] mod gpu_ref` in `lib.rs`) — GLSL-`int`-style color transliteration,
  **byte-exact** vs ironrdp `ycbcr_to_rgb` over ~700k inputs. `cargo test -p session --lib color_pass_byte_exact` → `max|Δ|==0`.
- **Split point (verified):** `TileState::display: [[i16; 4096]; 3]` (`progressive.rs:793`)
  is the final IDWT input; `reconstruct_to_rgba(&self)` (`progressive.rs:956`) runs
  IDWT then color and is **`&self`** → stateless (compiler-enforced no write-back).
  Difference/upgrade accumulate in `coefficients` (CPU) and `display := coefficients`
  wholesale (`:885/:896/:940`) → **a GPU IDWT is stateless per-frame; a divergent
  frame can't poison future frames.** This de-risks GPU (even float is non-poisoning),
  but a strict diff-test still wants `max|Δ|==0`.
- **A2 NEXT:** IDWT reference (see Lever A step 1).
- **A3:** real WebGL2 single-tile pipeline (R16I textures, 6 IDWT sub-passes + color),
  `readPixels` vs CPU `DecodedTile` / `reconstruct_to_rgba` (pub). Phase-A gate `max|Δ|==0`.
- **A4:** extrapolate variant or CPU fallback (route `use_reduce_extrapolate` tiles to CPU).
- **Blueprint** (full GLSL pass plan, integer-exactness notes, even/odd split,
  `decode_block` offsets NOT `band_layout()`): see the design produced for this work;
  key file:line are in §3/§4 here.

**Vendored ironrdp edits so far (minimal):** `pub fn ycbcr_to_rgb` (`progressive.rs`,
1 line). `dwt::decode`/`dwt_extrapolate::decode`/`reconstruct_to_rgba` were already `pub`.

---

## 5. Orthogonal multi-video levers (help BOTH paths; measurement-independent)

- **Kill the all-N reply barrier:** a frame presents only when the *slowest* partition
  replies (`codecTestWorker.ts` `dispatchRfx`/`onReply`; head-of-line in `paint_queue.rs:189-195`),
  so latency = max-over-workers. Let disjoint partitions blit immediately, gate only
  the EndFrame `Present`. **Needs a read-dependency check** so the window-drag
  SurfaceToSurface ordering fix (`paint_queue.rs` module docs) doesn't regress.
- **Work-stealing instead of static hash partition** (`TilePartition`, salted by
  surface id): a hot video region piles tiles on a few workers while others idle. Big for multi-video.
- **Drop/coalesce under backlog:** when behind, drop superseded progressive *Upgrade*
  passes (monotonic) so present-rate tracks capacity instead of collapsing + ballooning memory.
- **Raise pool >8** (`clamp(hc-2,1,8)` in `codecTestWorker.ts`) on many-core hosts —
  marginal unless the duplicated per-worker PDU parse is also fixed.
- **REFUTED — do not chase:** "every pool worker decodes the full PDU then discards."
  Non-owned tiles are parse-only then `continue`d before the wavelet math
  (`progressive.rs:1594-1601`); IDWT runs once per tile across the pool. The genuine
  redundant cost is the per-worker stream *parse* (cheap) and the return-path RGBA copies.

---

## 6. File / symbol map

| File | What |
|---|---|
| `gpu_ref.rs` (new, session crate, `#[cfg(test)]`) | GLSL-transliteration references + diff-tests. A1 color done; add A2 IDWT here. |
| `gpuPlumbingBench.ts` (new) | GPU plumbing microbench. R32I now → switch to R16I for Lever B. |
| `codecTestWorker.ts` | `perfSnapshot()` (L.perf), `rfxStats` aggregation, `'gpuBench'` handler, pool dispatch (`dispatchRfx`/`onReply`). |
| `rfxDecodeWorker.ts` | pool worker; `takeRfxStageTimings()` flush every 60 decodes. |
| `crates/session/src/egfx_decoder.rs` | offload decode (`decode_wire_to_surface2`) + stage-timing accum + `take_rfx_stage_timings`. |
| `crates/session/src/framebuffer.rs` | inline decode (`apply_rfx_progressive`, `?offload=0`) + blit + `[apply]` timers. |
| `crates/session/src/perf.rs` | `record_rfx_stages`, the perf message. |
| `crates/session/src/paint_queue.rs` | wire-order apply queue + the all-N barrier head-of-line. |
| `third_party/ironrdp/crates/ironrdp-graphics/src/dwt.rs` | legacy inverse-DWT (`decode:111`). SIMD target. |
| `third_party/ironrdp/crates/ironrdp-graphics/src/dwt_extrapolate.rs` | extrapolate inverse-DWT (`decode:99`). |
| `third_party/.../progressive.rs` | `TileState::display:793`, `reconstruct_to_rgba:956`, `ycbcr_to_rgb:1027` (now pub), `simd_color:1062`. |
| `DesktopSessionTestMulti.tsx` | main; worker wiring; `gpuBench()` global; `[apply]` formatter. |
| `build-release-wasm.sh` | wasm build (RELEASE for perf; wasm-opt now non-fatal). |

## 7. Gotchas
- **Perf-test only the RELEASE wasm.** The script defaults to debug; debug is opt-level=0 and ~5–10× slower.
- **`max|Δ|==0` is non-negotiable for any IDWT change** — the difference/upgrade chain poisons permanently on mismatch.
- **`L.perf().stages` needs the `rfx-stage-timing` feature** (the build enables it) and reads the *offload* path; the `[apply] rfx{}` line is the *inline* path (0 under offload).
- **GLSL signed `>>` is implementation-defined on negatives** (ES 3.00) — the A3 GPU diff-test (not the native A1/A2) is what catches this; pick negative-coefficient test tiles.
- **`gpuBench()` blocks the worker briefly** (it's a synchronous bench loop) — expected; run it deliberately, not during a live session you care about.

---

## 8. Working-tree state (as of this handoff)

Branch `ryan/rdpclient/codec`. **HEAD = `7fec7029b26`** ("checkpoint codec + monitor
work before native-parity refactor"). Everything below is **uncommitted on top of it.**

**Uncommitted, tracked (Track A perf + instrumentation — keep, consider committing):**
- `build-release-wasm.sh` — wasm-opt step now non-fatal (falls back to the fat-LTO wasm).
- `codecTestWorker.ts` — `perfSnapshot()` RFX section + `rfxStats` aggregation + `'gpuBench'` handler/types.
- `rfxDecodeWorker.ts` — per-worker `takeRfxStageTimings()` flush.
- `crates/session/src/egfx_decoder.rs` — offload-path stage timing (`rfx-stage-timing`).
- `crates/session/src/lib.rs` — `#[cfg(test)] mod gpu_ref;`.
- `logsink.ts` — `L.perf()` op + `setLogPerfProvider`.
- `DesktopSessionTestMulti.tsx` — `gpuBench()` global.

**Untracked, new (keep):**
- `gpu_ref.rs` — A1 color reference + diff-test (verified). Add A2 IDWT here.
- `gpuPlumbingBench.ts` — the GPU microbench.
- `RFX_IDWT_CEILING_HANDOFF.md` — this doc.

**Vendored edit (in `third_party/`, which is UNTRACKED in this repo):**
- `third_party/ironrdp/crates/ironrdp-graphics/src/progressive.rs` — `pub fn ycbcr_to_rgb`
  (1 line; was already a test-only reference). Because `third_party/` is untracked, this
  edit is **not** captured by `git status` of the main tree — don't lose it.

**Stashed (Track B, reverted):**
- `stash@{0}` = "increment-1 minimal AVC present-deferral (reverted: shared-texture
  overlap artifacts at AVC/RFX boundary)" — touches `gl.rs`, `framebuffer.rs`, `lib.rs`
  (session crate). See §9.1. `git stash show -p stash@{0}` to inspect; don't `pop` unless
  resuming Track B.

> Recommendation: commit the Track-A perf + instrumentation set (it's verified/useful)
> before starting either bake-off lever, so a failed experiment doesn't lose it. Keep
> the throwaway diagnostics (§12) clearly marked so they're easy to strip pre-merge.

---

## 9. Track B — native-parity rendering (PAUSED; gated on AVC, §10)

**Why it exists.** The original report was "mixed RFX and AVC draw at different times,"
plus boundary artifacts. Root cause (traced): AVC and RFX render on **two independent
present clocks** into **one shared GPU texture** — AVC uploads its `VideoFrame` straight
to the texture and presents on the WebCodecs cadence; RFX/chrome composites into the CPU
`DecodedImage` and presents at the EGFX `END_FRAME`. Native clients (mstsc/FreeRDP) avoid
both problems: **one present per `END_FRAME`**, all codecs composited into **per-surface
offscreen textures** mapped to output, AVC444 chroma recombined, and `FRAME_ACKNOWLEDGE`
flow control. The four increments below were the plan to reach that.

**Hard dependency:** all of this is only observable/testable when **AVC actually flows**.
On this host it doesn't (§10). So Track B is parked until AVC is enabled. (Exception:
Increment 2's surface model has standalone value for multi-monitor/scaling even RFX-only.)

### 9.1 Increment 1 — unify the present clock (AVC → END_FRAME). **REVERTED (stash@{0}).**
- *Attempt:* in offload mode, make AVC upload to the texture *without drawing*
  (`gl::upload_video_frame`, `framebuffer::blit_video_frame_no_draw` + a `force_present`
  flag) and let the existing `END_FRAME` `Present` marker do the single draw — same clock
  as RFX. `blitAvcFrame`/`blitAvcRgba` made offload-aware (defer present).
- *Why reverted:* on overlapping content (YouTube UI over the video), a single shared
  texture can't composite AVC (GPU-only) + chrome (CPU-only) — each clobbers the other,
  and removing AVC's per-frame self-redraw exposed it as boundary artifacts. **Not
  patchable in the shared-texture model.** The correct fix is Increment 2 (separate
  surfaces). Code is in `stash@{0}` if useful as reference.

### 9.2 Increment 2 — offscreen surface-texture model + MapSurfaceToOutput. **(the real fix)**
- Per-EGFX-surface GPU textures; all codecs (AVC via `texSubImage2D(VideoFrame)`, RFX via
  CPU→texture, ClearCodec, etc.) composite into their surface texture; a final pass maps/
  scales surfaces → output canvas(es) (`RDPGFX_MAP_SURFACE_TO_OUTPUT` / `…SCALED_OUTPUT`).
- Delivers the unified present clock **and** correct AVC/RFX overlap (separate textures,
  no clobber), and cleans up multi-monitor + DPR scaling. This is the native model.
- Touches: `framebuffer.rs` (surface model, replacing the single `DecodedImage`→canvas),
  `gl.rs` (per-surface FBOs + composite pass), the EGFX surface-create/map handlers in
  `lib.rs`/`egfx.rs`. Largest of the four.

### 9.3 Increment 3 — AVC444 YUV420→YUV444 chroma recombination.
- Today `feedChroma` (`codecTestWorker.ts`) is a **probe stub**: only luma is composited,
  so AVC444 renders at 4:2:0 (soft text). Implement the real recombination — `LumaToYUV444`
  + `ChromaV1ToYUV444`/`ChromaV2ToYUV444` geometry ([MS-RDPEGFX] 3.3.8.3.2/3.3.8.3.3,
  FreeRDP `general_YUV420CombineToYUV444`) — ideally in a WebGL2 shader, including the
  **optional reverse anti-alias filter** (`u0' = 4·u_filtered − u1 − u2 − u3`, cutoff
  threshold 30) to match mstsc quality (FreeRDP issue #11040).
- The two AVC444 substreams must be decoded by **one** H.264 decoder "as one stream"
  ([MS-RDPEGFX] MUST) — open question whether one WebCodecs `VideoDecoder` can interleave
  them preserving references (esp. LC=0x1/0x2 luma-only/chroma-only frames). Verify before building.

### 9.4 Increment 4 — FRAME_ACKNOWLEDGE pacing (post-present queueDepth).
- Ack each frame **after composite+present** with a real `queueDepth` (sentinels
  `QUEUE_DEPTH_UNAVAILABLE=0x0`, `SUSPEND_FRAME_ACKNOWLEDGEMENT=0xFFFFFFFF`,
  [MS-RDPEGFX] 2.2.2.13) so the server paces to the true render rate (back-pressure).
  Server side already emits `FrameAcknowledge(queue_depth=frames_queued)` from IronRDP;
  the client present timing should feed a true post-present depth. Closes the flow-control loop.

---

## 10. AVC enablement (prerequisite for Track B; the "best video" path when it works)

Video is RFX **only because** the single 4112-px-wide surface exceeds the host H.264
encoder's ~4096/stream cap → host sends `AVC_DISABLED`. To get native AVC for video at
native resolution (no clamp), one of:

1. **Multi-monitor, each ≤4096** (recommended; hardware AVC per monitor, native res, no
   clamp). Windows allocates one AVC/H.264 encoder per monitor; total width can exceed 4096.
   The harness already has multi-monitor support — present the desktop as ≥2 monitors each ≤4096.
2. **Software AVC** via the host GPO/registry
   `HKLM\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services\AVC444ModePreferred=1`
   (forces software AVC, not bound by the GPU 4096 cap; host CPU cost; **forces 444**).
3. **HEVC** (NVENC HEVC = 8192-wide; needs HEVC-capable host encoder + WebCodecs HEVC decode — verify browser support).

(Clamping the desktop to ≤4096 also works but the user explicitly wants native resolution.)

**Verify it took:** `docker logs rdpclient-windows-1 2>&1 | grep confirmed | tail` →
want `V10_7 { ...0x0... }` (no `AVC_DISABLED`); then `await L.perf()` shows `avcDecoders ≥ 1`
and `await L.distinct('[avc-probe]')` shows codecId 11 (AVC420) or 14/15 (AVC444).
**Note:** enabling AVC re-introduces the two-clocks/boundary problem → that's exactly when
Track B (Increment 2) becomes the work to do.

---

## 11. Open questions / unverified assumptions

- **Disjoint-region assumption** (`lib.rs` AVC WIRE-ORDER NOTE): AVC and RFX/inline are
  assumed to target disjoint regions; unverified against a real capture. If a host overlaps
  them in one frame, immediate-AVC vs deferred-END_FRAME present can composite out of wire
  order. (Track B / Increment 2 removes the assumption.)
- **One WebCodecs decoder for both AVC444 substreams** preserving inter-frame references — unverified (§9.3).
- **GLSL signed `>>` on negatives is implementation-defined** (ES 3.00) — only the A3 GPU
  diff-test catches divergence; pick negative-coefficient test tiles.
- **WebCodecs decode of >4096-wide H.264** likely falls to software decode (HW decoders
  also cap ~4096) — relevant if you enable software/HEVC AVC at native 4112.
- **`glFramebufferPainter.ts` appears to be dead code** (no importers) — confirm and remove.
- **`rfx_video_probe.rs` is inert** (accept-and-discard) — exists to detect if Windows ever
  routes pixels over the legacy RDS Video DVCs; currently does nothing.
- **`DesktopSessionTest.tsx` (single-monitor) uses a different worker (`./worker`)** than
  the codec-test's `codecTestWorker.ts` — its AVC/queue behavior is unconfirmed.

---

## 12. Diagnostics & cleanup TODO (strip before merge)

Throwaway instrumentation added during this work — keep while benchmarking, remove for a real PR:
- `[perf-probe]` counters + `perfSnapshot()` / `L.perf()` (decide if any stays as a perma-diagnostic).
- `gpuPlumbingBench.ts` + `gpuBench()` global + the `'gpuBench'` worker message path.
- `rfx-stage-timing` cargo feature usage (it's `--features rfx-stage-timing` in `build-release-wasm.sh`; throwaway per the feature's own doc comment).
- `[avc-probe]` / `[avc444]` / `[avc-green]` / `[pacing]` console logs in `codecTestWorker.ts`.
- `paint_magenta_border` (`framebuffer.rs`) and `Traffic::summary` (`lib.rs`) — pre-existing dead code, warned by the compiler.
- The stale `egfx.rs:109-113` header comment ("CURRENT STATE (RFX-only): AVC is DISABLED")
  contradicts the live region-split caps below it — fix or delete.
- `gpu_ref.rs` is `#[cfg(test)]` (test-only); if Phase B ships, the GLSL becomes production
  and `gpu_ref` stays as the test oracle.

