#!/usr/bin/env bash
set -euo pipefail

# Run a pod and read its logs.

KUBE_WORKLOAD_HOME="$(mktemp -d)"
export KUBE_WORKLOAD_HOME

source "$(dirname "${BASH_SOURCE[0]}")/helper_kube_common.sh"

trap 'kube_cleanup_home' EXIT
kube_driver_init

POD="$(kube_unique_name kube-logs)"
MARKER="antithesis-log-marker-$(kube_nonce)"

trap 'kube_delete_pod "$POD"; kube_cleanup_home' EXIT
kube_start_pod "$POD" "$MARKER" || log_fatal "pod ${POD} never became ready"
log "pod ${POD} ready"

logs_contain_marker() {
  local hits
  hits="$(kctl logs "$POD" | grep -c -F -- "$MARKER")" || hits=0
  if [[ "$hits" -ne 1 ]]; then
    log "expected marker '${MARKER}' exactly once in ${POD} logs, found ${hits}"
    return 1
  fi
}

kube_retry logs_contain_marker
