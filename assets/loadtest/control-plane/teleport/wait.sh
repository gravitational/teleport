#!/bin/bash

set -euo pipefail

source vars.env

component="${1}"

case "$component" in
    auth)
        ;;
    proxy)
        ;;
    *)
        echo "unknown component '$component'" >&2
        exit 1
        ;;
esac

kubectl wait pods \
    --namespace teleport \
    --timeout 10m \
    --selector "app.kubernetes.io/component=${component}" \
    --for "condition=Ready"
