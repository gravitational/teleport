#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

source vars.env

mkdir -p state

echo "attempting to build inventory..." >&2

tsh -i /opt/machine-id/identity --proxy "${PROXY_HOST:?}:${PROXY_PORT:?}" ls --format=json > state/inventory.json

jq -r '.[] | select(.metadata.expires > (now | strftime("%Y-%m-%dT%H:%M:%SZ"))) | .spec.hostname + ".scale-crdb.cloud.gravitational.io"' < state/inventory.json | sort -R > state/inventory

echo "successfully generated inventory node_count=$(cat state/inventory | wc -l)" >&2
