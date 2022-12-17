#!/bin/bash
set -e

usage() {
  echo "Notarizes MacOS binaries with xcrun."
  echo "Usage: $(basename $0) [file ...]" 1>&2
  exit 1
}

# Pulled from https://stackoverflow.com/a/26809278
shell_array_to_json_array() {
    echo -n '['
    while [ $# -gt 0 ]; do
        x=${1//\\/\\\\}
        echo -n \"${x//\"/\\\"}\"
        [ $# -gt 1 ] && echo -n ', '
        shift
    done
    echo ']'
}

# This is largely pulled from `build-common.sh` but modified for this use case
sign_and_notarize_binaries() {
  local bundle_id="$1"
  local targets=(${@:2})

  local notarization_zip="teleport.zip"

  local gondir=''
  gondir="$(mktemp -d)"
  #shellcheck disable=SC2064
  trap "rm -fr '$gondir'" EXIT

  # Gon configuration file needs a proper extension.
  local goncfg="$gondir/gon.json"
  # Note that xcrun stapler does not support stapling zip files.
  # Instead, Apple wants you to staple notarization tickets to binaries then
  # rebuild the zip file. That being said, Apple also does not support
  # binaries directly. Rather, they must be archived and then notarized.
  # For details, see
  # https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution/customizing_the_notarization_workflow
  cat >"$goncfg" <<EOF
{
  "source": $(shell_array_to_json_array $targets),
  "bundle_id": "$bundle_id",
  "sign": {
    "application_identity": "$DEVELOPER_ID_APPLICATION"
  },
  "zip": {
    "output_path": "$notarization_zip"
  },
  "notarize": [{
    "path": "$notarization_zip",
    "bundle_id": "$bundle_id",
    "staple": false
  }],
  "apple_id": {
    "username": "$APPLE_USERNAME",
    "password": "@env:APPLE_PASSWORD"
  }
}
EOF

  echo "gon configuration:"
  cat "$goncfg"
  
  # Workaround for https://github.com/mitchellh/gon/issues/43
  if ! output=$(gon -log-level=debug "$goncfg"); then
    if ! (echo "$output" | grep -qF "[$notarization_zip] File notarized!"); then
      # Look for a success message. If none was received, then the tool really did fail.
      # Log the failure.
      echo "Notarization failed. Output:"
      echo "$output"
      exit 1
    else
      echo "Notarization actually succeeded but logged an error."
    fi
  fi
  echo "Notarization output:"
  echo "$output"

  echo "Received notarization for binaries, stapling..."

  for BINARY in "$targets"; do
    echo "Stapling $BINARY..."
    xcrun stapler staple -v "$BINARY"
  done

  echo "Binary notarization complete"
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

BUNDLE_ID="com.gravitational.teleport"
echo "Notarizing '$@' with bundle ID $BUNDLE_ID..."
sign_and_notarize_binaries "$BUNDLE_ID" $@
