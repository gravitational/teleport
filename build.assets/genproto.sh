#!/usr/bin/env bash
#
# Generates protos for Teleport and Teleport API.
set -eu

echoed() {
  echo "$*" >&2
  "$@"
}

main() {
  cd "$(dirname "$0")"  # ./build-assets/
  cd ../                # teleport root

  # Parse optional args.
  local skip_js=0 # skips Javascript and Typescript protogen
  local skip_rm=0 # skips removal of old protos
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-js)
        skip_js=1
        ;;
      --skip-rm)
        skip_rm=1
        ;;
      *)
        echo "Unknown argument $1" >&2
        exit 1
        ;;
    esac
    shift
  done

  # Clean gen/proto directories before regenerating them. Legacy protos are
  # generated all over the directory tree, so they won't get cleaned up
  # automatically if the proto is deleted.
  [[ $skip_rm -eq 0 ]] && echoed rm -fr api/gen/proto gen/proto

  # Generate Gogo protos. Generated protos are written to
  # gogogen/github.com/gravitational/teleport/..., so we copy them to the
  # correct relative path. This is in lieu of the module= option, which would do
  # this for us (and which is what we use for the non-gogo protogen).
  rm -fr gogogen
  trap 'rm -fr gogogen' EXIT # don't leave files behind
  echoed buf generate --template=buf-gogo.gen.yaml
  cp -r gogogen/github.com/gravitational/teleport/. .
  # error out if there's anything outside of github.com/gravitational/teleport
  rm -fr gogogen/github.com/gravitational/teleport
  rmdir gogogen/github.com/gravitational gogogen/github.com gogogen

  # Generate go, go-grpc and connect-go protos (preferred).
  echoed buf generate --template=buf-go.gen.yaml
  echoed buf generate --template=buf-connect-go.gen.yaml

  # Generate TS protos.
  [[ $skip_js -eq 0 ]] && echoed buf generate --template=buf-ts.gen.yaml
}

main "$@"
