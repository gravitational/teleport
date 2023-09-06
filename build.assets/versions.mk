# Keep all tool versions in one place.
# This file can be included in other Makefiles to avoid duplication.
# Keep versions in sync with devbox.json, when applicable.

# Sync with devbox.json.
GOLANG_VERSION ?= go1.21.1

NODE_VERSION ?= 18.17.1

# Run lint-rust check locally before merging code after you bump this.
RUST_VERSION ?= 1.71.1
WASM_PACK_VERSION ?= 0.11.0
LIBBPF_VERSION ?= 1.0.1
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

# Protogen related versions.
# Keep in sync with api/proto/buf.yaml (and buf.lock).
GOGO_PROTO_TAG ?= v1.3.2
NODE_GRPC_TOOLS_VERSION ?= 1.12.4
NODE_PROTOC_TS_VERSION ?= 5.0.1
PROTOC_VERSION ?= 3.20.3
