# RFX-Progressive inverse-DWT — CPU SIMD (Lever A) — DESIGN

**Date:** 2026-06-17
**Branch:** `ryan/rdpclient/codec`
**Status:** design approved; ready for implementation plan.
**Companion:** `RFX_IDWT_CEILING_HANDOFF.md` (the bake-off this lever belongs to).

> Spec location note: the brainstorming skill defaults to
> `docs/superpowers/specs/`, but this is co-located with the existing
> `RFX_IDWT_CEILING_HANDOFF.md` for discoverability — all this perf work lives
> under `web/packages/shared/libs/player/`.

---

## 1. Goal & context

Make RFX-Progressive video decode scale to multiple full-screen videos by
SIMD-vectorizing the **inverse-DWT** stage — the largest per-tile cost
(48.1 µs/tile ≈ **56%** of decode per the handoff stage-split). This is **Lever A**
of the bake-off.

**Measured head-to-head baseline to beat (Lever B, GPU offload, R16I, this MacBook
Pro 16″, DPR 2):**

| workload | tiles | GPU R16I total | frames/22 ms |
|---|---|---|---|
| windowed 720p | 240 | ~2.0 ms | ~11 |
| windowed 1080p | 510 | ~2.6 ms | ~8.5 |
| full-screen (More Space) | 2730 | ~11.5 ms | ~1.9 |

GPU plumbing throughput ≈ **236k tiles/s**, but realizing it needs the per-surface
GPU-composite architecture (Track B / Increment 2). CPU SIMD drops into the existing
decode pool with **zero structural change** — this lever measures whether it's
competitive (~130–160k tiles/s estimated; the real number is the deliverable).

## 2. Non-goals

