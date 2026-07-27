# RFX-Progressive SIMD inverse-DWT (Lever A) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Vectorize the RFX-Progressive inverse-DWT (the 56% / 48.1 µs-per-tile decode stage) with wasm `simd128`, bit-exact with the scalar decode, and measure how many full-screen videos it sustains at 45 fps.

**Architecture:** Follow the existing `simd_color` pattern — a kernel written once against a tiny `I32x4` whose backend is cfg-selected (real `core::arch::wasm32` v128 on `wasm32+simd128`, scalar `[i32;4]` elsewhere). Lanes run across **independent rows/columns** (the lifting has an intra-row/col dependency). The scalar `decode` stays as the bit-exact oracle; a native `cargo test` gates `max|Δ|==0`. Per-variant with scalar fallback, so correctness never depends on which variant a region uses.

**Tech Stack:** Rust (ironrdp-graphics, vendored & untracked), `core::arch::wasm32` v128 intrinsics, `wasm-pack`/`build-release-wasm.sh`, the codec-test `L.perf()` dashboard.

**User decisions (already made):**
- "build A" — implement Lever A (CPU SIMD IDWT), not the GPU lever.
- "lever B first, measure, then try A" — Lever B is measured (R16I: ~236k tiles/s, full-screen ~1.9 frames/22ms); this plan is the A side of the head-to-head.
- "Detect from live stream first" — detect the hot `use_reduce_extrapolate` variant before building (Task 0).
- Approach 1 approved (mirror `simd_color`; reject nightly `std::simd` and autovectorization).
- Spec approved at `RFX_SIMD_IDWT_DESIGN.md`; left uncommitted by user choice.

> Location note: co-located with `RFX_SIMD_IDWT_DESIGN.md` and `RFX_IDWT_CEILING_HANDOFF.md` under the player dir, rather than the skill default `docs/superpowers/plans/`. The `.tasks.json` sits beside this file.

## Build & test commands (IMPORTANT — two workspaces)

Discovered during Task 1: the vendored `ironrdp-graphics` is **not** a member of the session/Teleport workspace, so its tests are **not** reachable via `-p session`.

- **ironrdp-graphics tests** (`simd_lane`, `dwt`, `dwt_extrapolate`, `progressive` incl. `reconstruct_toggle_identical`): `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib <name>`
- **session-crate tests** (e.g. `gpu_ref::color_pass`): `cargo test -p session --lib <name>` from `web/packages/shared/libs/player`
- **wasm compile-check of the vendored crate** (verifies the `wasm32+simd128` v128 backend, which host tests do NOT compile): `cargo check -p session --target wasm32-unknown-unknown` from the player dir
- **wasm build:** `cd web/packages/shared/libs/player && ./build-release-wasm.sh debug|release`

Wherever a task below says `cargo test -p session --lib <ironrdp-graphics test>`, read it as the `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib <name>` form.

---

### Task 0: Detect the hot IDWT variant

**USER-ORDERED GATE — NON-SKIPPABLE.** This task was requested by the user in the current conversation ("Detect from live stream first"). It MUST NOT be closed by walking around it, by declaring it "verified inline", or by substituting a cheaper check. Close only after every item in `acceptanceCriteria` has been re-validated independently, with output captured.

**Goal:** Determine whether the live video stream decodes via `dwt_extrapolate::decode` (reduce-extrapolate) or `dwt::decode` (legacy), so we vectorize the path that actually runs.

**Files:**
- Modify: `third_party/ironrdp/crates/ironrdp-graphics/src/progressive.rs` (temporary probe; reverted at end of task)
- Modify: `web/packages/shared/libs/player/crates/session/src/egfx_decoder.rs` (read + log the flag once)

**Acceptance Criteria:**
- [ ] During video playback the DevTools console prints `[idwt-variant] extrapolate` or `[idwt-variant] legacy` at least once.
- [ ] The probe is removed from both files before the task is committed (the commit contains no `[idwt-variant]` code).
- [ ] The detected variant is recorded in this plan's Task 2 (which `decode_simd` to build first).

**Verify:** `./build-release-wasm.sh debug` → reload codec-test tab with a video playing → `await L.distinct('[idwt-variant]')` shows exactly one variant tag.

