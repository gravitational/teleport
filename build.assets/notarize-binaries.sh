#!/bin/bash
set -e

usage() {
  echo "Notarizes MacOS binaries with xcrun."
  echo "Usage: $(basename $0) [file ...]" 1>&2
  exit 1
}

# Don't follow sourced script.
#shellcheck disable=SC1090
#shellcheck disable=SC1091
. "$(dirname "$0")/build-common.sh"

# Verify arguments
if [ "$#" -eq 0 ]; then
    usage
fi

for BINARY in "$@"; do
    if [ ! -f "$BINARY" ]; then
        echo "$BINARY does not exist." 1>&2
        exit 2
    fi

    FILE_TYPE="$(file $BINARY)"
    if [ "$(echo $FILE_TYPE | grep -ic 'mach-o')" -eq 0 ]; then
        echo "$BINARY is not a MacOS binary (file is of type $FILE_TYPE)" 1>&2
        exit 2
    fi
done

ZIP_FILE="teleport.zip"

for BINARY in "$@"; do
    zip -ur "$ZIP_FILE" "$BINARY"
done

BUNDLE_ID="com.gravitational.teleport.zip"
echo "Notarizing $ZIP_FILE with team ID $TEAMID and bundle ID $BUNDLE_ID..."
notarize "$ZIP_FILE" "$TEAMID" "$BUNDLE_ID"
