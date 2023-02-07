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
  # cleanup gen/proto folders
  rm -fr api/gen/proto gen/proto lib/teleterm/api/protogen
  buf generate api/proto
  buf generate proto

  buf generate --template=lib/teleterm/buf.gen.yaml lib/teleterm/api/proto

  cp -r github.com/gravitational/teleport/* .
}

main "$@"
