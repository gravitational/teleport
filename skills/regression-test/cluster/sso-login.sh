#!/usr/bin/env bash
# sso-login.sh — drive `tsh login` against the mock OIDC IdP for a given
# actor. Uses plain tsh in a transient container with the actor's persistent
# home volume; a sidecar curl container walks the OIDC redirect chain so no
# real browser is needed.
#
# Usage: sso-login.sh <project> <user> <version> [extra-tsh-login-args...]
#
# Extra args are appended to `tsh login`. Common use: --request-id=<id> to
# assume an approved access request.

set -euo pipefail

PROJECT="${1:?usage: sso-login.sh <project> <user> <version> [args...]}"
USER_NAME="${2:?usage: sso-login.sh <project> <user> <version> [args...]}"
VERSION="${3:?usage: sso-login.sh <project> <user> <version> [args...]}"
shift 3

VOLUME="${PROJECT}-${USER_NAME}-home"
docker volume create "$VOLUME" >/dev/null

LOGIN_CONTAINER="${PROJECT}-${USER_NAME}-tsh-login"
CALLBACK_PORT=1234

# This cluster's network lives on 10.99.<slot>.0/24, where <slot> was chosen by
# up.sh and persisted to state/<project>.env. Default to 0 for clusters brought
# up before slots existed.
SLOT=0
STATE_FILE="$(dirname "$0")/state/${PROJECT}.env"
[[ -f "$STATE_FILE" ]] && source "$STATE_FILE"

# Pin a static IP per actor in the cluster's docker subnet (10.99.<slot>.0/24).
# Teleport's SSO callback validator only allows HTTP redirects to IP literals
# inside an OIDC connector's insecure_allowed_cidr_ranges list — not DNS names.
# Hash the username to a stable host octet in the upper half of the subnet to
# avoid colliding with docker's auto-assigned IPs.
OCTET=$(printf '%s' "$USER_NAME" | cksum | awk '{ print 100 + ($1 % 100) }')
LOGIN_IP="10.99.${SLOT}.${OCTET}"

# Ensure no leftover from a previous run.
docker rm -f "$LOGIN_CONTAINER" >/dev/null 2>&1 || true

# Start tsh login in foreground (so we can pipe "y" to its callback-confirmation
# prompt), backgrounded via `&`. The username comes from the OIDC connector
# (we don't pass --user), and --browser=none makes tsh print the login URL
# instead of opening a browser. --callback tells the proxy where to send the
# user back — the tsh container's docker DNS name, reachable on the same network.
LOGIN_LOG=$(mktemp)
docker run --rm \
  --network "${PROJECT}-net" \
  --ip "$LOGIN_IP" \
  -v "${VOLUME}:/home/teleport" \
  -e HOME=/home/teleport \
  -e TELEPORT_LOGIN_SKIP_REMOTE_HOST_WARNING=1 \
  --user 0:0 \
  --hostname "$LOGIN_CONTAINER" \
  --name "$LOGIN_CONTAINER" \
  --entrypoint=/usr/local/bin/tsh \
  "public.ecr.aws/gravitational/teleport-ent-distroless:${VERSION}" \
  login \
    --proxy=teleport-proxy:3080 \
    --auth="${USER_NAME}-oidc" \
    --browser=none \
    --bind-addr="0.0.0.0:${CALLBACK_PORT}" \
    --callback="${LOGIN_IP}:${CALLBACK_PORT}" \
    --insecure \
    --skip-version-check \
    "$@" >"$LOGIN_LOG" 2>&1 &
TSH_PID=$!

cleanup() {
  docker rm -f "$LOGIN_CONTAINER" >/dev/null 2>&1 || true
  rm -f "$LOGIN_LOG"
}
trap cleanup EXIT

# Poll for tsh's printed listener URL. Looks like:
#   Use the following URL to authenticate:
#    https://10.99.0.132:1234/<session-id>
# Hitting this URL starts the SSO redirect chain.
URL=""
for i in $(seq 1 30); do
  URL=$(grep -oE "https://${LOGIN_IP}:${CALLBACK_PORT}/[a-f0-9-]+" "$LOGIN_LOG" 2>/dev/null | head -1 || true)
  [[ -n "$URL" ]] && break
  sleep 1
done
if [[ -z "$URL" ]]; then
  echo "sso-login: timed out waiting for tsh to print the login URL" >&2
  cat "$LOGIN_LOG" >&2 || true
  kill "$TSH_PID" 2>/dev/null || true
  exit 1
fi
echo "sso-login: following $URL" >&2

# Walk the redirect chain. curl follows tsh listener -> mock IdP -> Teleport
# callback -> Teleport's HTML redirector page. The very last hop is a meta
# refresh (curl doesn't follow these), so we fetch the page, extract the URL
# from the <meta http-equiv="refresh"> tag, decode HTML entities, and hit it.
HTTP_URL="${URL/https:/http:}"
HTML=$(docker run --rm --network "${PROJECT}-net" curlimages/curl:latest \
  -sLk "$HTTP_URL" 2>/dev/null || true)

# Extract URL from: <meta http-equiv="refresh" content="0;URL='https://...'"
REFRESH=$(printf '%s' "$HTML" | grep -oE "URL='[^']+'" | sed "s/URL='//;s/'$//" | head -1 || true)
if [[ -n "$REFRESH" ]]; then
  # Decode common HTML entities (& and +) and swap https→http (listener is HTTP).
  REFRESH=$(printf '%s' "$REFRESH" \
    | sed 's/&amp;/\&/g; s/&#43;/+/g; s/&quot;/"/g' \
    | sed 's|^https://|http://|')
  docker run --rm --network "${PROJECT}-net" curlimages/curl:latest \
    -sLk -o /dev/null "$REFRESH" 2>/dev/null || true
fi

# Wait for tsh to exit; success = it received the cert and saved a profile.
if ! wait "$TSH_PID"; then
  echo "sso-login: tsh login failed" >&2
  cat "$LOGIN_LOG" >&2 || true
  exit 1
fi
tail -10 "$LOGIN_LOG" >&2
