#!/bin/bash
#
# Rebuilds a UI make target based on whether the given directories contents have changed
# minus any node-modules.
#

set -eo pipefail

ROOT_PATH="$(cd "$(dirname "$0")/.." && pwd -P)"
MAKE="${MAKE:-make}"
SHASUMS=("shasum -a 512" "sha512sum")

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

for i in "${!SRC_DIRECTORIES[@]}"; do
  SRC_DIRECTORIES[i]="$ROOT_PATH/${SRC_DIRECTORIES[i]}"
done

function calculate_sha() {
  #shellcheck disable=SC2005,SC2086
  echo "$(find "${SRC_DIRECTORIES[@]}" "$ROOT_PATH/package.json" "$ROOT_PATH/yarn.lock" -not \( -type d -name node_modules -prune \) -type f -print0 | LC_ALL=C sort -z | xargs -0 $SHASUM | awk '{print $1}' | $SHASUM | tr -d ' -')"  
}

# Calculate the current hash-of-hashes of the given source directories. Adds in package.json as well.
# This excludes node_modules, as the package.json differences should handle this.
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
  mkdir -p "$(dirname "$LAST_SHA_FILE")"
  # Save SHA with yarn.lock before yarn install
  echo $CURRENT_SHA > "$LAST_SHA_FILE"
  echo "$TYPE webassets successfully updated."
else
  echo "$TYPE webassets up to date."
fi
