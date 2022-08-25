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
  # TODO(codingllama): Find a way to generate in the correct path.
  buf generate
  for f in $(find github.com -name '*.pb.go'); do
    mv "$f" "${f#github.com/gravitational/teleport/}"
  done
  rm -fr github.com/  # Remove empty generated root.
}

main "$@"