**Steps:**

- [ ] **Step 1: Add a one-time recorder in ironrdp (no console there — it's `no_std`).** In `progressive.rs`, near the other module statics, add:

```rust
/// TEMPORARY probe (Task 0): records whether the last decoded region used the
/// reduce-extrapolate IDWT, for the session crate to log once. REMOVE after detection.
pub static LAST_IDWT_EXTRAPOLATE: core::sync::atomic::AtomicBool =
    core::sync::atomic::AtomicBool::new(false);
```

In `reconstruct_to_rgba` (`progressive.rs:969`), at the top of the IDWT block, record the flag:

```rust
LAST_IDWT_EXTRAPOLATE.store(self.use_reduce_extrapolate, core::sync::atomic::Ordering::Relaxed);
```

- [ ] **Step 2: Log it once from the session crate** (where `web_sys` is available). In `egfx_decoder.rs`, after `decode_wire_to_surface2` completes a decode, add a one-shot log:

```rust
{
    use core::sync::atomic::Ordering;
    static LOGGED: core::sync::atomic::AtomicBool = core::sync::atomic::AtomicBool::new(false);
    if !LOGGED.swap(true, Ordering::Relaxed) {
        let v = ironrdp_graphics::progressive::LAST_IDWT_EXTRAPOLATE.load(Ordering::Relaxed);
        web_sys::console::warn_1(
            &format!("[idwt-variant] {}", if v { "extrapolate" } else { "legacy" }).into(),
        );
    }
}
```

- [ ] **Step 3: Build debug wasm and detect.**

Run: `cd web/packages/shared/libs/player && ./build-release-wasm.sh debug`
Then reload the codec-test tab with a video playing and run in console: `await L.distinct('[idwt-variant]')`
Expected: one line, `[idwt-variant] extrapolate` (most likely) or `[idwt-variant] legacy`.

- [ ] **Step 4: Record the result and revert the probe.** Write the detected variant into Task 2 below. Remove the Step 1 + Step 2 code from both files.

- [ ] **Step 5: Commit** (the revert — the working tree should have no probe code):

```bash
git -C third_party/ironrdp diff --stat   # confirm progressive.rs probe removed
git add web/packages/shared/libs/player/RFX_SIMD_IDWT_PLAN.md
git commit -m "chore(rdpclient): record detected IDWT variant for SIMD lever"
```

> Note: `third_party/` is untracked in the main repo, so its revert isn't captured by `git status` of the main tree — verify by reading the file.

---

### Task 1: `I32x4` lane abstraction (`simd_lane.rs`)

**Goal:** A portable 4-lane i32 type with v128 and scalar backends, exposing exactly the ops the IDWT needs, with host unit tests proving the lane semantics.

**Files:**
- Create: `third_party/ironrdp/crates/ironrdp-graphics/src/simd_lane.rs`
- Modify: `third_party/ironrdp/crates/ironrdp-graphics/src/lib.rs` (add `mod simd_lane;`)

**Acceptance Criteria:**
- [ ] `I32x4` compiles on both `wasm32+simd128` and the host.
- [ ] Host unit tests pass for: `wrap_i16`, arithmetic `sra`, logical `shl`, `div2_trunc` (incl. negatives), `from_lanes`/`to_array` round-trip.
- [ ] `div2_trunc` matches Rust `/2` for negatives (e.g. −3→−1, −1→0, −5→−2); `sra(1)` matches `>>1` floor (−3→−2).

**Verify:** `cargo test -p session --lib simd_lane` → all pass. (Run from `web/packages/shared/libs/player`.)

**Steps:**

- [ ] **Step 1: Write the failing tests.** Create `simd_lane.rs` with a `#[cfg(test)] mod tests` first:

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn div2_trunc_matches_rust_division() {
        for a in [-5i32, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, i32::MIN + 2, i32::MAX - 1] {
            let lanes = I32x4::from_lanes([a, a, a, a]).div2_trunc().to_array();
            assert_eq!(lanes[0], a / 2, "div2_trunc({a})");
        }
    }

    #[test]
    fn sra_is_arithmetic_floor() {
        for a in [-5i32, -4, -3, -1, 1, 7] {
            let got = I32x4::from_lanes([a, a, a, a]).sra(1).to_array()[0];
            assert_eq!(got, a >> 1, "sra({a})");
        }
    }

    #[test]
    fn wrap_i16_wraps_like_as_i16() {
        for a in [0i32, 32767, 32768, 40000, -32768, -32769, 70000, -70000] {
            let got = I32x4::from_lanes([a, a, a, a]).wrap_i16().to_array()[0];
            assert_eq!(got, a as i16 as i32, "wrap_i16({a})");
        }
    }

    #[test]
    fn shl_is_logical() {
        for a in [1i32, -1, 0x4000, -0x4000] {
            let got = I32x4::from_lanes([a, a, a, a]).shl(1).to_array()[0];
            assert_eq!(got, ((a as u32) << 1) as i32, "shl({a})");
        }
    }
}
```

- [ ] **Step 2: Run to confirm red.**

Run: `cargo test -p session --lib simd_lane`
Expected: FAIL to compile — `I32x4` not defined.

- [ ] **Step 3: Implement `I32x4`.** Write the two cfg backends (mirror `simd_color`'s, plus the IDWT-specific ops). Wasm backend:

```rust
//! Portable 4-lane i32 SIMD for the inverse-DWT, mirroring `simd_color`'s
//! cfg-backend pattern. The kernel is written once against `I32x4`; the host
//! (scalar backend) is the diff-test oracle, each v128 op a 1:1 of its lane op.

