#!/usr/bin/env bash
#
# Generates protos for Teleport and Teleport API.
set -eu

main() {
  cd "$(dirname "$0")"  # ./build-assets/
  cd ../                # teleport root

  # Clean gen/proto directories before regenerating them. Legacy protos are
  # generated all over the directory tree, so they won't get cleaned up
  # automatically if the proto is deleted.
  rm -fr api/gen/proto gen/proto

  # Generate Gogo protos. Generated protos are written to
  # gogogen/github.com/gravitational/teleport/..., so we copy them to the
  # correct relative path. This is in lieu of the module= option, which would do
  # this for us (and which is what we use for the non-gogo protogen).
  rm -fr gogogen
  trap 'rm -fr gogogen' EXIT # don't leave files behind
  buf generate --template=buf-gogo.gen.yaml \
    --path=api/proto/teleport/legacy/ \
    --path=api/proto/teleport/attestation/ \
    --path=api/proto/teleport/usageevents/ \
    --path=proto/teleport/lib/web/envelope.proto
  cp -r gogogen/github.com/gravitational/teleport/. .
  # error out if there's anything outside of github.com/gravitational/teleport
  rm -fr gogogen/github.com/gravitational/teleport
  rmdir gogogen/github.com/gravitational gogogen/github.com gogogen

  # Generate protoc-gen-go protos (preferred).
  buf generate --template=buf-go.gen.yaml \
    --exclude-path=api/proto/teleport/legacy/ \
    --exclude-path=api/proto/teleport/attestation/ \
    --exclude-path=api/proto/teleport/usageevents/ \
    --exclude-path=proto/teleport/lib/web/envelope.proto \
    --exclude-path=proto/prehog/

  # Generate connect-go protos.
  buf generate --template=buf-connect-go.gen.yaml \
    --path=proto/prehog/

  # Generate JS protos.
	buf generate --template=buf-js.gen.yaml \
    --path=proto/prehog/ \
    --path=proto/teleport/lib/teleterm/
}

main "$@"
