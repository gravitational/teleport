#!/bin/sh
export E2E_CALLER_DIR="$PWD"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

cd "$SCRIPT_DIR/runner" && GOWORK=off go build -o e2e . && exec ./e2e "$@"
