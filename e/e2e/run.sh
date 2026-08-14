#!/bin/sh
export E2E_CALLER_DIR="${E2E_CALLER_DIR:-$PWD}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENT_REPO_ROOT="$(dirname "$SCRIPT_DIR")"
OSS_REPO_ROOT="$(dirname "$ENT_REPO_ROOT")"
OSS_E2E_DIR="$OSS_REPO_ROOT/e2e"

export E2E_DIR="$SCRIPT_DIR"
export E2E_SHARED_DIR="$OSS_E2E_DIR"

mkdir -p "$OSS_E2E_DIR/.auth"
rm -rf "$SCRIPT_DIR/.auth"
ln -sfn "$OSS_E2E_DIR/.auth" "$SCRIPT_DIR/.auth"

license_set=0
for arg in "$@"; do
  case "$arg" in
    --license-file | --license-file=*)
      license_set=1
      break
      ;;
  esac
done

if [ "$license_set" -eq 0 ]; then
  set -- --license-file "$ENT_REPO_ROOT/fixtures/license-eub.pem" "$@"
fi

cd "$OSS_E2E_DIR/runner" && GOWORK=off go build -o e2e . && exec ./e2e "$@"