#[cfg(all(target_arch = "wasm32", target_feature = "simd128"))]
mod backend {
    use core::arch::wasm32::{
        i32x4, i32x4_add, i32x4_extract_lane, i32x4_shl, i32x4_shr, i32x4_splat, i32x4_sub, v128,
    };

    #[derive(Clone, Copy)]
    pub(super) struct I32x4(v128);

    impl I32x4 {
        #[inline]
        pub(super) fn splat(v: i32) -> Self { Self(i32x4_splat(v)) }
        #[inline]
        pub(super) fn from_lanes(a: [i32; 4]) -> Self { Self(i32x4(a[0], a[1], a[2], a[3])) }
        #[inline]
        pub(super) fn add(self, o: Self) -> Self { Self(i32x4_add(self.0, o.0)) }
        #[inline]
        pub(super) fn sub(self, o: Self) -> Self { Self(i32x4_sub(self.0, o.0)) }
        #[inline]
        pub(super) fn shl(self, n: u32) -> Self { Self(i32x4_shl(self.0, n)) }     // logical
        #[inline]
        pub(super) fn sra(self, n: u32) -> Self { Self(i32x4_shr(self.0, n)) }     // arithmetic
        #[inline]
        pub(super) fn to_array(self) -> [i32; 4] {
            [
                i32x4_extract_lane::<0>(self.0),
                i32x4_extract_lane::<1>(self.0),
                i32x4_extract_lane::<2>(self.0),
                i32x4_extract_lane::<3>(self.0),
            ]
        }
    }
}

#[cfg(not(all(target_arch = "wasm32", target_feature = "simd128")))]
mod backend {
    #[derive(Clone, Copy)]
    pub(super) struct I32x4([i32; 4]);

    impl I32x4 {
        #[inline]
        pub(super) fn splat(v: i32) -> Self { Self([v; 4]) }
        #[inline]
        pub(super) fn from_lanes(a: [i32; 4]) -> Self { Self(a) }
        #[inline]
        pub(super) fn add(self, o: Self) -> Self { Self(core::array::from_fn(|i| self.0[i].wrapping_add(o.0[i]))) }
        #[inline]
        pub(super) fn sub(self, o: Self) -> Self { Self(core::array::from_fn(|i| self.0[i].wrapping_sub(o.0[i]))) }
        #[inline]
        pub(super) fn shl(self, n: u32) -> Self { Self(core::array::from_fn(|i| ((self.0[i] as u32) << n) as i32)) }
        #[inline]
        pub(super) fn sra(self, n: u32) -> Self { Self(core::array::from_fn(|i| self.0[i] >> n)) }
        #[inline]
        pub(super) fn to_array(self) -> [i32; 4] { self.0 }
    }
}

pub(crate) use backend::I32x4;

