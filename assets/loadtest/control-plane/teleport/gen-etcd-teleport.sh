#!/bin/bash

set -euo pipefail

source vars.env

values_yaml="$STATE_DIR/teleport-values.yaml"

mkdir -p "$STATE_DIR"

cat > "$values_yaml" <<EOF
chartMode: standalone
clusterName: ${CLUSTER_NAME}.${ROUTE53_ZONE}      # Name of your cluster. Use the FQDN you intend to configure in DNS below.
teleportVersionOverride: ${TELEPORT_VERSION}

extraArgs: ['--debug']
image: "public.ecr.aws/gravitational-staging/teleport-distroless-debug"
enterpriseImage: "public.ecr.aws/gravitational-staging/teleport-ent-distroless-debug"

persistence:
    enabled: false

highAvailability:
  replicaCount: 2
  certManager:
    enabled: true
    issuerName: letsencrypt-production

authentication:
  type: local
  secondFactor: "webauthn"
  webauthn:
    rp_id: ${CLUSTER_NAME}.${ROUTE53_ZONE}
  connector_name: passwordless
  device_trust:
    mode: "off"
proxyListenerMode: "multiplex"
auth:
  teleportConfig:
    version: v3
    teleport:
      log:
        severity: DEBUG
        format:
          output: json
      storage:
        type: etcd
        peers: [http://etcd.etcd.svc.cluster.local:2379]
        insecure: true
        prefix: teleport
      connection_limits:
        max_connections: 65000
        max_users: 10000
      advertise_ip: teleport-auth
    auth_service:
      enabled: true
      cluster_name: ${CLUSTER_NAME}.${ROUTE53_ZONE}
      listen_addr: 0.0.0.0:3025
      routing_strategy: most_recent

proxy:
  annotations:
    service:
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
      service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
  teleportConfig:
    version: v3
    teleport:
      auth_server: "teleport-auth.teleport.svc.cluster.local:3025"
      log:
        severity: DEBUG
        format:
          output: json
      connection_limits:
        max_connections: 65000
        max_users: 1000
    proxy_service:
      enabled: true
      web_listen_addr: 0.0.0.0:3080
      public_addr: ${CLUSTER_NAME}.${ROUTE53_ZONE}:443
podSecurityPolicy:
  enabled: false
podMonitor:
  enabled: true
EOF
