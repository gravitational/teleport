#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(realpath "$(dirname -- "${BASH_SOURCE[0]}")")"
VERSION="$(tr -d '\r\n' < "$SCRIPT_DIR/helm-unittest.version")"
VERSION_NUMBER="${VERSION#v}"
OUTPUT="$SCRIPT_DIR/helm-unittest.sha256"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

# This manifest is the single source of truth for the expected helm-unittest
# executable checksums. The Dockerfile, Makefile instructions, and GitHub Actions
# verify the extracted binary against it rather than trusting the release archive.
checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

download() {
  local platform="$1"
  local archive="helm-unittest-${platform}-${VERSION_NUMBER}.tgz"
  local directory="$TEMP_DIR/$platform"

  mkdir -p "$directory"
  curl -fsSL \
    "https://github.com/helm-unittest/helm-unittest/releases/download/${VERSION}/${archive}" \
    | tar -xz -C "$directory"
}

download linux-amd64
download linux-arm64
download macos-arm64

printf '%s\n' \
  "${VERSION} linux amd64 $(checksum "$TEMP_DIR/linux-amd64/untt")" \
  "${VERSION} linux arm64 $(checksum "$TEMP_DIR/linux-arm64/untt")" \
  "${VERSION} macos arm64 $(checksum "$TEMP_DIR/macos-arm64/untt")" \
  > "$TEMP_DIR/helm-unittest.sha256"

mv "$TEMP_DIR/helm-unittest.sha256" "$OUTPUT"