impl I32x4 {
    /// `as i16` truncation = wrap to low 16 bits, sign-extended (the `gpu_ref`/`simd_color` idiom).
    #[inline]
    pub(crate) fn wrap_i16(self) -> Self { self.shl(16).sra(16) }

    /// Integer `/2` with truncation toward zero (matches Rust `/2` for negatives),
    /// for the reduce-extrapolate variant. `(a + (sign_bit)) >> 1`.
    #[inline]
    pub(crate) fn div2_trunc(self) -> Self {
        // sign_bit lane = 1 if negative, else 0  ==  (a >> 31) & 1, built without an `&` op:
        let sign = self.sra(31);                 // -1 if negative, else 0
        self.sub(sign).sra(1)                    // a - (-1) = a + 1 when negative; floor-shift
    }
}
```

> Note the `div2_trunc` identity: for `a < 0`, `sra(31) = -1`, so `a - (-1) = a + 1`, then `>>1` gives trunc-toward-zero; for `a >= 0`, `sra(31) = 0`, so `a >> 1` = floor = trunc. Verified by the Step-1 test.

- [ ] **Step 4: Add `mod simd_lane;`** to `lib.rs` (next to the other `mod` declarations).

- [ ] **Step 5: Run to confirm green.**

Run: `cargo test -p session --lib simd_lane`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit.**

```bash
git -C third_party/ironrdp add crates/ironrdp-graphics/src/simd_lane.rs crates/ironrdp-graphics/src/lib.rs
git -C third_party/ironrdp commit -m "feat(graphics): I32x4 lane type for SIMD inverse-DWT"
```

---

### Task 2: Diff-test + `decode_simd` for the hot variant

**Goal:** Implement the SIMD inverse-DWT for the variant detected in Task 0, gated bit-exact (`max|Δ|==0`) against the scalar `decode` over real coefficient buffers.

> **Detected hot variant (from Task 0, 2026-06-17): `extrapolate`** — port `dwt_extrapolate::decode` (row/col `idwt_row`/`idwt_col`, `/2` truncating rounding). Confirmed via the `[idwt-variant]` probe: 8 pool workers all logged `extrapolate`.

**Files:**
- Modify: `third_party/ironrdp/crates/ironrdp-graphics/src/dwt_extrapolate.rs` *(or `dwt.rs`)* — add `pub fn decode_simd(buffer: &mut [i16], temp: &mut [i16])`.
- Test: same file, `#[cfg(test)] mod simd_tests`.

**Acceptance Criteria:**
- [ ] `decode_simd` produces **byte-identical** output to scalar `decode` (`max|Δ|==0`) across: all-zeros, DC, H/V gradients, LCG-random, dense edge/boundary, negative-heavy, near-i16-overflow buffers.
- [ ] Signature and buffer contract match scalar `decode` (in-place, `>=4096` len).
- [ ] No change to scalar `decode` (it remains the oracle).

**Verify:** `cargo test -p session --lib idwt_simd_matches_scalar` → PASS with `max|Δ|==0`.

**Steps:**

- [ ] **Step 1: Write the failing diff-test** (real data construction, no mocks). In the target file:

```rust
#[cfg(test)]
mod simd_tests {
    use super::{decode, decode_simd};

    /// Build the canonical set of coefficient buffers exercised by the gate.
    fn test_buffers() -> Vec<[i16; 4096]> {
        let mut out = Vec::new();
        out.push([0i16; 4096]);                                   // zeros
        out.push([100i16; 4096]);                                 // DC
        let mut grad_h = [0i16; 4096];
        let mut grad_v = [0i16; 4096];
        for r in 0..64 { for c in 0..64 {
            grad_h[r * 64 + c] = (c as i16) * 4;
            grad_v[r * 64 + c] = (r as i16) * 4;
        }}
        out.push(grad_h);
        out.push(grad_v);
        // LCG pseudo-random (the dwt_extrapolate test idiom)
        let mut lcg = [0i16; 4096];
        let mut seed: u32 = 12345;
        for v in lcg.iter_mut() {
            seed = seed.wrapping_mul(1103515245).wrapping_add(12345);
            *v = ((seed >> 16) as i16) >> 4;
        }
        out.push(lcg);
        // Negative-heavy + near-i16-overflow edge values where wrap/rounding bugs hide.
        let edges = [-32768i16, -32767, -16384, -4096, -129, -1, 0, 1, 127, 4096, 16384, 32767];
        let mut edge = [0i16; 4096];
        for (i, v) in edge.iter_mut().enumerate() { *v = edges[i % edges.len()]; }
        out.push(edge);
        let mut neg = [0i16; 4096];
        for (i, v) in neg.iter_mut().enumerate() { *v = -((i as i16 % 4000) + 1); }
        out.push(neg);
        out
    }

    #[test]
    fn idwt_simd_matches_scalar() {
        let mut max_delta = 0i32;
        for buf in test_buffers() {
            let (mut a, mut b) = (buf, buf);
            let (mut ta, mut tb) = ([0i16; 4096], [0i16; 4096]);
            decode(&mut a, &mut ta);
            decode_simd(&mut b, &mut tb);
            for i in 0..4096 {
                max_delta = max_delta.max((i32::from(a[i]) - i32::from(b[i])).abs());
            }
        }
        assert_eq!(max_delta, 0, "SIMD inverse-DWT must be byte-exact with scalar decode");
    }
}
```

- [ ] **Step 2: Run to confirm red.**

Run: `cargo test -p session --lib idwt_simd_matches_scalar`
Expected: FAIL to compile — `decode_simd` not defined.

- [ ] **Step 3: Implement `decode_simd` red→green against the test.** This is the core engineering. The contract is the diff-test (`max|Δ|==0`); develop iteratively. Concrete strategy:
  - Port the scalar `decode` of the detected variant (in this same file — see scalar `decode`/`decode_block`/`idwt_row`/`idwt_col` for extrapolate, or `decode`/`decode_block`/`inverse_horizontal`/`inverse_vertical` for legacy).
  - Within each `decode_block`, **all rows of a band share the same `n_l`/`n_h`** ⇒ identical control flow ⇒ process **4 rows per `I32x4`** (and 4 columns for the vertical pass), packing strided i16 into lanes via `I32x4::from_lanes([...])` and storing back with `to_array()`.
  - Apply the **exactness rules** verbatim (these are the bug sources):
    - `as i16`/`t()` narrow → `.wrap_i16()` at every store site.
    - **rounding:** extrapolate `/2` → `.div2_trunc()`; legacy `>>1` → `.sra(1)`.
    - legacy `<<1`-on-i16-then-widen → `.shl(1).wrap_i16()` before treating as i32.
    - preserve even-then-odd sub-pass ordering; lanes never run along the dependency axis.
  - Handle tile-edge rows/cols whose count isn't a multiple of 4 with a scalar tail (call the existing scalar 1-D helper for the remainder), so correctness never depends on lane count.
  - Re-run the test after each pass (horizontal, then vertical) until `max|Δ|==0`.

  > Why no pre-written v128 body here: a non-trivial lifting kernel cannot be authored correctly without iterating against the oracle. The test above IS the executable spec; the rules + scalar source + lane strategy are the complete inputs. Do not fabricate a "finished" kernel — grow it green.

- [ ] **Step 4: Run to confirm green.**

Run: `cargo test -p session --lib idwt_simd_matches_scalar`
Expected: PASS, `max|Δ|==0`.

- [ ] **Step 5: Run the full graphics test suite** to ensure no regression in the scalar path:

Run: `cargo test -p session --lib`
Expected: all pass (including the existing `dwt_extrapolate` round-trip tests and `color_pass_byte_exact`).

- [ ] **Step 6: Commit.**

```bash
git -C third_party/ironrdp add crates/ironrdp-graphics/src/<variant_file>.rs
git -C third_party/ironrdp commit -m "feat(graphics): SIMD inverse-DWT (<variant>), byte-exact vs scalar"
```

---

### Task 3: Runtime SIMD toggle + `perfAB` single-command A/B harness

**Goal:** Make the SIMD path **runtime-flippable across all pool workers (no reload)** and deliver ONE console command — `await perfAB()` — that runs the full SIMD-off vs SIMD-on comparison and prints a single result. (Replaces the manual reload + toggle + two-`L.perf()` dance.)

