#!/bin/bash
# Copyright 2024 Gravitational, Inc

# This script fetches and runs the latest Teleport install script from the release CDN.

# The script is wrapped inside a function to protect against the connection being interrupted
# in the middle of the stream.

# For more download options, head to https://goteleport.com/download/
set -euo pipefail

# download uses curl or wget to download a teleport binary
download() {
    URL=$1
    TMP_PATH=$2

    echo "Downloading $URL"
    if type curl &>/dev/null; then
        set -x
        # shellcheck disable=SC2086
        $CURL -o "$TMP_PATH" "$URL"
    else
        set -x
        # shellcheck disable=SC2086
        $CURL -O "$TMP_PATH" "$URL"
    fi
    set +x
}

# get_version returns either the requested version, if available, or a default (16.2.0)
# otherwise
get_version() {
    REQUESTED=$1
    MIN=16.1.8 # minimum install script version

    IFS='.' read -r -a requested_parts <<<"$REQUESTED"
    IFS='.' read -r -a min_parts <<<"$MIN"

    # Compare each part
    for i in {0..2}; do
        if ((requested_parts[i] < min_parts[i])); then
            echo "$MIN"
            return
        elif ((requested_parts[i] > min_parts[i])); then
            echo "$REQUESTED"
            return
        fi
    done

    # If all parts are equal
    echo "$REQUESTED"
}

fetch_and_run() {
    # require curl/wget
    CURL=""
    if type curl &>/dev/null; then
        CURL="curl -fL"
    elif type wget &>/dev/null; then
        CURL="wget"
    fi
    if [ -z "$CURL" ]; then
        echo "ERROR: This script requires either curl or wget in order to download files. Please install one of them and try again."
        exit 1
    fi

    # require shasum/sha256sum
    SHA_COMMAND=""
    if type shasum &>/dev/null; then
        SHA_COMMAND="shasum -a 256"
    elif type sha256sum &>/dev/null; then
        SHA_COMMAND="sha256sum"
    else
        echo "ERROR: This script requires sha256sum or shasum to validate the download. Please install it and try again."
        exit 1
    fi

    SCRIPT_VERSION=$(get_version "$TELEPORT_VERSION")

    # fetch install script
    TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)
    SCRIPT_FILENAME="install-v$SCRIPT_VERSION.sh"
    SCRIPT_PATH="${TEMP_DIR}/${SCRIPT_FILENAME}"
    URL="https://cdn.teleport.dev/${SCRIPT_FILENAME}"
    download "${URL}" "${SCRIPT_PATH}"

    # verify checksum
    TMP_CHECKSUM="${SCRIPT_PATH}.sha256"
    download "${URL}.sha256" "$TMP_CHECKSUM"

    set -x
    cd "$TEMP_DIR"
    $SHA_COMMAND -c "$TMP_CHECKSUM"
    cd -
    set +x

    # run install script
    bash "${SCRIPT_PATH}" "$TELEPORT_VERSION" "$TELEPORT_EDITION"
}

TELEPORT_VERSION=""
TELEPORT_EDITION=""
if [ $# -ge 1 ] && [ -n "$1" ]; then
    TELEPORT_VERSION=$1
else
    echo "ERROR: Please provide the version you want to install (e.g., 16.2.0)."
    exit 1
fi

if ! echo "$1" | grep -qE "[0-9]+\.[0-9]+\.[0-9]+"; then
    echo "ERROR: The first parameter must be a version number, e.g., 16.2.0."
    exit 1
fi

if [ $# -ge 2 ] && [ -n "$2" ]; then
    TELEPORT_EDITION=$2

    if [ "$TELEPORT_EDITION" != "oss" ] && [ "$TELEPORT_EDITION" != "enterprise" ] && [ "$TELEPORT_EDITION" != "cloud" ]; then
        echo 'ERROR: The second parameter must be "oss", "cloud", or "enterprise".'
        exit 1
    fi
fi
fetch_and_run
