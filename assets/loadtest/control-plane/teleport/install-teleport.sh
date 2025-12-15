#!/bin/bash

set -euo pipefail

source vars.env

values_yaml="$STATE_DIR/teleport-values.yaml"

helm upgrade --install teleport teleport/teleport-cluster \
  --version "$TELEPORT_VERSION" \
  --create-namespace \
  --namespace teleport \
  -f "$values_yaml"