**Files:**
- Modify: `third_party/ironrdp/crates/ironrdp-graphics/src/progressive.rs` — `set_simd_idwt` + `simd_idwt_enabled` + toggle-aware dispatch in `reconstruct_to_rgba`.
- Modify: `web/packages/shared/libs/player/crates/session/src/lib.rs` — `#[wasm_bindgen] pub fn set_simd_idwt(on: bool)` free export (one per wasm instance).
- Modify: `web/packages/shared/libs/player/rfxDecodeWorker.ts` — handle `{type:'set-simd-idwt', on}` → call the worker's wasm `set_simd_idwt(on)`.
- Modify: `web/packages/shared/libs/player/codecTestWorker.ts` — broadcast `set-simd-idwt` to ALL pool workers (+ its own instance); add a structured `perf-data` reply (the `perfSnapshot()` numbers, not the formatted string) and a `reset-perf` handler that clears the rolling accumulators.
- Modify: `web/packages/shared/libs/player/DesktopSessionTestMulti.tsx` — `setSimdIdwt`, `resetPerf`, `perfData`, and `perfAB` console globals; parse `?simd-idwt` for the initial default.

**Acceptance Criteria:**
- [ ] `setSimdIdwt(false|true)` in the console flips the decode path live on ALL pool workers — **no reload** (confirm via `stages.idwt` changing in `L.perf()`).
- [ ] `?simd-idwt=0` sets the initial default to scalar; absent/`=1` → SIMD.
- [ ] `await perfAB()` runs both phases and prints ONE comparison block (off vs on: `stages.idwt`, `present.p50/p95`, `inflight`, per-tile µs, idwt speedup) and returns `{off, on}` — **no reloads, no manual toggling**.
- [ ] Host test: toggle off vs on yields identical `reconstruct_to_rgba` RGBA for the implemented variant.

**Verify:** `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib reconstruct_toggle_identical` → PASS; in the browser with a video playing, `await perfAB()` prints both phases in one block.

**Steps:**

- [ ] **Step 1: Add the toggle to `progressive.rs`.**

```rust
/// Runtime switch for the SIMD inverse-DWT (A/B vs the scalar path). Default on.
static SIMD_IDWT: core::sync::atomic::AtomicBool = core::sync::atomic::AtomicBool::new(true);

/// Enable/disable the SIMD inverse-DWT at runtime (set from `?simd-idwt`).
pub fn set_simd_idwt(on: bool) {
    SIMD_IDWT.store(on, core::sync::atomic::Ordering::Relaxed);
}

#[inline]
fn simd_idwt_enabled() -> bool {
    SIMD_IDWT.load(core::sync::atomic::Ordering::Relaxed)
}
```

- [ ] **Step 2: Dispatch in `reconstruct_to_rgba`** (`progressive.rs:969`). Replace the IDWT block's variant calls with a toggle-aware version, e.g. for the extrapolate branch:

```rust
if self.use_reduce_extrapolate {
    let f = if simd_idwt_enabled() { crate::dwt_extrapolate::decode_simd } else { crate::dwt_extrapolate::decode };
    f(&mut y_buf, &mut temp);
    f(&mut cb_buf, &mut temp);
    f(&mut cr_buf, &mut temp);
} else {
    // legacy: SIMD only if Task 5 implemented it, else always scalar
    let mut dwt_temp = [0i16; COEFFICIENTS_PER_COMPONENT];
    crate::dwt::decode(&mut y_buf, &mut dwt_temp);
    crate::dwt::decode(&mut cb_buf, &mut dwt_temp);
    crate::dwt::decode(&mut cr_buf, &mut dwt_temp);
}
```

- [ ] **Step 3: Toggle-identity host test** (in `progressive.rs` tests): build a `TileState`, set known `display` coefficients with `use_reduce_extrapolate=true`, call `reconstruct_to_rgba` with `set_simd_idwt(true)` and `(false)`, assert the RGBA buffers are equal.

