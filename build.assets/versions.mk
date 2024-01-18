# Keep all tool versions in one place.
# This file can be included in other Makefiles to avoid duplication.
# Keep versions in sync with devbox.json, when applicable.

# Sync with devbox.json.
GOLANG_VERSION ?= go1.21.6

NODE_VERSION ?= 18.18.2

# Run lint-rust check locally before merging code after you bump this.
RUST_VERSION ?= 1.71.1
WASM_PACK_VERSION ?= 0.12.1
LIBBPF_VERSION ?= 1.2.2
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

DEVTOOLSET ?= devtoolset-12

# Protogen related versions.
BUF_VERSION ?= v1.28.0
# Keep in sync with api/proto/buf.yaml (and buf.lock).
GOGO_PROTO_TAG ?= v1.3.2
PROTOBUF_TS_PLUGIN_VERSION ?= 2.9.3
PROTOC_VERSION ?= 3.20.3
