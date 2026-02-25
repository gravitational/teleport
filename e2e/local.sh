#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MODE="test"
BUILD=true

usage() {
  echo "Usage: $0 [--no-build] [--ui | --codegen | --debug]"
  echo ""
  echo "  --no-build   Skip building Teleport"
  echo "  --ui         Open Playwright UI mode"
  echo "  --codegen    Open Playwright codegen against running Teleport"
  echo "  --debug      Run tests with Playwright inspector"
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-build) BUILD=false; shift ;;
    --ui) MODE="ui"; shift ;;
    --codegen) MODE="codegen"; shift ;;
    --debug) MODE="debug"; shift ;;
    --help|-h) usage ;;
    *) echo "Unknown option: $1"; usage ;;
  esac
done

cleanup() {
  if [ -n "${TELEPORT_PID:-}" ]; then
    echo "Stopping Teleport (PID: $TELEPORT_PID)..."
    kill "$TELEPORT_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Build teleport
if [ "$BUILD" = false ]; then
  echo "==> Skipping build (--no-build)"
elif [ -f "$REPO_ROOT/build/teleport" ]; then
  echo "==> Rebuilding Teleport (incremental)..."
  make -C "$REPO_ROOT" binaries
else
  echo "==> Building Teleport..."
  make -C "$REPO_ROOT" binaries
fi

# Install e2e deps if needed
echo "==> Installing e2e dependencies..."
cd "$SCRIPT_DIR"
pnpm install
pnpm exec playwright install --with-deps chromium

# Start teleport
echo "==> Starting Teleport..."
"$SCRIPT_DIR/run.sh"
TELEPORT_PID=$(pgrep -nf "teleport start")

# Run in selected mode
case "$MODE" in
  test)
    echo "==> Running e2e tests..."
    pnpm test
    ;;
  ui)
    echo "==> Opening Playwright UI mode..."
    pnpm exec playwright test --ui
    ;;
  codegen)
    echo "==> Opening Playwright codegen..."
    pnpm exec playwright codegen --ignore-https-errors https://localhost:3080/web
    ;;
  debug)
    echo "==> Running tests with Playwright inspector..."
    PWDEBUG=1 pnpm test
    ;;
esac
