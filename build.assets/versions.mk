# Keep all tool versions in one place.
# This file can be included in other Makefiles to avoid duplication.

# Sync with devbox.json.
GOLANG_VERSION ?= go1.20.7

# Sync with devbox.json.
NODE_VERSION ?= 16.18.1

# Sync any version changes below with devbox.json.
# run lint-rust check locally before merging code after you bump this
RUST_VERSION ?= 1.71.1
LIBBPF_VERSION ?= 1.0.1
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

# Sync any version changes below with devbox.json.
# Protogen related versions.
BUF_VERSION ?= 1.26.1
# Keep in sync with api/proto/buf.yaml (and buf.lock)
GOGO_PROTO_TAG ?= v1.3.2
NODE_GRPC_TOOLS_VERSION ?= 1.12.4
NODE_PROTOC_TS_VERSION ?= 5.0.1
PROTOC_VER ?= 3.20.3
