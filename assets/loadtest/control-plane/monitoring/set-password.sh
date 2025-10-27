#!/bin/bash

set -euo pipefail

source vars.env

GRAFANA_POD=$(kubectl get pods --selector=app.kubernetes.io/name=grafana -n monitoring | tail -n 1 | awk '{print $1}')

if test -z "$GRAFANA_POD"; then
    echo "failed to discover grafana pod" >&2
    exit 1
fi

kubectl exec --stdin --namespace="monitoring" --tty --container grafana $GRAFANA_POD -- grafana cli admin reset-admin-password $GRAFANA_PASS
