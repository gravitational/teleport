#!/usr/bin/env bash
set -euo pipefail

# Run a pod and exec commands inside it through Teleport.

KUBE_WORKLOAD_HOME="$(mktemp -d)"
export KUBE_WORKLOAD_HOME

source "$(dirname "${BASH_SOURCE[0]}")/helper_kube_common.sh"

trap 'kube_cleanup_home' EXIT
kube_driver_init

POD="$(kube_unique_name kube-exec)"
WANT_RC=7

trap 'kube_delete_pod "$POD"; kube_cleanup_home' EXIT
kube_start_pod "$POD" || log_fatal "pod ${POD} never became ready"
log "pod ${POD} ready"

exec_stdout() {
  local nonce
  nonce="$(kube_nonce)"
  kctl exec "$POD" -- /bin/sh -c "echo ${nonce}" | grep -x -- "$nonce" >/dev/null
}

kube_retry exec_stdout

exec_stdin() {
  local nonce
  nonce="$(kube_nonce)"
  printf '%s\n' "$nonce" | kctl exec -i "$POD" -- /bin/cat | grep -x -- "$nonce" >/dev/null
}

kube_retry exec_stdin

exec_stderr() {
  local nonce
  nonce="$(kube_nonce)"
  kctl exec "$POD" -- /bin/sh -c "echo ${nonce} >&2" 2>&1 >/dev/null | grep -x -- "$nonce" >/dev/null
}

kube_retry exec_stderr

exec_exit_code() {
  local rc=0
  kctl exec "$POD" -- /bin/sh -c "exit ${WANT_RC}" >/dev/null 2>&1 || rc=$?
  if [[ $rc -ne $WANT_RC ]]; then
    log "expected exit code ${WANT_RC}, got ${rc}"
    return 1
  fi
  return 0
}

kube_retry exec_exit_code
