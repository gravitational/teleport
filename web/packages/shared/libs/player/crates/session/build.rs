// Build script.
//
// Only does anything when `--features freerdp-oracle` is enabled on a host
// target — locates libfreerdp3 + libwinpr3 via pkg-config so the FFI in
// `progressive::oracle` links cleanly. For wasm builds and the default
// (no-feature) cargo test, this is a no-op.

fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    println!("cargo:rerun-if-env-changed=PKG_CONFIG_PATH");

    let oracle_enabled = std::env::var_os("CARGO_FEATURE_FREERDP_ORACLE").is_some();
    let target_arch = std::env::var("CARGO_CFG_TARGET_ARCH").unwrap_or_default();
    if !oracle_enabled || target_arch == "wasm32" {
        return;
    }

    // First probe WITHOUT emitting cargo metadata — we only want the include +
    // link-search paths here. The `-lfreerdp3 -lwinpr3` directives are emitted
    // LATER (after the C wrapper's static archive) so link order is correct:
    // the archive that references FreeRDP symbols must precede `-lfreerdp3`, or
    // the linker's `--as-needed` drops the dylib and the symbols go undefined.
    let mut include_paths = Vec::new();
    for lib in ["freerdp3", "winpr3"] {
        match pkg_config::Config::new().cargo_metadata(false).probe(lib) {
            Ok(l) => {
                include_paths.extend(l.include_paths);
                for p in &l.link_paths {
                    println!("cargo:rustc-link-search=native={}", p.display());
                }
            }
            Err(e) => {
                panic!(
                    "feature `freerdp-oracle` is enabled but pkg-config could not find {lib}: {e}\n\
                     On macOS Homebrew, run `brew install freerdp` and ensure PKG_CONFIG_PATH includes \
                     $(brew --prefix)/lib/pkgconfig before invoking cargo.\n\
                     On Debian/Ubuntu, install `freerdp3-dev` and `libwinpr3-dev`."
                );
            }
        }
    }

    // Compile the C wrapper around FreeRDP's `progressive_decompress`. cc emits
    // `cargo:rustc-link-lib=static=freerdp_progressive_ref`.
    let c_src = "src/progressive/freerdp_progressive_ref.c";
    println!("cargo:rerun-if-changed={c_src}");
    let mut build = cc::Build::new();
    build.file(c_src).opt_level(2).warnings(false);
    for inc in &include_paths {
        build.include(inc);
    }
    build.compile("freerdp_progressive_ref");

    // NOW emit the FreeRDP/WinPR link directives, so they land AFTER the static
    // archive on the final link line (fixes the `--as-needed` drop above).
    println!("cargo:rustc-link-lib=dylib=freerdp3");
    println!("cargo:rustc-link-lib=dylib=winpr3");
}
