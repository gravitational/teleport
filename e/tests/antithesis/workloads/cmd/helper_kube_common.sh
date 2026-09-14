#!/usr/bin/env bash
export TELEPORT_PROXY="${TELEPORT_PROXY:-antithesis.teleport.local:3080}"
# TELEPORT_IDENTITY_FILE rather than tsh --identity, which does not propagate the identity to the exec plugin.
export TELEPORT_IDENTITY_FILE="${TELEPORT_IDENTITY_FILE:-${TELEPORT_IDENTITY:-/creds/allowed/identity}}"
export SSL_CERT_FILE="${SSL_CERT_FILE:-/certs/rootCA.pem}"
export NO_COLOR=1

KUBE_CLUSTER="${KUBE_CLUSTER:-default}"
KUBE_NAMESPACE="${KUBE_NAMESPACE:-antithesis}"
KUBE_POD_IMAGE="${KUBE_POD_IMAGE:-nginx:latest}"

# Use a label for a selector to avoid deleting anything else in the namespace we have not created.
KUBE_POD_LABEL_KEY="antithesis.teleport.local/workload"
KUBE_POD_LABEL_VALUE="kube"

: "${KUBE_WORKLOAD_HOME:?must be set}"
export TELEPORT_HOME="$KUBE_WORKLOAD_HOME"
export KUBECONFIG="$KUBE_WORKLOAD_HOME/kubeconfig"

KUBE_DEADLINE="${KUBE_DEADLINE:-360}"

# Applied to every kctl call. Kept well under KUBE_DEADLINE so a request stalled by an injected fault is
# retried rather than eating the whole budget. port-forward deliberately bypasses kctl, since its connection
# is long lived.
KUBE_REQUEST_TIMEOUT=30s

log() { printf '%s\n' "$*" >&2; }

log_fatal() {
  log "FATAL: $*"
  exit 1
}

kube_nonce() {
  printf '%08x' "$SRANDOM"
}

kube_unique_name() {
  printf '%s-%s\n' "$1" "$(kube_nonce)"
}

kube_port_listening() {
  (exec 3<>"/dev/tcp/127.0.0.1/$1") >/dev/null 2>&1
}

kube_free_port() {
  local port i
  # Dynamic range 49152-65535
  for ((i = 0; i < 50; i++)); do
    port=$((49152 + SRANDOM % 16384))
    if ! kube_port_listening "$port"; then
      printf '%s\n' "$port"
      return 0
    fi
  done

  return 1
}

# Pick randomly between `tsh kubectl` and `kubectl` per invocation so the serial drivers exercise both paths.
# Alternatively if KUBECTL_BIN is set, use that as the client command.
kube_select_client() {
  if [[ -n "${KUBECTL_BIN:-}" ]]; then
    read -r -a KUBE_CLIENT_CMD <<<"$KUBECTL_BIN"
  elif (( SRANDOM % 2 == 0 )); then
    KUBE_CLIENT_CMD=(tsh kubectl)
  else
    KUBE_CLIENT_CMD=(kubectl)
  fi
  KUBE_CLIENT_NAME="${KUBE_CLIENT_CMD[*]}"
  export KUBE_CLIENT_NAME
}

kctl() {
  "${KUBE_CLIENT_CMD[@]}" --namespace="$KUBE_NAMESPACE" --request-timeout="$KUBE_REQUEST_TIMEOUT" "$@"
}

kctl_cluster() {
  "${KUBE_CLIENT_CMD[@]}" --request-timeout="$KUBE_REQUEST_TIMEOUT" "$@"
}

# kube_retry <cmd> [args...]
kube_retry() {
  local deadline=$(($(date +%s) + KUBE_DEADLINE))
  local attempt=0 rc=0

  while :; do
    attempt=$((attempt + 1))
    rc=0
    "$@" || rc=$?
    if (( rc == 0 )); then
      return 0
    fi

    if [[ "$(date +%s)" -ge "$deadline" ]]; then
      log_fatal "Timed out waiting for: ${*}"
    fi
    sleep 2
  done
}

