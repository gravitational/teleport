#!/usr/bin/env bash
# Usage: fetch_verify.sh <base-url> <manifest> <dir> <name>...
#
# Names are relative to <dir> and are appended to <base-url> to build each
# download URL.
set -euo pipefail

if [[ $# -lt 4 ]]; then
  echo "usage: $(basename "$0") <base-url> <manifest> <dir> <name>..." >&2
  exit 2
fi

if [[ ! -f "$2" ]]; then
  echo "error: manifest not found: $2" >&2
  exit 1
fi

BASE_URL="${1%/}"
MANIFEST="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
DIR="$3"
shift 3

mkdir -p "${DIR}"

for name in "$@"; do
  if [[ -e "${DIR}/${name}" ]]; then
    continue
  fi
  curl -fsSL --create-dirs -o "${DIR}/${name}" "${BASE_URL}/${name}"
done

if command -v sha256sum >/dev/null 2>&1; then
  check=(sha256sum -c --strict "${MANIFEST}")
else
  check=(shasum -a 256 -c --strict "${MANIFEST}")
fi

if ! (cd "${DIR}" && "${check[@]}"); then
  echo "error: ${DIR} does not match ${MANIFEST}" >&2
  echo "       update the manifest by hand if a pinned version changed," >&2
  echo "       or delete the file to re-fetch it" >&2
  exit 1
fi
