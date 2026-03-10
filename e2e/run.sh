#!/bin/sh
export E2E_CALLER_DIR="$PWD"
cd "$(dirname "$0")/runner" && go build -o e2e . && exec ./e2e "$@"
