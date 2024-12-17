#!/bin/bash

set -euo pipefail

source vars.env

values_yaml="$STATE_DIR/teleport-values.yaml"

mkdir -p "$STATE_DIR"

cat > "$values_yaml" <<EOF
chartMode: gcp
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
gcp:
  projectId: ${GCP_PROJECT}
  region: ${REGION}                           # AWS region
  backendTable: ${CLUSTER_NAME}-backend           # Firestore table to use for the Teleport backend
  auditLogTable: ${CLUSTER_NAME}-events           # Firestore table to use for the Teleport audit log (must be different to the backend table)
  auditLogMirrorOnStdout: false                   # Whether to mirror audit log entries to stdout in JSON format (useful for external log collectors)
  sessionRecordingBucket: ${FIRESTORE_SESSION_BUCKET}       # Storage bucket to use for Teleport session recordings
  backups: true                                   # Whether or not to turn on DynamoDB backups
  dynamoAutoScaling: false                        # Whether Teleport should configure DynamoDB's autoscaling.
highAvailability:
  replicaCount: 2                                 # Number of replicas to configure
  certManager:
    enabled: false                                # No certManager because we only run simulated load tests and don't need to connect any agents
# If you are running Kubernetes 1.23 or above, disable PodSecurityPolicies
podSecurityPolicy:
  enabled: false
podMonitor:
  enabled: true
extraArgs:
  - --debug
image: "public.ecr.aws/gravitational-staging/teleport-distroless-debug"
enterpriseImage: "public.ecr.aws/gravitational-staging/teleport-ent-distroless-debug"
auth:
  teleportConfig:
    kubernetes_service:
      enabled: false
EOF
