#!/bin/bash

set -euo pipefail

source vars.env

issuer_yaml="$STATE_DIR/aws-issuer.yaml"

mkdir -p "$STATE_DIR"

cat > "$issuer_yaml" <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-production
  namespace: teleport
spec:
  acme:
    email: ${EMAIL}
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-production
    solvers:
    - selector:
        dnsZones:
          - ${ROUTE53_ZONE}
      dns01:
        route53:
          region: ${REGION}
          hostedZoneID: ${ROUTE53_ZONE_ID}
EOF

# install cert-manager in the kube cluster
helm install cert-manager jetstack/cert-manager \
    --create-namespace \
    --namespace cert-manager \
    --set installCRDs=true \
    --set global.leaderElection.namespace=cert-manager \
    --set extraArgs="{--issuer-ambient-credentials}" # required to automount ambient AWS credentials when using an Issuer


# set up namespace
kubectl create namespace teleport
kubectl label namespace teleport 'pod-security.kubernetes.io/enforce=baseline'

# configure cert manager
kubectl --namespace teleport create -f "$issuer_yaml"

