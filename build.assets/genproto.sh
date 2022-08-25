#!/bin/bash
#
# Builds, formats, lints and generates protos for teleport and teleport/api.
set -eu

echoed() {
  echo "$*" >&2
  "$@"
}

main() {
  cd "$(dirname "$0")"  # ./build-assets/
  cd ../                # teleport root

  buf build
  buf lint
  buf format -w

  # Generated protos are written to
  # <teleport-root>/github.com/gravitational/teleport/..., so we copy them to
  # the correct relative path.
  buf generate
  find github.com -name '*.pb.go' | while read -r f; do
    mv "$f" "${f#github.com/gravitational/teleport/}"
  done
  rm -fr github.com/  # Remove empty generated root.
}

main "$@"
