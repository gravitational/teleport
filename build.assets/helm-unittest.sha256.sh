#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(realpath "$(dirname -- "${BASH_SOURCE[0]}")")"
VERSION="$(tr -d '\r\n' < "$SCRIPT_DIR/helm-unittest.version")"
VERSION_NUMBER="${VERSION#v}"
OUTPUT="$SCRIPT_DIR/helm-unittest.sha256"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_CMD=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_CMD=(shasum -a 256)
else
  echo "Neither sha256sum nor shasum is available" >&2
  exit 1
fi

download() {
  local platform="$1"
  local archive="helm-unittest-${platform}-${VERSION_NUMBER}.tgz"
  local path="$TEMP_DIR/$archive"

  (set -x; curl -fsSL -o "$path" \
    "https://github.com/helm-unittest/helm-unittest/releases/download/${VERSION}/${archive}")

}

download linux-amd64
download linux-arm64
download macos-arm64

cd "$TEMP_DIR"
(set -x; "${CHECKSUM_CMD[@]}" -- *.tgz) > "${OUTPUT}"
