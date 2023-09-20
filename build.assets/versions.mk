# Keep all tool versions in one place.
# This file can be included in other Makefiles to avoid duplication.

GOLANG_VERSION ?= go1.20.8

NODE_VERSION ?= 18.18.0

# Run lint-rust check locally before merging code after you bump this.
RUST_VERSION ?= 1.68.0
LIBBPF_VERSION ?= 1.0.1
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

# Protogen related versions.
BUF_VERSION ?= 1.26.1
# Keep in sync with api/proto/buf.yaml (and buf.lock).
GOGO_PROTO_TAG ?= v1.3.2
NODE_GRPC_TOOLS_VERSION ?= 1.12.4
NODE_PROTOC_TS_VERSION ?= 5.0.1
PROTOC_VER ?= 3.20.3
