#!/bin/bash
#
# Generates protos for Teleport and Teleport API.
set -eu

main() {
  cd "$(dirname "$0")"  # ./build-assets/
  cd ../                # teleport root

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
