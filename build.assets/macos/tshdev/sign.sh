#!/bin/bash
set -eu

readonly CERTHASH='A5604F285B0957134EA099AC515BD9E0787228AC'
readonly APP='K497G57PDJ.com.goteleport.tshdev'

main() {
  if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <binary>"
    exit 1
  fi

  local dir=
  dir="$(dirname "$0")"
  codesign -f \
    -o kill,hard,runtime \
    -s "$CERTHASH" \
    -i "$APP" \
    --entitlements "$dir/tshdev.entitlements" \
    --timestamp \
    "$@"
}

main "$@"
