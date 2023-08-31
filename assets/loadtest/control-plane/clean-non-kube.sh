#!/bin/bash

set -euo pipefail

# clean up the resources not automatically destroyed by bringing down the kube cluster.

log_info() {
    echo "[i] $* [ $(caller | awk '{print $1}') ]" >&2
}

log_info "detaching iam policies..."

./policies/attach-policies.sh detach

log_info "deleting iam policies..."

./policies/delete-policies.sh

log_info "destroying cluster storage..."

./storage/delete-storage.sh

log_info "deleting dns records..."

./dns/update-record.sh DELETE
