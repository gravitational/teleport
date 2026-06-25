#!/usr/bin/env bash
#
# Downloads and installs protoc to the given directory at the given version.
# Usage: install-protoc.sh <install-dir> <version>

set -eu

INSTALL_DIR=${1:?install dir required}
VERSION=${2:?version required}

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/aarch64/aarch_64/')
TMP=$(mktemp /tmp/protoc-XXXXXX.zip)
trap 'rm -f "$TMP"' EXIT

mkdir -p "$INSTALL_DIR"
curl -fsSL "https://github.com/protocolbuffers/protobuf/releases/download/v${VERSION}/protoc-${VERSION}-${OS}-${ARCH}.zip" \
    -o "$TMP"
unzip -p "$TMP" bin/protoc > "$INSTALL_DIR/protoc"
chmod +x "$INSTALL_DIR/protoc"
# Extract well-known .proto files alongside the binary
unzip -o "$TMP" "include/*" -d "$(dirname "$INSTALL_DIR")"
echo "Installed protoc ${VERSION} to ${INSTALL_DIR}/protoc"