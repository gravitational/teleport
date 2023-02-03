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
  rm -fr api/gen/proto gen/proto lib/teleterm/api/protogen lib/prehog/gen lib/prehog/gen-js

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

  # Generate lib/teleterm & JS protos.
  # TODO(ravicious): Refactor generating JS protos to follow the approach from above, that is have a
  # separate call to generate Go protos and another for JS protos instead of having
  # teleterm-specific buf.gen.yaml files.
  # https://github.com/gravitational/teleport/pull/19774#discussion_r1061524458
	buf generate --template=lib/prehog/buf-teleterm.gen.yaml lib/prehog/proto
	buf generate --template=lib/teleterm/buf.gen.yaml lib/teleterm/api/proto

  cp -r github.com/gravitational/teleport/* .
}

main "$@"
