#!/usr/bin/env bash
# down.sh — tear down a regression-test cluster. Idempotent; safe to run even if
# some containers already exited. Only touches resources for the given project,
# so it's safe to run while other parallel clusters are still up.

set -euo pipefail

PROJECT="${1:?usage: down.sh <project-name>}"

cd "$(dirname "$0")"

export COMPOSE_PROJECT_NAME="$PROJECT"
# These don't matter for `down`, but compose still requires them to be set.
export AUTH_VERSION="${AUTH_VERSION:-latest}"
export PROXY_VERSION="${PROXY_VERSION:-latest}"
export NODE_VERSION="${NODE_VERSION:-latest}"

# Recover this project's subnet / users-file from state (fall back to defaults so
# compose variable substitution still resolves for older clusters).
STATE_FILE="state/${PROJECT}.env"
OIDC_USERS_FILE="./mock-oidc-users-${PROJECT}.json"
CLUSTER_SUBNET="10.99.0.0/24"
[[ -f "$STATE_FILE" ]] && source "$STATE_FILE"
export OIDC_USERS_FILE CLUSTER_SUBNET

docker compose -f compose.tmpl.yml down -v --remove-orphans >&2 || true

# Remove per-actor home volumes created by tsh.sh
docker volume ls -q --filter "name=^${PROJECT}-" | xargs -r docker volume rm >/dev/null 2>&1 || true

# Remove this project's transient mock-oidc users file and state file.
rm -f "$OIDC_USERS_FILE" "$STATE_FILE" 2>/dev/null || true

echo "Tore down $PROJECT" >&2
