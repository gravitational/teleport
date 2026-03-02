# Keep all tool versions in one place.
# This file can be included in other Makefiles to avoid duplication.

# Sync with devbox.json.
GOLANG_VERSION ?= go1.25.7
GOLANGCI_LINT_VERSION ?= v2.10.1

# NOTE: Remember to update engines.node in package.json to match the major version.
NODE_VERSION ?= 24.13.0

WASM_OPT_VERSION ?= 0.116.1
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

DEVTOOLSET ?= devtoolset-12

# Protogen related versions.
BUF_VERSION ?= v1.66.0
# Keep in sync with api/proto/buf.yaml (and buf.lock).
GOGO_PROTO_TAG ?= v1.3.2
NODE_GRPC_TOOLS_VERSION ?= 1.12.4
NODE_PROTOC_TS_VERSION ?= 5.0.1
PROTOC_VERSION ?= 26.1
