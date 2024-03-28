#!/bin/bash

set -euo pipefail

# set up cluster resources (kube cluster must exist, aws and kubectl must be authenticated,
# and helm repos must be up to date).

source vars.env

log_info() {
    echo "[i] $* [ $(caller | awk '{print $1}') ]" >&2
}


case "$TELEPORT_BACKEND" in
    dynamo)
        ;;
    etcd)
        ;;
    firestore)
        ;;
    *)
        echo "invalid teleport backend '$TELEPORT_BACKEND', expected one of 'dynamo' or 'etcd'" >&2
        exit 1
        ;;
esac

log_info "generating iam policies..."

./policies/gen-policies.sh

log_info "creating iam policies..."

./policies/create-policies.sh

log_info "attaching iam policies..."

./policies/attach-policies.sh attach

log_info "installing monitoring stack..."

./monitoring/install-monitoring.sh

if [[ "$TELEPORT_BACKEND" != "firestore" ]]; then
  log_info "setting up cert-manager..."

  ./dns/init-cert-manager.sh
fi

case "$TELEPORT_BACKEND" in
    dynamo)
        log_info "generating helm values for dynamo-backed control plane..."
        ./teleport/gen-dynamo-teleport.sh
        ;;
    etcd)
        log_info "installing etcd..."
        make -C ../etcd deploy

        log_info "generating helm values for etcd-backed control plane..."
        ./teleport/gen-etcd-teleport.sh
        ;;
    firestore)
        log_info "generating helm values for firestore-backed control plane..."

        kubectl create namespace teleport
        kubectl label namespace teleport 'pod-security.kubernetes.io/enforce=baseline'
        kubectl --namespace teleport create secret generic teleport-gcp-credentials --from-file=gcp-credentials.json="$GCP_CREDENTIALS"

        ./teleport/gen-firestore-teleport.sh
        ;;
    *)
        echo "invalid teleport backend '$TELEPORT_BACKEND', expected one of 'dynamo' or 'etcd'" >&2
        exit 1
        ;;
esac

log_info "installing control plane chart..."

./teleport/install-teleport.sh

log_info "waiting for auths to report ready..."

./teleport/wait.sh auth

if [[ "$TELEPORT_BACKEND" != "firestore" ]]; then
  log_info "setting up dns record..."

  ./dns/update-record.sh UPSERT # CREATE|UPSERT|DELETE
fi

log_info "waiting for proxies to report ready..."

./teleport/wait.sh proxy


if [[ "$TELEPORT_BACKEND" == "dynamo" ]]; then
    log_info "switching dynamo to on-demand mode..."

    ./storage/set-on-demand.sh
fi

log_info "setting grafana admin password..."

./monitoring/set-password.sh
