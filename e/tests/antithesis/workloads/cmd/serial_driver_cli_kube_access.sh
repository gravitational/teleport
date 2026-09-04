#!/usr/bin/env bash
set -euo pipefail

# Smoke test: verify that the Teleport CLI can log in with the "allowed" bot
# identity and see the Kubernetes cluster registered by the agent's
# kubernetes_service.
#
#   tsh kube ls                   -> the cluster is registered and visible
#   tsh kube login $KUBE_CLUSTER  -> a Kubernetes certificate can be issued for it
#   kubectl get nodes             -> the API server is actually reachable through the proxy

CLUSTER="${KUBE_CLUSTER:-default}"
export TELEPORT_PROXY="${TELEPORT_PROXY:-antithesis.teleport.local:3080}"
# Use TELEPORT_IDENTITY_FILE here, `tsh kube login --identiy /foo` does not
# propagate the identity to `kubeconfig`
export TELEPORT_IDENTITY_FILE="${TELEPORT_IDENTITY:-/creds/allowed/identity}"

export SSL_CERT_FILE="${SSL_CERT_FILE:-/certs/rootCA.pem}"
export NO_COLOR=1

# Per-run home in case other tests need to write config
TELEPORT_HOME="$(mktemp -d /tmp/kube-access.XXXXXX)"
export TELEPORT_HOME
export KUBECONFIG="$TELEPORT_HOME/kubeconfig"
trap 'rm -rf "$TELEPORT_HOME"' EXIT

log() { printf '%s\n' "$*" >&2; }

attempt() {
  local out rc

  out="$(tsh kube ls --format=json 2>&1)"; rc=$?
  if [[ $rc -ne 0 ]]; then
    log "tsh kube ls failed (rc=$rc): $(tail -c 400 <<<"$out")"
    return 1
  fi
  # -e Sets the exit status of jq to 0 if the last output value was neither false nor null,
  # otherwise an error is set. We only look for a valid match.
  if ! jq -e --arg c "$CLUSTER" 'any(.[]; .kube_cluster_name == $c)' <<<"$out" >/dev/null; then
    log "cluster '${CLUSTER}' not listed by tsh kube ls:"
    log "$out"
    return 1
  fi
  log "tsh kube ls lists '${CLUSTER}'"

  out="$(tsh kube login "$CLUSTER" 2>&1)"; rc=$?
  if [[ $rc -ne 0 ]]; then
    log "tsh kube login ${CLUSTER} failed (rc=$rc): $(tail -c 400 <<<"$out")"
    return 1
  fi
  if [[ ! -s "$KUBECONFIG" ]]; then
    log "tsh kube login succeeded but wrote no kubeconfig at $KUBECONFIG"
    return 1
  fi
  log "tsh kube login ${CLUSTER} succeeded"

  # kubectl authenticates via the exec plugin tsh wrote into the kubeconfig,
  # which re-invokes `tsh kube credentials` against $TELEPORT_HOME.
  out="$(kubectl get nodes -o wide --request-timeout=30s 2>&1)"; rc=$?
  if [[ $rc -ne 0 ]]; then
    log "kubectl get nodes failed (rc=$rc): $(tail -c 400 <<<"$out")"
    return 1
  fi
  log "kubectl get nodes succeeded:"
  log "$out"

  return 0
}

for bin in tsh kubectl jq; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    log "FATAL: $bin is not installed in this image"
    exit 1
  fi
done

log "proxy=$TELEPORT_PROXY identity=$TELEPORT_IDENTITY_FILE cluster=$CLUSTER deadline=360s"

deadline=$(( $(date +%s) + 360 ))
n=0
while :; do
  n=$((n + 1))
  if attempt; then
    log "OK: Kubernetes access verified on attempt $n"
    exit 0
  fi
  [[ "$(date +%s)" -ge "$deadline" ]] && break
  sleep 2
done

log "FAIL: no Kubernetes access after $n attempts over 360s"
exit 1
