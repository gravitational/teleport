#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cleanup() {
  if [ -n "${TELEPORT_PID:-}" ]; then
    echo "Stopping Teleport (PID: $TELEPORT_PID)..."
    kill "$TELEPORT_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Build teleport
echo "==> Building Teleport..."
make -C "$REPO_ROOT" binaries

# Install e2e deps if needed
echo "==> Installing e2e dependencies..."
cd "$SCRIPT_DIR"
pnpm install
pnpm exec playwright install --with-deps chromium

# Start teleport
echo "==> Starting Teleport..."
"$SCRIPT_DIR/run.sh"
TELEPORT_PID=$(pgrep -nf "teleport start")

# Run tests
echo "==> Running e2e tests..."
pnpm test
