#!/usr/bin/env bash
# Usage: fetch_verify.sh <url> <sha256> <output>
set -euo pipefail

URL="$1"
EXPECTED="$2"
OUT="$3"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

DOWNLOAD="${TMP_DIR}/$(basename "${OUT}")"
curl -fsSL -o "${DOWNLOAD}" "${URL}"

actual="$(_sha256 "${DOWNLOAD}")"
if [[ "${actual}" != "${EXPECTED}" ]]; then
  echo "error: ${URL}: expected sha256 ${EXPECTED}, got ${actual}" >&2
  exit 1
fi

mkdir -p "$(dirname "${OUT}")"
mv "${DOWNLOAD}" "${OUT}"
