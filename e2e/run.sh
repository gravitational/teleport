#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WITH_SSH_NODE=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-ssh-node) WITH_SSH_NODE=true; shift ;;
    *) shift ;;
  esac
done

# Allow overrides via environment for enterprise reuse
E2E_DIR="${E2E_DIR:-$SCRIPT_DIR}"
TELEPORT_BIN="${TELEPORT_BIN:-${REPO_ROOT}/build/teleport}"
CONFIG_FILE="${E2E_DIR}/config/teleport.yaml"
STATE_FILE="${E2E_DIR}/config/state.yaml"
CERTS_DIR="${E2E_DIR}/certs"
DATA_DIR="${E2E_DIR}/data"

# Build SAN list — include host.docker.internal when running the SSH node
SAN="DNS:localhost,DNS:teleport-e2e,IP:127.0.0.1"
if [ "$WITH_SSH_NODE" = true ]; then
  SAN="${SAN},DNS:host.docker.internal"
fi

# Generate self-signed TLS certs for the proxy
mkdir -p "$CERTS_DIR"
openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERTS_DIR/tls.key" \
  -out "$CERTS_DIR/tls.crt" \
  -days 1 -nodes \
  -subj "/CN=localhost" \
  -addext "subjectAltName=${SAN}" \
  2>/dev/null

# Clean and create data directory
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

# Rewrite config with generated cert paths and local data dir
sed \
  -e "s|/etc/teleport/certs/tls.key|${CERTS_DIR}/tls.key|" \
  -e "s|/etc/teleport/certs/tls.crt|${CERTS_DIR}/tls.crt|" \
  -e "s|/var/lib/teleport|${DATA_DIR}|" \
  ${EXTRA_SED_ARGS:-} \
  "$CONFIG_FILE" > "${E2E_DIR}/teleport-e2e.yaml"

cleanup_on_error() {
  echo "ERROR: run.sh failed, cleaning up..."
  if [ -n "${TELEPORT_PID:-}" ]; then
    kill "$TELEPORT_PID" 2>/dev/null || true
  fi
  docker stop teleport-e2e-node 2>/dev/null || true
}
trap cleanup_on_error ERR

echo "Starting Teleport with bootstrap state..."
if [ -n "${CI:-}" ]; then
  TELEPORT_LOG="${E2E_DIR}/teleport.log"
  "$TELEPORT_BIN" start -c "${E2E_DIR}/teleport-e2e.yaml" --bootstrap "$STATE_FILE" &>"$TELEPORT_LOG" &
else
  "$TELEPORT_BIN" start -c "${E2E_DIR}/teleport-e2e.yaml" --bootstrap "$STATE_FILE" &
fi
TELEPORT_PID=$!

echo "Waiting for Teleport to be ready..."
for _ in $(seq 1 30); do
  if curl -sf -o /dev/null -k https://localhost:3080/web/config.js 2>/dev/null; then
    echo "Teleport is ready"
    break
  fi
  sleep 1
done

if ! curl -sf -o /dev/null -k https://localhost:3080/web/config.js 2>/dev/null; then
  echo "ERROR: Teleport failed to start within 30 seconds"
  exit 1
fi

# Start Docker SSH node if requested
if [ "$WITH_SSH_NODE" = true ]; then
  TELEPORT_NODE_VERSION="${TELEPORT_NODE_VERSION:-18}"
  NODE_CONFIG="${SCRIPT_DIR}/config/node.yaml"
  NODE_CONTAINER="teleport-e2e-node"

  echo "Building Docker SSH node (teleport:${TELEPORT_NODE_VERSION})..."
  docker build \
    --build-arg "TELEPORT_VERSION=${TELEPORT_NODE_VERSION}" \
    -t teleport-e2e-node \
    -f "${SCRIPT_DIR}/Dockerfile.node" \
    "${SCRIPT_DIR}"

  echo "Starting Docker SSH node..."
  docker run -d --rm \
    --name "$NODE_CONTAINER" \
    -p 3022:3022 \
    -v "${NODE_CONFIG}:/etc/teleport/node.yaml:ro" \
    --add-host=host.docker.internal:host-gateway \
    teleport-e2e-node

  echo "Waiting for Docker node to join cluster..."
  for _ in $(seq 1 30); do
    if "${TCTL_BIN:-${REPO_ROOT}/build/tctl}" -c "${E2E_DIR}/teleport-e2e.yaml" nodes ls 2>/dev/null | grep -q docker-node; then
      echo "Docker SSH node is ready"
      break
    fi
    sleep 1
  done
fi
