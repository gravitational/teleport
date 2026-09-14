#!/usr/bin/env bash
set -euo pipefail

# Run a pod, port-forward to it, verify the forwarding works.

KUBE_WORKLOAD_HOME="$(mktemp -d)"
export KUBE_WORKLOAD_HOME

source "$(dirname "${BASH_SOURCE[0]}")/helper_kube_common.sh"

trap 'kube_cleanup_home' EXIT
kube_driver_init

POD="$(kube_unique_name kube-pf)"
PF_READY_TIMEOUT=30

PF_PID=""
PF_PORT=""

# `tsh kubectl` re-execs itself, so the process doing the actual
# port-forward is a child of the tsh we started.
pf_stop() {
  if [[ -n "$PF_PID" ]]; then
    kill -KILL -- "-$PF_PID" >/dev/null 2>&1 || true
    wait "$PF_PID" >/dev/null 2>&1 || true
    PF_PID=""
  fi
}


trap 'pf_stop; kube_delete_pod "$POD"; kube_cleanup_home' EXIT
kube_start_pod "$POD" || log_fatal "pod ${POD} never became ready"
log "pod ${POD} ready"

pf_start() {
  pf_stop
  PF_PORT="$(kube_free_port)" || return 1

  # Enable job control just for the launch so bash puts the job in a new process group whose PGID
  # equals PF_PID.
  #
  # From docs:
  #       -m      Monitor mode.  Job control is enabled.  This option is on by default for interactive shells on systems that support it (see JOB CONTROL above).
  # All processes run in a separate process group.  When a background job completes, the shell prints a line containing its exit status.

  set -m
  "${KUBE_CLIENT_CMD[@]}" \
    --namespace="$KUBE_NAMESPACE" \
    port-forward "pod/${POD}" "${PF_PORT}:80" --address=127.0.0.1 </dev/null 2>&1 &
  PF_PID=$!
  set +m

  local deadline=$(($(date +%s) + PF_READY_TIMEOUT))
  while :; do
    if ! kill -0 "$PF_PID" >/dev/null 2>&1; then
      log "port-forward exited early"
      pf_stop
      return 1
    fi

    if kube_port_listening "$PF_PORT"; then
      log "forwarding 127.0.0.1:${PF_PORT} -> ${POD}:80"
      return 0
    fi

    if [[ "$(date +%s)" -ge "$deadline" ]]; then
      log "nothing listening on 127.0.0.1:${PF_PORT} within ${PF_READY_TIMEOUT}s"
      pf_stop
      return 1
    fi
    sleep 1
  done
}


pf_serves_index() {
  pf_start || return 1
  curl --silent --show-error --fail --max-time 10 "http://127.0.0.1:${PF_PORT}/" \
    | grep -i 'nginx' >/dev/null
}

kube_retry pf_serves_index
