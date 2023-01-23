#!/bin/bash
#
# Rebuilds a UI make target based on whether the given directories contents have changed
# minus any node-modules.
#

set -eo pipefail

ROOT_PATH="$(realpath "$(dirname "$0")/..")"
MAKE="${MAKE:-make}"
SHASUMS=("sha256sum" "sha512sum" "shasum -a 256")

if ! command -v "$MAKE" >/dev/null; then
  echo "Unable to find \"$MAKE\" on path."
  exit 1
fi

if [ -n "$SHASUM" ]; then
  EXEC="$(echo "$SHASUM" | awk '{print $1}')"
  if ! command -v "$EXEC" >/dev/null; then
    echo "Unable to find custom SHA sum $SHASUM on path."
    exit 1
  fi
else
  for shasum in "${SHASUMS[@]}"; do
    EXEC="$(echo "$shasum" | awk '{print $1}')"
    if command -v "$EXEC" >/dev/null; then
      SHASUM="$shasum"
      break
    fi
  done
fi

if [ -z "$SHASUM" ]; then
  echo "Unable to find a SHA sum executable."
  exit 1
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

# Calculate the current hash-of-hashes of the given source directories. Adds in yarn.lock and package.json as well.
# This excludes node_modules, as the yarn.lock/package.json differences should handle this.
#shellcheck disable=SC2086
CURRENT_SHA="$( (for dir in "${SRC_DIRECTORIES[@]}"; do
  find "$ROOT_PATH/$dir" -type f
done && echo "$ROOT_PATH/yarn.lock" && echo "$ROOT_PATH/package.json") | 
  grep -v "node_modules" | xargs -I{} -n1 -P8 $SHASUM {} | sort -k 2 | $SHASUM |
  cut -f1 -d' ')"

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
  # Record the current SHA into the LAST_SHA_FILE. The make target is expected to have
  # created any necessary directories here.
  echo "$CURRENT_SHA" > "$LAST_SHA_FILE"
  echo "$TYPE webassets successfully updated."
else
  echo "$TYPE webassets up to date."
fi