- No GPU work (that's Lever B / Track B).
- No change to the entropy (RLGR + dequant) or color stages — color is already
  SIMD (`simd_color`).
- No refactor of the working `simd_color` path (note a future `I32x4` dedup, don't do it).
- No change to the difference/upgrade accumulation (stays CPU coefficient-domain).

## 3. Architecture

Vectorize the IDWT following the **exact pattern `simd_color` already established**
(`progressive.rs:1063`): a kernel written once against a tiny `I32x4` whose backend
is chosen by `cfg` — real `core::arch::wasm32` v128 on `wasm32+simd128`, plain
`[i32; 4]` everywhere else (native builds + the oracle + unit tests run the identical
lane logic).

- Keep the existing scalar `dwt::decode` / `dwt_extrapolate::decode` as the **bit-exact
  oracle** (do not delete; the diff-test compares against them).
- Add a SIMD `decode_simd` per variant. **Dispatch is per-variant with scalar
  fallback** — `reconstruct_to_rgba` already branches on `use_reduce_extrapolate`
  (`progressive.rs:969`), so a variant we haven't vectorized simply uses scalar.
  **Correctness never depends on which variant a region uses; detection only orders
  the work.**

The IDWT lifting has a data dependency *along* each row/column (e.g. `inverse_vertical`
reads its own just-written output, `dwt.rs:197-198`). So we vectorize **across
independent rows/columns** — 4 independent rows (or columns) in the 4 i32 lanes,
identical control flow per lane — never along the dependency axis. Strided access
(columns, scattered subbands) needs explicit lane packing.

All edits live in the **vendored, untracked** `third_party/ironrdp/crates/ironrdp-graphics/`
(plus a thin toggle in the session crate). Vendored edits are not captured by the main
tree's `git status` — they must be preserved deliberately (same caveat the handoff
flags for the `ycbcr_to_rgb` edit).

## 4. Components

| Component | File | What |
|---|---|---|
| `I32x4` lanes | `simd_lane.rs` (new, ironrdp-graphics) | v128 / `[i32;4]` cfg backends; `splat/wrapping_{add,sub,mul}/shl/sra/smin/smax/to_array` + strided i16 loads. Mirrors `simd_color`'s inline copy; `simd_color` left untouched. |
| `dwt::decode_simd` | `dwt.rs` (new fn) | SIMD legacy inverse-DWT, lanes across rows/cols. |
| `dwt_extrapolate::decode_simd` | `dwt_extrapolate.rs` (new fn) | SIMD reduce-extrapolate inverse-DWT. |
| dispatch | `progressive.rs:969` `reconstruct_to_rgba` | call `decode_simd` vs scalar per the toggle. |
| toggle | `progressive.rs` `set_simd_idwt(bool)` + `AtomicBool` | mirrors the `stage_timing::set_now_hook` side-channel; default **on**. |
| toggle wiring | session crate + `DesktopSessionTestMulti.tsx` | `?simd-idwt=0/1` URL param → `set_simd_idwt`, for same-session A/B. |
| variant probe (temporary) | `stage_timing` side-channel + session-crate `web_sys` log | detect the hot variant once; removed after detection. |

## 5. Data flow (per 64×64 tile)

`TileState::display[Y|Cb|Cr]` (i16, the final IDWT input)
→ copy to scratch buffers
→ `decode_simd`: for each pass, pack 4 independent rows/columns into 4 i32 lanes,
   run the lifting per lane, narrow back to i16 — bit-identical to scalar
→ feed the existing `simd_color::ycbcr_to_rgba_tile`
→ interleaved RGBA8.

Stateless per frame (`reconstruct_to_rgba(&self)`; accumulation is in the coefficient
domain, unchanged), so a divergent frame cannot poison future frames.

## 6. Exactness requirements (the hard part — must hold for `max|Δ|==0`)

1. **`as i16` truncation is a WRAP**, reproduced lane-wise as `(v << 16) >> 16`
   (the `gpu_ref::wrap_i16` / `simd_color` idiom), applied at **every** narrow site.
2. **Per-variant rounding differs:**
   - legacy `dwt.rs` uses arithmetic `>> 1` (floor, toward −∞) → `i32x4_shr` (free).
   - extrapolate `dwt_extrapolate.rs` uses `/ 2` (truncate toward zero) → lane-wise
     `(a + ((a >> 31) & 1)) >> 1`.
   Each `decode_simd` must match **its** variant's semantics.
3. **`<< 1` happens on `i16` before widening** in legacy (`dwt.rs:153,161,198,205`):
   reproduce the i16-domain wrap before extending to i32.
4. **Sub-pass ordering preserved:** even-then-odd within a row/column
   (`inverse_horizontal`, `inverse_vertical`; extrapolate `idwt_row`/`idwt_col`). Lanes
   run across independent rows/cols so the read-your-own-output dependency stays
   intra-lane and ordering is identical to scalar.
5. **Subband offsets** come from the existing `decode_block` offsets — reuse the scalar
   code's indexing, do not re-derive from `band_layout()`.

## 7. Testing

Native `cargo test -p session --lib` (host = scalar backend), asserting
**`max|Δ| == 0`** of `decode_simd` vs scalar `decode`, over coefficient buffers built
from **real data construction** (no mocks):

- all-zeros, DC (flat), horizontal & vertical gradients
- LCG pseudo-random (the `dwt_extrapolate` test idiom)
- dense edge/boundary patterns (the `gpu_ref` color-test idiom)
- **negative-heavy** and **near-i16-overflow** inputs (where wrap/rounding bugs hide)

Both variants' diff-tests are written even if only one ships first.

**Known gap (same as `simd_color`, `progressive.rs:990`):** no wasm-side test — the
v128 path rests on the 1:1 lane correspondence with the host-tested scalar backend plus
in-browser render validation. Stretch: a `wasm-bindgen-test` for full v128 confidence.

**Adversarial verification (implementation phase):** use a Workflow to fan out agents
that (a) independently enumerate exactness hazards in both scalar `decode` fns and
(b) verify `decode_simd` against the scalar oracle on independently-generated edge
tiles, before claiming the gate is green.

## 8. Measurement

Release wasm (`./build-release-wasm.sh release`), `?simd-idwt` toggle for same-session
A/B:

- `L.perf().stages.idwt` — should fall sharply (SIMD-on vs off).
- `present.p50` / `inflight` — add full-screen videos until `inflight` climbs.
- Report **max full-screen videos @ 45fps** and the resulting **tiles/s**, to complete
  the head-to-head vs Lever B's measured ~236k tiles/s.

## 9. Acceptance criteria

- [ ] **Gate 1:** diff-tests `max|Δ| == 0` for the implemented variant(s) across all
      §7 input classes.
- [ ] **Gate 2:** release wasm renders the video with **no black rectangles / artifacts**
      (visual check).
- [ ] **Result:** measured `stages.idwt` drop (SIMD-on vs off) and max full-screen
      videos @ 45fps, recorded for the bake-off decision.

## 10. Implementation order

1. **Detect the hot variant** — extend the `stage_timing` side-channel + a one-time
   `web_sys` log in the session crate; debug build; reload; read console; remove probe.
   Tells us which `decode_simd` to build first.
2. `simd_lane.rs` `I32x4` + the diff-test scaffold (test fails: no impl yet).
3. Implement the hot variant's `decode_simd` → Gate 1 green.
4. Toggle plumbing + release build + `L.perf` A/B (Gate 2 + Result).
5. If the other variant later proves hot, implement it too (repeat 2–4).

## 11. Risks & open items

- **v128 pack/transpose overhead** may undercut the 2.6–4× literature speedup for the
  lifting — the measurement is the whole point; if it underperforms, fall back to the
  autovectorization experiment (Approach 3) or stop.
- **4-wide i32 only** (v128); wider lanes would need 2× v128 interleaving.
- **Vendored-crate edits are untracked** — preserve deliberately; list them in any
  handoff update.
- **Variant detection** could reveal both variants occur — then both need SIMD for full
  coverage (each is independently gated).
