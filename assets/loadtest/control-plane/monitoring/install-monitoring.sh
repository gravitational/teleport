#!/bin/bash

set -euo pipefail

source vars.env

monitor_yaml="$STATE_DIR/monitoring.yaml"

# source: examples/load-tests/values/kube-prometheus-stack.yaml
cat > "$monitor_yaml" <<EOF
prometheus:
  prometheusSpec:
    scrapeInterval: 15s
    retention: 30d
    resources:
      requests:
        memory: 16Gi
        cpu: "4"
      limits:
        memory: 16Gi
        # cpu: 4
    storageSpec:
      volumeClaimTemplate:
        spec:
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 50Gi
    podMonitorSelectorNilUsesHelmValues: false
    serviceMonitorSelectorNilUsesHelmValues: false
EOF

helm install monitoring \
    -n monitoring \
    --create-namespace prometheus-community/kube-prometheus-stack \
    -f "$monitor_yaml"
