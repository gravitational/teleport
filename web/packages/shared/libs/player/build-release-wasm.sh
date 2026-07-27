#!/usr/bin/env bash
# Build the codec-test `session` wasm crate and regenerate its JS/TS bindings.
#
# Usage:
#   build-release-wasm.sh [debug|release]
#     debug   (default) — fast unoptimized build, skips wasm-opt. The wasm-bindgen
#                         output (JS + .d.ts) is identical to release; only the
#                         wasm binary is larger/slower. Use for the dev inner loop.
#     release           — optimized: fat-LTO + panic=abort cargo profile, then
#                         wasm-opt -O3 with the SIMD/bulk-memory feature flags.
#                         Requires wasm-opt (binaryen) on PATH.
#
# Mirrors the source player's `build-player-wasm` Makefile target: invoke
# cargo + wasm-opt + wasm-bindgen directly so we can pass the SIMD enable flags
# to wasm-opt and the fat-LTO / panic=abort cargo profile overrides without
# going through wasm-pack (which 0.13.x doesn't accept a custom `--profile` for
# and re-runs wasm-opt without the feature flags).
#
# `.cargo/config.toml` provides --cfg getrandom_backend="wasm_js" and the
# target-feature RUSTFLAGS, so we don't need to pass RUSTFLAGS here.
set -euo pipefail

mode="${1:-debug}"
case "$mode" in
  ""|debug|dev|fast) mode="debug" ;;
  release|opt|optimized|--release) mode="release" ;;
  *)
    echo "usage: $(basename "$0") [debug|release]   (default: debug)" >&2
    exit 2
    ;;
esac

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../../../../.." && pwd)"
out_dir="$here/pkg/session"
target_dir="$repo/target"

mkdir -p "$out_dir"

if [ "$mode" = "release" ]; then
  # Optimized: fat-LTO release build, then a wasm-opt -O3 pass with the feature
  # flags. `bindgen_input` is the wasm-opt output.
  unopt="$target_dir/wasm32-unknown-unknown/release/session.wasm"
  bindgen_input="$target_dir/wasm32-unknown-unknown/release/session.opt.wasm"
  (
    cd "$repo"
    # --features rfx-stage-timing: throwaway per-stage RFX decode timers (entropy
    # / IDWT / color / blit) for the [apply] breakdown (Step 1 measurement).
    # Drop this flag when stripping the instrumentation.
    cargo build \
      --package session \
      --lib \
      --features rfx-stage-timing \
      --target wasm32-unknown-unknown \
      --release \
      --config 'profile.release.lto="fat"' \
      --config 'profile.release.panic="abort"' \
      --config 'profile.release.strip="symbols"' \
      --config 'profile.release.debug=0' \
      --config 'profile.release.codegen-units=1'
  )

  # wasm-opt is a bonus pass; the fat-LTO -O3 cargo build above is ~90% of the
  # win on its own. Some binaryen versions (e.g. v116) can't parse newer
  # rustc/wasm-bindgen output ("invalid code after misc prefix: NN"). So if
  # wasm-opt is missing OR fails, fall back to shipping the un-wasm-opt'd release
  # wasm instead of failing the whole build.
  if command -v wasm-opt >/dev/null 2>&1 && \
     wasm-opt "$unopt" \
       -o "$bindgen_input" \
       -O3 \
       --enable-simd \
       --enable-bulk-memory \
       --enable-nontrapping-float-to-int \
       --enable-sign-ext; then
    echo "[release] wasm-opt -O3 applied"
  else
    echo "warning: wasm-opt unavailable or failed — shipping the fat-LTO -O3 release wasm WITHOUT the wasm-opt pass (still ~90% of the win; upgrade binaryen to close the gap)." >&2
    bindgen_input="$unopt"
  fi
else
  # Fast unoptimized build: debug profile, no wasm-opt. wasm-bindgen consumes the
  # debug wasm directly; the emitted JS/.d.ts match a release build exactly.
  bindgen_input="$target_dir/wasm32-unknown-unknown/debug/session.wasm"
  (
    cd "$repo"
    # --features rfx-stage-timing: throwaway per-stage RFX decode timers (entropy
    # / IDWT / color / blit) for the [apply] breakdown (Step 1 measurement).
    # Drop this flag when stripping the instrumentation.
    cargo build \
      --package session \
      --lib \
      --features rfx-stage-timing \
      --target wasm32-unknown-unknown
  )
fi

# The wasm-bindgen CLI must EXACTLY match the `wasm-bindgen` crate version in
# Cargo.lock (the bindgen schema is unstable, so a 0.2.118 CLI cannot consume a
# 0.2.121-generated wasm and vice-versa). A different wasm-bindgen earlier on
# $PATH (Homebrew / a mise/asdf shim) otherwise shadows the cargo-installed one
# and fails every build with "schema version mismatch". Install the exact
# version into a repo-local, version-keyed dir and invoke it by ABSOLUTE PATH,
# bypassing $PATH entirely. Cached across builds; only (re)installs on a bump.
wb_version="$(awk '/^name = "wasm-bindgen"$/{getline; gsub(/[" ]/,""); sub(/version=/,""); print; exit}' "$repo/Cargo.lock")"
wb_root="$target_dir/wasm-bindgen-cli/$wb_version"
wb_bin="$wb_root/bin/wasm-bindgen"
if [ "$("$wb_bin" --version 2>/dev/null | awk '{print $2}')" != "$wb_version" ]; then
  echo "Installing wasm-bindgen-cli $wb_version to match Cargo.lock (one-time per version) ..."
  cargo install --root "$wb_root" --force wasm-bindgen-cli --version "$wb_version"
fi

"$wb_bin" "$bindgen_input" \
  --out-dir "$out_dir" \
  --out-name session \
  --typescript \
  --target web

echo "[$mode] bindings written to $out_dir"
ls -lh "$out_dir/session_bg.wasm"