```rust
#[test]
fn reconstruct_toggle_identical() {
    // (construct a TileState with use_reduce_extrapolate = true and nonzero display)
    let mut on = [0u8; 64 * 64 * 4];
    let mut off = [0u8; 64 * 64 * 4];
    super::set_simd_idwt(true);  tile.reconstruct_to_rgba(&mut on);
    super::set_simd_idwt(false); tile.reconstruct_to_rgba(&mut off);
    super::set_simd_idwt(true);  // restore default
    assert_eq!(on, off, "SIMD/scalar IDWT must produce identical RGBA");
}
```

- [ ] **Step 4: wasm-bindgen export in `lib.rs`** (so JS can flip each wasm instance):

```rust
#[wasm_bindgen]
pub fn set_simd_idwt(on: bool) {
    ironrdp_graphics::progressive::set_simd_idwt(on);
}
```

- [ ] **Step 5: Pool worker handles the broadcast** (`rfxDecodeWorker.ts`). In its message switch (find the existing initialized-wasm-module binding and match it):

```ts
case 'set-simd-idwt':
  wasmModule.set_simd_idwt(msg.on);
  break;
```

- [ ] **Step 6: Broadcast + structured perf + reset in `codecTestWorker.ts`.**
  - `set-simd-idwt`: forward `{type:'set-simd-idwt', on}` to every pool worker AND call the local instance's `set_simd_idwt(on)`.
  - `reset-perf`: clear `rfxStages` and the present/arrival/barrier histograms (and post `reset-perf` to pool workers so their `takeRfxStageTimings` accumulators reset).
  - `perf-data`: reply with the STRUCTURED `perfSnapshot()` numbers `{stages:{tiles,entropy,idwt,color}, present:{p50,p95}, arrival:{p50}, inflight}` (reuse the values `perfSnapshot()` already computes; just return the object instead of the formatted string).

- [ ] **Step 7: The single command — globals in `DesktopSessionTestMulti.tsx`:**

```ts
(globalThis as any).setSimdIdwt = (on: boolean) => worker.postMessage({ type: 'set-simd-idwt', on });
(globalThis as any).resetPerf  = () => worker.postMessage({ type: 'reset-perf' });
(globalThis as any).perfData   = () => new Promise(res => {
  const onMsg = (ev: MessageEvent) => {
    if ((ev.data as any)?.type === 'perf-data') { worker.removeEventListener('message', onMsg); res((ev.data as any).data); }
  };
  worker.addEventListener('message', onMsg);
  worker.postMessage({ type: 'perf-data' });
});

// ONE command: full SIMD-off vs SIMD-on A/B, no reloads, no manual toggling.
(globalThis as any).perfAB = async (secs = 10) => {
  const sleep = (s: number) => new Promise(r => setTimeout(r, s * 1000));
  const sample = async (on: boolean) => {
    (globalThis as any).setSimdIdwt(on);
    (globalThis as any).resetPerf();
    await sleep(secs);                              // let rolling stats fill under this mode
    return await (globalThis as any).perfData();     // structured snapshot
  };
  const off = await sample(false);
  const on  = await sample(true);
  (globalThis as any).setSimdIdwt(true);            // restore default
  const tot = (p: any) => p.stages.entropy + p.stages.idwt + p.stages.color;
  const row = (tag: string, p: any) =>
    `${tag} idwt=${p.stages.idwt.toFixed(1)}us present.p50=${p.present.p50.toFixed(1)}ms ` +
    `inflight=${p.inflight} per-tile=${tot(p).toFixed(1)}us`;
  console.log('[perfAB]\n' + row('scalar', off) + '\n' + row('simd  ', on) +
    `\nidwt speedup ${(off.stages.idwt / on.stages.idwt).toFixed(2)}x   ` +
    `per-tile ${tot(off).toFixed(1)} -> ${tot(on).toFixed(1)}us`);
  return { off, on };
};
```

- [ ] **Step 8: Initial default.** Parse `?simd-idwt` in TSX, forward in the worker `host` message; in `lib.rs` session init call `set_simd_idwt(v !== '0')` (default on when absent).

- [ ] **Step 9: Tests + commit.**

Run: `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib reconstruct_toggle_identical` → PASS.

