#!/usr/bin/env bash
set -euo pipefail

identity_file="${TELEPORT_IDENTITY_FILE:-/opt/machine-id/identity}"
proxy_addr="${TELEPORT_PROXY:-antithesis.teleport.local:3080}"
tsh_home="$(mktemp -d "${TMPDIR:-/tmp}/tsh-home.XXXXXX")"

cleanup() {
    rm -rf "$tsh_home"
}
trap cleanup EXIT

if [[ ! -f "$identity_file" ]]; then
    echo "identity file not found: $identity_file" >&2
    exit 1
fi

TELEPORT_HOME="$tsh_home" /usr/local/bin/tsh \
    --proxy="$proxy_addr" \
    --identity="$identity_file" \
    ls
