#!/bin/bash

set -euo pipefail

source vars.env

values_yaml="$STATE_DIR/teleport-values.yaml"

mkdir -p "$STATE_DIR"

cat > "$values_yaml" <<EOF
chartMode: aws
clusterName: ${CLUSTER_NAME}.${ROUTE53_ZONE}      # Name of your cluster. Use the FQDN you intend to configure in DNS below.
teleportVersionOverride: ${TELEPORT_VERSION}
proxyListenerMode: "multiplex"
authentication:
  type: local
  secondFactor: "webauthn"
  webauthn:
    rp_id: ${CLUSTER_NAME}.${ROUTE53_ZONE}
  connector_name: passwordless
  device_trust:
    mode: "off"
aws:
  region: ${REGION}                           # AWS region
  backendTable: ${CLUSTER_NAME}-backend           # DynamoDB table to use for the Teleport backend
  auditLogTable: ${CLUSTER_NAME}-events           # DynamoDB table to use for the Teleport audit log (must be different to the backend table)
  auditLogMirrorOnStdout: false                   # Whether to mirror audit log entries to stdout in JSON format (useful for external log collectors)
  sessionRecordingBucket: ${SESSION_BUCKET}       # S3 bucket to use for Teleport session recordings
  backups: true                                   # Whether or not to turn on DynamoDB backups
  dynamoAutoScaling: false                        # Whether Teleport should configure DynamoDB's autoscaling.
highAvailability:
  replicaCount: 2                                 # Number of replicas to configure
  certManager:
    enabled: true                                 # Enable cert-manager support to get TLS certificates
    issuerName: letsencrypt-production            # Name of the cert-manager Issuer to use (as configured above)
# If you are running Kubernetes 1.23 or above, disable PodSecurityPolicies
podSecurityPolicy:
  enabled: false
podMonitor:
  enabled: true
auth:
  teleportConfig:
    kubernetes_service:
      enabled: false
EOF
