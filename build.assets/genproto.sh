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

  # Generate Gogo protos.
  buf generate --template=buf-gogo.gen.yaml api/proto
  buf generate --template=buf-gogo.gen.yaml proto

  # Generate protoc-gen-go protos (preferred).
  # Add your protos to the list if you can.
  buf generate --template=buf-go.gen.yaml \
    --path=api/proto/teleport/devicetrust/ \
    --path=api/proto/teleport/loginrule/ \
    --path=api/proto/teleport/proxy/ \
    --path=proto/teleport/lib/multiplexer/
  buf generate --template=lib/prehog/buf.gen.yaml lib/prehog/proto

  cp -r github.com/gravitational/teleport/* .
}

main "$@"