# jq -e exits non-zero when the last value was false or null
kube_cluster_listed() {
  tsh kube ls --format=json \
    | jq -e --arg c "$KUBE_CLUSTER" 'any(.[]; .kube_cluster_name == $c)' >/dev/null
}

kube_login() {
  kube_cluster_listed || return 1
  tsh kube login "$KUBE_CLUSTER" >/dev/null || return 1

  if [[ ! -s "$KUBECONFIG" ]]; then
    log "tsh kube login succeeded but wrote no kubeconfig at ${KUBECONFIG}"
    return 1
  fi
}

kube_api_reachable() {
  kctl_cluster get --raw /version | jq -e '.gitVersion // empty' >/dev/null
}

kube_ensure_login() {
  mkdir -p "$KUBE_WORKLOAD_HOME"

  if [[ ! -s "$KUBECONFIG" ]]; then
    log "no kubeconfig at ${KUBECONFIG}, logging in"
    kube_retry kube_login
  fi

  if kube_api_reachable; then
    return 0
  fi

  log "API unreachable, retry with a new login"
  kube_retry kube_login
  kube_retry kube_api_reachable
}

kube_ensure_namespace() {
  [[ "$KUBE_NAMESPACE" == "default" ]] && return 0
  kctl_cluster get namespace "$KUBE_NAMESPACE" >/dev/null 2>&1 && return 0
  kctl_cluster create namespace "$KUBE_NAMESPACE" >/dev/null 2>&1 ||
    kctl_cluster get namespace "$KUBE_NAMESPACE" >/dev/null
}

# kube_pod_manifest <name> [log_marker]
kube_pod_manifest() {
  local name="$1" marker="${2:-}"

  cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${name}
  namespace: ${KUBE_NAMESPACE}
  labels:
    ${KUBE_POD_LABEL_KEY}: ${KUBE_POD_LABEL_VALUE}
spec:
  restartPolicy: Never
  terminationGracePeriodSeconds: 1
  containers:
    - name: nginx
      image: ${KUBE_POD_IMAGE}
      imagePullPolicy: Never
      ports:
        - name: http
          containerPort: 80
EOF

  if [[ -n "$marker" ]]; then
    cat <<EOF
      command: ["/bin/sh", "-c"]
      args:
        - |
          echo "${marker}"
          exec /docker-entrypoint.sh nginx -g 'daemon off;'
EOF
  fi

  cat <<EOF
      readinessProbe:
        tcpSocket:
          port: 80
        initialDelaySeconds: 1
        periodSeconds: 2
        failureThreshold: 30
EOF
}

kube_create_pod() {
  kube_pod_manifest "$1" "${2:-}" | kctl apply -f - >/dev/null
}

kube_pod_ready() {
  kctl wait --for=condition=Ready "pod/$1" --timeout="$KUBE_REQUEST_TIMEOUT" >/dev/null
}

kube_describe_pod() {
  kctl describe "pod/$1" 2>&1 | tail -c 2000 >&2 || true
}

kube_start_pod() {
  local name="$1" marker="${2:-}"

  kube_retry kube_create_pod "$name" "$marker"

  if ! kube_retry kube_pod_ready "$name"; then
    #  Try our best to emit some diagnostics for debug.
    kube_describe_pod "$name"
    return 1
  fi
}

kube_delete_pod() {
  kctl delete "pod/$1" --ignore-not-found --wait=false --grace-period=1 >/dev/null 2>&1 || true
}

kube_preflight() {
  local bin
  for bin in tsh kubectl jq; do
    command -v "$bin" >/dev/null 2>&1 || log_fatal "$bin is not installed in this image"
  done

  if [[ ! -s "$TELEPORT_IDENTITY_FILE" ]]; then
    log_fatal "identity file not found or empty: ${TELEPORT_IDENTITY_FILE}"
  fi
}

kube_cleanup_home() {
  rm -rf "$KUBE_WORKLOAD_HOME"
}

kube_driver_init() {
  log "home=${KUBE_WORKLOAD_HOME} identity=${TELEPORT_IDENTITY_FILE}"
  kube_preflight
  kube_select_client
  kube_ensure_login
  kube_retry kube_ensure_namespace
}
