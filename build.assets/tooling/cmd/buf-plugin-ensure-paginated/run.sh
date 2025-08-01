#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_NAME=

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <plugin-name> [args...]" >&2
  exit 1
fi

if [[ $# -gt 0 ]]; then
  PLUGIN_NAME="$1"
  shift
fi

PLUGIN_DIR="${SCRIPT_DIR}/${PLUGIN_NAME}"
if [ ! -d "${SCRIPT_DIR}/${PLUGIN_NAME}" ]; then
  echo "ERROR: Plugin '$PLUGIN_NAME' not found." >&2
  exit 1
fi

pushd "$PLUGIN_DIR" > /dev/null
go run . "$@"
popd > /dev/null