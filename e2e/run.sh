#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Allow overrides via environment for enterprise reuse
E2E_DIR="${E2E_DIR:-$SCRIPT_DIR}"
TELEPORT_BIN="${TELEPORT_BIN:-${REPO_ROOT}/build/teleport}"
CONFIG_FILE="${E2E_DIR}/config/teleport.yaml"
STATE_FILE="${E2E_DIR}/config/state.yaml"
CERTS_DIR="${E2E_DIR}/certs"
DATA_DIR="${E2E_DIR}/data"

# Generate self-signed TLS certs for the proxy
mkdir -p "$CERTS_DIR"
openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERTS_DIR/tls.key" \
  -out "$CERTS_DIR/tls.crt" \
  -days 1 -nodes \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,DNS:teleport-e2e,IP:127.0.0.1"

# Create data directory
mkdir -p "$DATA_DIR"

# Rewrite config with generated cert paths and local data dir
sed \
  -e "s|/etc/teleport/certs/tls.key|${CERTS_DIR}/tls.key|" \
  -e "s|/etc/teleport/certs/tls.crt|${CERTS_DIR}/tls.crt|" \
  -e "s|/var/lib/teleport|${DATA_DIR}|" \
  ${EXTRA_SED_ARGS:-} \
  "$CONFIG_FILE" > "${E2E_DIR}/teleport-e2e.yaml"

echo "Starting Teleport with bootstrap state..."
"$TELEPORT_BIN" start -c "${E2E_DIR}/teleport-e2e.yaml" --bootstrap "$STATE_FILE" &

echo "Waiting for Teleport to be ready..."
for i in $(seq 1 30); do
  if curl -sf -o /dev/null -k https://localhost:3080/web/config.js 2>/dev/null; then
    echo "Teleport is ready"
    exit 0
  fi
  sleep 1
done

echo "ERROR: Teleport failed to start within 30 seconds"
exit 1
