#!/usr/bin/env bash
# up.sh — bring up an ephemeral Teleport cluster with the given component versions.
# Prints progress to stderr; prints the unique project name as the LAST line of stdout.
# Exit non-zero on bring-up failure (still prints the project name so caller can tear down).
#
# Concurrency-safe: multiple invocations may run in parallel. Each cluster gets:
#   - uniquely-named containers/volumes/network (via COMPOSE_PROJECT_NAME),
#   - its own docker network on a distinct 10.99.<slot>.0/24 subnet,
#   - its own mock-oidc users file (mock-oidc-users-<project>.json).
# The chosen subnet/users-file are persisted to state/<project>.env for sso-login.sh
# and down.sh to read back.
#
# Usage: up.sh <auth-version> <proxy-version> <node-version> [subnet-slot]
#   subnet-slot: optional 1..254 forcing the network's second octet (10.99.<slot>.0/24).
#                Omit to auto-pick a free slot. Overlap with a concurrent cluster is
#                detected and retried automatically, so an explicit slot is never required.

set -euo pipefail

AUTH_VERSION="${1:?usage: up.sh <auth-version> <proxy-version> <node-version> [slot]}"
PROXY_VERSION="${2:?usage: up.sh <auth-version> <proxy-version> <node-version> [slot]}"
NODE_VERSION="${3:?usage: up.sh <auth-version> <proxy-version> <node-version> [slot]}"
FORCE_SLOT="${4:-}"

cd "$(dirname "$0")"

PROJECT="regression-$(date +%s)-$$-${RANDOM}"
mkdir -p state

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$(cd ../.. && pwd)")"
LICENSE_SRC="$REPO_ROOT/e/fixtures/license-eub.pem"
if [[ ! -f "$LICENSE_SRC" ]]; then
  echo "License file not found: $LICENSE_SRC" >&2
  exit 1
fi
# Shared, read-only, identical content across runs — copy once if absent.
[[ -f license-eub.pem ]] || cp "$LICENSE_SRC" license-eub.pem

# Build the mock OIDC image if it's not in the local docker cache.
if ! docker image inspect regression-mock-oidc:latest >/dev/null 2>&1; then
  echo "Building regression-mock-oidc image (first run only)..." >&2
  docker build -q -t regression-mock-oidc:latest mock-oidc/ >&2
fi

# Per-project mock-oidc users file. setup-user.sh appends to this same path; the
# file must exist before `compose up` because it's bind-mounted into mock-oidc.
OIDC_USERS_FILE="./mock-oidc-users-${PROJECT}.json"
echo '{}' > "$OIDC_USERS_FILE"

export COMPOSE_PROJECT_NAME="$PROJECT"
export AUTH_VERSION PROXY_VERSION NODE_VERSION OIDC_USERS_FILE

# Pre-pull images for all three versions so docker compose up doesn't race the
# registration-wait loop. Pulls are idempotent and skipped if cached.
ENT="public.ecr.aws/gravitational/teleport-ent-distroless"
ENT_DEBUG="public.ecr.aws/gravitational/teleport-ent-distroless-debug"
for ref in "${ENT}:${AUTH_VERSION}" "${ENT}:${PROXY_VERSION}" "${ENT_DEBUG}:${NODE_VERSION}"; do
  echo "Pulling $ref..." >&2
  if ! docker pull --quiet "$ref" >&2; then
    echo "Failed to pull $ref — verify the tag exists at https://gallery.ecr.aws/gravitational/" >&2
    echo "$PROJECT"
    exit 1
  fi
done

# Second octets of all 10.99.x.0/24 subnets currently owned by docker networks.
list_used_octets() {
  local net sub
  for net in $(docker network ls --format '{{.Name}}' 2>/dev/null); do
    sub=$(docker network inspect -f '{{range .IPAM.Config}}{{println .Subnet}}{{end}}' "$net" 2>/dev/null || true)
    echo "$sub" | grep -oE '^10\.99\.[0-9]+\.0/24$' | sed -E 's#^10\.99\.([0-9]+)\.0/24$#\1#' || true
  done
}

# Pick a free subnet and bring the cluster up. Two parallel runs can momentarily
# pick the same free octet (TOCTOU); docker rejects the overlap, so we detect that
# and retry with a freshly-scanned free octet.
UPOUT="$(mktemp)"
SLOT=""
started=0
for attempt in $(seq 1 12); do
  USED=" $(list_used_octets | tr '\n' ' ') "
  if [[ -n "$FORCE_SLOT" && $attempt -eq 1 ]]; then
    SLOT="$FORCE_SLOT"
  else
    SLOT=""
    start=$(( (RANDOM % 254) + 1 ))
    for k in $(seq 0 253); do
      cand=$(( ((start + k - 1) % 254) + 1 ))
      if [[ "$USED" != *" $cand "* ]]; then SLOT=$cand; break; fi
    done
  fi
  if [[ -z "$SLOT" ]]; then
    echo "No free 10.99.x.0/24 subnet available (too many clusters running?)" >&2
    echo "$PROJECT"; exit 1
  fi
  CLUSTER_SUBNET="10.99.${SLOT}.0/24"
  export CLUSTER_SUBNET
  echo "Attempt $attempt: bringing up project=$PROJECT subnet=$CLUSTER_SUBNET (auth=$AUTH_VERSION, proxy=$PROXY_VERSION, node=$NODE_VERSION)..." >&2
  if docker compose -f compose.tmpl.yml up -d >"$UPOUT" 2>&1; then
    cat "$UPOUT" >&2
    started=1; break
  fi
  cat "$UPOUT" >&2
  if grep -qiE 'overlap|pool overlaps|already in use' "$UPOUT"; then
    echo "Subnet $CLUSTER_SUBNET clashes with a concurrent cluster; cleaning up and retrying..." >&2
    docker compose -f compose.tmpl.yml down -v --remove-orphans >/dev/null 2>&1 || true
    FORCE_SLOT=""   # auto-pick on every retry
    continue
  fi
  echo "docker compose up failed (non-subnet error)" >&2
  rm -f "$UPOUT"
  echo "$PROJECT"; exit 1
done
rm -f "$UPOUT"

if [[ $started -ne 1 ]]; then
  echo "Failed to bring up cluster after retries (subnet contention)" >&2
  echo "$PROJECT"; exit 1
fi

# Persist chosen subnet/users-file so sso-login.sh and down.sh can recover them.
cat > "state/${PROJECT}.env" <<EOF
SLOT=${SLOT}
CLUSTER_SUBNET=${CLUSTER_SUBNET}
OIDC_USERS_FILE=${OIDC_USERS_FILE}
EOF

echo "Waiting for proxy and node to register with auth..." >&2
proxies=0
nodes=0
for i in $(seq 1 60); do
  proxies=$(docker exec "${PROJECT}-auth" tctl get proxies --format=json 2>/dev/null | grep -c '"kind": *"proxy"' || true)
  nodes=$(docker exec "${PROJECT}-auth" tctl get nodes --format=json 2>/dev/null | grep -c '"kind": *"node"' || true)
  if [[ ${proxies:-0} -ge 1 && ${nodes:-0} -ge 1 ]]; then
    echo "Cluster ready (proxies=$proxies, nodes=$nodes)" >&2
    echo "$PROJECT"
    exit 0
  fi
  sleep 2
done

echo "Timed out waiting for registration (proxies=$proxies, nodes=$nodes)" >&2
echo "$PROJECT"
exit 1
