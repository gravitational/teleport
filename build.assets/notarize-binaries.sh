#!/bin/bash
set -e

usage() {
  echo "Notarizes MacOS binaries with xcrun."
  echo "Usage: $(basename $0) [file ...]" 1>&2
  exit 1
}

shell_array_to_json_array() {
    $shell_array=$1
    printf "["; IFS=, ; printf "${shell_array[*]}"; echo "]"
}

# This is largely pulled from `build-common.sh` but modified for this use case
sign_and_notarize_binaries() {
  local output_zip="$1"
  local bundle_id="$2"
  local targets="${@:3}"

  # XCode 12.
  local gondir=''
  gondir="$(mktemp -d)"
  # Early expansion on purpose.
  #shellcheck disable=SC2064
  trap "rm -fr '$gondir'" EXIT

  # Gon configuration file needs a proper extension.
  local goncfg="$gondir/gon.json"
  cat >"$goncfg" <<EOF
{
  "source": $(shell_array_to_json_array $targets),
  "sign": {
    "application_identity": "$DEVELOPER_ID_APPLICATION"
  },
  "zip": {
    "output_path": "$output_zip"
  },
  "notarize": [{
    "path": "$output_zip",
    "bundle_id": "$bundle_id",
    "staple": true
  }],
  "apple_id": {
    "username": "$APPLE_USERNAME",
    "password": "@env:APPLE_PASSWORD"
  }
}
EOF

  echo "gon configuration:"
  cat "$goncfg"
  ls -laht $target
  $DRY_RUN_PREFIX gon -log-level=debug "$goncfg"
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
BUNDLE_ID="com.gravitational.$ZIP_FILE"
echo "Notarizing $ZIP_FILE with bundle ID $BUNDLE_ID..."
sign_and_notarize_binaries "$ZIP_FILE" "$BUNDLE_ID" $@
