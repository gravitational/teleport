#!/bin/bash
#
# Rebuilds a UI make target based on whether the given directories contents have changed
# minus any node-modules.
#

set -eo pipefail

ROOT_PATH="$(cd "$(dirname "$0")/.." && pwd -P)"
MAKE="${MAKE:-make}"
SHASUMS=("shasum -a 512" "sha512sum")

function print_for_user() {
  echo "${0##*/}: $*"
}

if ! command -v "$MAKE" >/dev/null; then
  print_for_user "Unable to find \"$MAKE\" on path."
  exit 1
fi

if [ -n "$SHASUM" ]; then
  EXEC="$(echo "$SHASUM" | awk '{print $1}')"
  if ! command -v "$EXEC" >/dev/null; then
    print_for_user "Unable to find custom SHA sum $SHASUM on path."
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
  print_for_user "Unable to find a SHA sum executable."
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

# Calculate the current hash-of-hashes of the given source directories, package.json, and pnpm-lock.yaml.
# We exclude node_modules as it's covered by package.json and pnpm-lock.yaml.
# We also exclude .swc as it's a cache directory for the swc compiler,
# and ironrdp/pkg as it's filled with the generated wasm files.
function calculate_sha() {
  #shellcheck disable=SC2086
  #We want to split $SHASUM on spaces so we dont want it quoted.
  find "${SRC_DIRECTORIES[@]}" "$ROOT_PATH/package.json" "$ROOT_PATH/pnpm-lock.yaml" \
    -not \( -type d \( -name node_modules -o -name .swc -o -path '*ironrdp/pkg*' \) -prune \) \
    -type f -print0 | \
    LC_ALL=C sort -z | xargs -0 $SHASUM | awk '{print $1}' | $SHASUM | tr -d ' -'
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
  print_for_user "detected changes in $TYPE webassets. Rebuilding..."
  "$MAKE" -C "$ROOT_PATH" "$BUILD_TARGET"; \
  # Recalculate the current SHA and record into the LAST_SHA_FILE. The make target is expected to have
  # created any necessary directories here. The recalculation is necessary as pnpm-lock.yaml may have been
  # updated by the build process.
  mkdir -p "$(dirname "$LAST_SHA_FILE")"
  # Save SHA with pnpm-lock.yaml before installing dependencies.
  echo "$CURRENT_SHA" > "$LAST_SHA_FILE"
  print_for_user "$TYPE webassets successfully updated."
else
  print_for_user "$TYPE webassets up to date."
fi
