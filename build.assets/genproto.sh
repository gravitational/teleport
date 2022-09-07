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
  trap 'rm -fr github.com' EXIT   # don't leave github.com/ behind
  rm -fr api/gen/proto gen/proto  # cleanup gen/proto folders
  buf generate
  cp -r github.com/gravitational/teleport/* .
}

main "$@"
