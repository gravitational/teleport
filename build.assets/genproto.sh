#!/bin/bash
#
# Builds, formats, lints and generates protos for teleport and teleport/api.
#
# Env variables:
# - PROTO_INCLUDE: `protoc -I` extension with path to imported protos
#   (eg, Gogo import path).
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

  # Build legacy protos.
  # Output path is the same as their old path, inside Go packages.
  for p in client/proto types/{events,webauthn,wrappers,}; do
    echoed protoc -I="./api/proto/:$PROTO_INCLUDE" \
      --gogofast_out='plugins=grpc:.' \
      "api/proto/teleport/legacy/$p"/*.proto
  done

  # Generated protos are written to
  # <teleport-root>/github.com/gravitational/teleport/..., so we copy them to
  # the correct relative path.
  # TODO(codingllama): Find a way to generate in the correct path.
  for f in $(find github.com -name '*.pb.go'); do
    mv "$f" "${f#github.com/gravitational/teleport/}"
  done

  rm -fr github.com/  # Remove empty generated root.
}

main "$@"