```bash
git -C third_party/ironrdp add crates/ironrdp-graphics/src/progressive.rs
git -C third_party/ironrdp commit -m "feat(graphics): runtime SIMD-IDWT toggle"
git add web/packages/shared/libs/player/crates/session/src/lib.rs web/packages/shared/libs/player/{rfxDecodeWorker,codecTestWorker}.ts web/packages/shared/libs/player/DesktopSessionTestMulti.tsx
git commit -m "feat(rdpclient): runtime setSimdIdwt + perfAB single-command A/B"
```

---

### Task 4: Release build + `L.perf` A/B measurement

**USER-ORDERED GATE — NON-SKIPPABLE.** This task was requested by the user in the current conversation ("lever B first, measure, then try A"). It MUST NOT be closed by walking around it, by declaring it "verified inline", or by substituting a cheaper check. Close only after every item in `acceptanceCriteria` has been re-validated independently, with output captured.

**Goal:** Measure the SIMD IDWT's real impact: `stages.idwt` drop and max full-screen videos @ 45 fps, SIMD-off vs SIMD-on, to complete the head-to-head with Lever B.

**Files:** none (measurement only).

**Acceptance Criteria:**
- [ ] Release wasm built (`./build-release-wasm.sh release`).
- [ ] Gate 1 holds: `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib idwt_simd_matches_scalar` PASS (`max|Δ|==0`).
- [ ] Gate 2 holds: video renders with **no black rectangles / artifacts** (visual confirmation, SIMD-on).
- [ ] `await perfAB()` output captured — SIMD-off (baseline) and SIMD-on (optimized) `stages.idwt` + per-tile µs in one block.
- [ ] Max full-screen videos @ 45 fps (before `inflight` climbs) recorded, and the SIMD-on tiles/s compared to Lever B's ~236k.

**Verify:** The single `await perfAB()` output block pasted into the result — both phases' `stages.idwt`, the idwt speedup, and present/inflight.

**Steps:**

- [ ] **Step 1: Build release wasm.** `cd web/packages/shared/libs/player && ./build-release-wasm.sh release`
- [ ] **Step 2: Gate 1.** `cd third_party/ironrdp && cargo test -p ironrdp-graphics --lib idwt_simd_matches_scalar` → PASS (`max|Δ|==0`).
- [ ] **Step 3: The single command.** Reload once, play ONE full-screen video, then run `await perfAB(10)`. It prints the SIMD-off vs SIMD-on block (per-tile µs, idwt speedup, present.p50, inflight) and returns `{off, on}` — no reloads, no manual toggling. Confirm no black rectangles while SIMD is on (Gate 2).
- [ ] **Step 4: Scale.** With `setSimdIdwt(true)`, add full-screen videos until `inflight` climbs / `present.p50` > ~22ms. Record max videos @ 45 fps and tiles/s (or re-run `perfAB` per video count).
- [ ] **Step 5: Record the head-to-head** in `RFX_IDWT_CEILING_HANDOFF.md` (SIMD tiles/s vs Lever B ~236k; recommend ship-SIMD vs invest-GPU per the doc's decision criteria).

---

### Task 5 (conditional): SIMD `decode_simd` for the second variant

**Goal:** If Task 0 showed both variants occur (or the other variant later becomes hot), vectorize it too. Same structure as Task 2 against the other file's scalar `decode`.

**Files:** Modify the other of `dwt.rs` / `dwt_extrapolate.rs`.

**Acceptance Criteria:**
- [ ] `decode_simd` for the second variant `max|Δ|==0` vs its scalar `decode` over the Task-2 buffer set.
- [ ] Dispatch in `reconstruct_to_rgba` uses SIMD for both variants under the toggle.

**Verify:** `cargo test -p session --lib idwt_simd_matches_scalar` (extended to the second variant) → PASS.

**Steps:** Repeat Task 2 Steps 1–6 for the second variant (build its own `test_buffers`-backed diff-test in that file; apply the variant's rounding rule — `>>1` for legacy, `/2` for extrapolate), then update the Task-3 dispatch legacy branch to call `decode_simd`.

---

## Risks

- v128 pack/transpose overhead for strided column passes may undercut the literature 2.6–4× — Task 4 measures it; if `stages.idwt` barely moves, stop and report (Lever B remains the throughput leader, gated on architecture).
- Vendored `third_party/` edits are untracked in the main tree — commit them in the `third_party/ironrdp` repo and list them in any handoff.
