# Gravitational Teleport Web UI

This package contains the source code of Teleport Web UI.

## Development

Follow the instructions from `web/README.md`.

#### wasm-pack

The [`wasm-pack`](https://github.com/rustwasm/wasm-pack) version in [build.assets/Makefile](https://github.com/gravitational/teleport/blob/master/build.assets/versions.mk#L12) (search for `WASM_PACK_VERSION`) is required to build the WebAssembly module.

When calling `wasm-pack`, we set the environment variable `RUST_MIN_STACK=16777216`. This is necessary to avoid a `SIGSEGV` error when building the module on some systems.

`16777216` was chosen based on [the suggestion in the rust compiler error message](https://github.com/rust-lang/rust/blob/10a7aa14fed9b528b74b0f098c4899c37c09a9c7/compiler/rustc_driver_impl/src/signal_handler.rs#L104-L106) to double the [`DEFAULT_STACK_SIZE`](https://github.com/rust-lang/rust/blob/10a7aa14fed9b528b74b0f098c4899c37c09a9c7/compiler/rustc_interface/src/util.rs#L52) value.
