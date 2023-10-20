#!/bin/bash
set -euo pipefail

kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: database-agents
  namespace: database-agents
spec:
  jobLabel: app
  namespaceSelector:
    matchNames:
      - database-agents
  selector:
    matchLabels:
      app: database-agents
  podMetricsEndpoints:
    - port: diag
      path: /metrics
EOF
