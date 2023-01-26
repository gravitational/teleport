#!/bin/bash
#
# Rebuilds a UI make target based on whether the given directories contents have changed
# minus any node-modules.
#

set -eo pipefail

ROOT_PATH="$(realpath "$(dirname "$0")/..")"
MAKE="${MAKE:-make}"
PYTHON="${PYTHON:-python3}"

if ! command -v "$MAKE" >/dev/null; then
  echo "Unable to find \"$MAKE\" on path."
  exit 1
fi

if ! command -v "$PYTHON" >/dev/null; then
  echo "Warning: Unable to find \"$PYTHON\" on path, unable to calculate the SHA for the build."
  echo "         webassets will be rebuilt for every build until a valid Python 3 executable is available."
  PYTHON=""
fi

if [ "$#" -lt 4 ]; then
  echo "Usage: $0 <type> <last-sha-file> <build-target> <directories...>"
  exit 1
fi

TYPE="$1"
LAST_SHA_FILE="$ROOT_PATH/$2"
BUILD_TARGET="$3"
shift 3
SRC_DIRECTORIES=("$@")

function calculate_sha() {
  if [ -z "$PYTHON" ]; then
    echo ""
  else
    "$PYTHON" "$ROOT_PATH/build.assets/shacalc.py" "${SRC_DIRECTORIES[@]}"
  fi
}

CURRENT_SHA="$(calculate_sha)"

BUILD=true

# If the LAST_SHA_FILE exists, test whether it's equivalent to the current calculated SHA. If it is,
# set BUILD to false.
if [ -f "$LAST_SHA_FILE" ]; then
  LAST_SHA="$(cat "$LAST_SHA_FILE")"
  if [ "$LAST_SHA" = "$CURRENT_SHA" ]; then
    BUILD=false
  fi
fi

# If BUILD is true, make the build target. This assumes using the root Makefile.
if [ "$BUILD" = "true" ]; then \
  "$MAKE" -C "$ROOT_PATH" "$BUILD_TARGET"; \
  # Recalculate the current SHA and record into the LAST_SHA_FILE. The make target is expected to have
  # created any necessary directories here. The recalculation is necessary as yarn.lock may have been
  # updated by the build process.
  UPDATED_SHA="$(calculate_sha)"

  if [ -n "$UPDATED_SHA" ]; then
    echo "$UPDATED_SHA" > "$LAST_SHA_FILE"
  fi
  echo "$TYPE webassets successfully updated."
else
  echo "$TYPE webassets up to date."
fi
