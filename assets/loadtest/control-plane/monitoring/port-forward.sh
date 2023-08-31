#!/bin/bash

set -euo pipefail

localport="6060"

echo "port forwarding grafana on $localport..."

kubectl --namespace monitoring port-forward svc/monitoring-grafana "${localport}:80"
