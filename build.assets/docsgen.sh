#!/usr/bin/env bash
set -eu

main() {
  cd "$(dirname "$0")"  # ./build-assets/
  cd ../                # teleport root

  # Generate from custom go generator.
  go run ./docs/gen/
}

main "$@"
