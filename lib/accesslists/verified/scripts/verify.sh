#!/usr/bin/env bash
# Runs the Lean4 proof checker on the formal verification project.
#
# Prerequisites:
#   - elan (Lean4 version manager)
#   - lake (Lean4 build tool, installed via elan)
#
# See the README.md for installation instructions.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LEAN_DIR="$(dirname "$SCRIPT_DIR")/lean4"

# Check prerequisites
if ! command -v lake &>/dev/null; then
    echo "Error: lake not found. Install elan first:"
    echo "  curl -sSf https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh | sh"
    exit 1
fi

echo "==> Building and verifying Lean4 proofs..."
cd "$LEAN_DIR"
lake build FormalVerification.Theorems

echo "==> All proofs verified successfully!"
