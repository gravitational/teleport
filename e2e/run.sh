#!/bin/sh
export E2E_CALLER_DIR="$PWD"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

# If any argument points to an enterprise e2e test, delegate to e/e2e/run.sh.
for arg in "$@"; do
  case "$arg" in
    -*) continue ;;
    e/e2e/* | */e/e2e/* | e/e2e | */e/e2e)
      exec "$REPO_ROOT/e/e2e/run.sh" "$@"
      ;;
  esac
done

cd "$SCRIPT_DIR/runner" && GOWORK=off go build -o e2e . && exec ./e2e "$@"
