#!/usr/bin/env bash
# Self-contained runner for the resource-access-request scenario.
# Runs the whole lifecycle (up -> prep -> flow -> down) for one (auth,proxy,node)
# tuple inside a single process, so Claude Code prompts at most once (for this
# script), never per inner command. Writes a structured per-step log that the
# caller parses for the final report.
#
# Usage: run-tuple.sh <auth> <proxy> <node>
set -u

AUTH="$1"; PROXY="$2"; NODE="$3"
HERE="/Users/r0mant/.claude/skills/regression-test"
CLUSTER="$HERE/cluster"
LOG="$HERE/state/run-${AUTH}_${PROXY}_${NODE}.log"
mkdir -p "$HERE/state"
: >"$LOG"

log()  { echo "$*" >>"$LOG"; }
step() { echo "##STEP $*" >>"$LOG"; }

# portable 30s timeout (no `timeout` binary on macOS)
to() { perl -e 'alarm shift; exec @ARGV' "$1" "${@:2}"; }

PROJ=""
cleanup() { [ -n "$PROJ" ] && "$CLUSTER/down.sh" "$PROJ" >>"$LOG" 2>&1; }
trap cleanup EXIT

# --- Up -------------------------------------------------------------------
step "up"
PROJ=$("$CLUSTER/up.sh" "$AUTH" "$PROXY" "$NODE" 2>>"$LOG" | tail -1)
if [ -z "$PROJ" ]; then log "##RESULT up FAIL (no project name; cluster bring-up failed)"; exit 1; fi
log "##PROJECT $PROJ"
log "##RESULT up PASS"

# --- Prep: roles ----------------------------------------------------------
step "prep-roles"
docker exec -i "${PROJ}-auth" tctl create -f - >>"$LOG" 2>&1 <<'EOF'
kind: role
version: v7
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles: [node-access]
---
kind: role
version: v7
metadata:
  name: node-access
spec:
  allow:
    logins: [root]
    node_labels:
      env: ['test']
---
kind: role
version: v7
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles: [node-access]
EOF
[ $? -eq 0 ] && log "##RESULT prep-roles PASS" || { log "##RESULT prep-roles FAIL"; exit 1; }

# --- Prep: users + SSO logins --------------------------------------------
step "prep-users"
"$CLUSTER/setup-user.sh" "$PROJ" alice requester >>"$LOG" 2>&1 && \
"$CLUSTER/setup-user.sh" "$PROJ" bob   reviewer  >>"$LOG" 2>&1 && \
"$CLUSTER/sso-login.sh"  "$PROJ" alice "$PROXY"  >>"$LOG" 2>&1 && \
"$CLUSTER/sso-login.sh"  "$PROJ" bob   "$PROXY"  >>"$LOG" 2>&1
[ $? -eq 0 ] && log "##RESULT prep-users PASS" || { log "##RESULT prep-users FAIL"; exit 1; }

NODE_ID=$(docker exec "${PROJ}-auth" tctl get nodes --format=json 2>>"$LOG" | jq -r '.[0].metadata.name')
log "##NODE_ID $NODE_ID"
[ -n "$NODE_ID" ] && [ "$NODE_ID" != "null" ] || { log "##RESULT prep-users FAIL (no node id)"; exit 1; }

# --- Step 1: alice baseline (should see no nodes) -------------------------
step "1 alice baseline ls"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$PROXY" ls --insecure 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
if [ $RC -eq 0 ] && ! echo "$OUT" | grep -q "$NODE_ID"; then log "##RESULT 1 PASS"; else log "##RESULT 1 FAIL"; fi

# --- Step 2: alice request search -----------------------------------------
step "2 alice request search"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$PROXY" request search --kind=node --labels=env=test --insecure 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
if [ $RC -eq 0 ] && echo "$OUT" | grep -q "$NODE_ID"; then log "##RESULT 2 PASS"; else log "##RESULT 2 FAIL"; fi

# --- Step 3: alice creates access request (--nowait) ----------------------
step "3 alice request create"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$PROXY" request create \
  --resource "/regression.local/node/$NODE_ID" --reason regression-test --nowait --insecure 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
# Source of truth for the request ID is the create command's stdout (scenario's
# documented method). The tctl listing is only a fallback and is subject to a
# read-after-write race that can return an empty array (=> null) right after create.
REQ_ID=$(echo "$OUT" | sed -n 's/^Request ID:[[:space:]]*\([^[:space:]]*\).*/\1/p' | head -1)
if [ -z "$REQ_ID" ]; then
  REQ_ID=$(docker exec "${PROJ}-auth" tctl get access_requests --format=json 2>>"$LOG" | jq -r '.[0].metadata.name')
fi
# Poll for the request to appear as PENDING (state=1); the listing is eventually
# consistent and can lag the create's return (same race step 4 handles).
STATE=""
for _ in 1 2 3 4 5 6 7 8 9 10; do
  STATE=$(docker exec "${PROJ}-auth" tctl get access_requests --format=json 2>>"$LOG" | jq -r ".[] | select(.metadata.name==\"$REQ_ID\") | .spec.state")
  [ "$STATE" = "1" ] && break
  sleep 1
done
log "##REQ_ID $REQ_ID"; log "##STATE $STATE"
if [ $RC -eq 0 ] && [ -n "$REQ_ID" ] && [ "$REQ_ID" != "null" ] && [ "$STATE" = "1" ]; then log "##RESULT 3 PASS"; else log "##RESULT 3 FAIL"; fi

# --- Step 4: bob approves --------------------------------------------------
step "4 bob approve"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" bob "$PROXY" request review --approve "$REQ_ID" --insecure 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
# Poll for APPROVED (state=2) to avoid a read-after-write race against auth.
STATE=1
for _ in 1 2 3 4 5 6 7 8 9 10; do
  STATE=$(docker exec "${PROJ}-auth" tctl get access_requests --format=json 2>>"$LOG" | jq -r ".[] | select(.metadata.name==\"$REQ_ID\") | .spec.state")
  [ "$STATE" = "2" ] && break
  sleep 1
done
log "##STATE $STATE"
if [ $RC -eq 0 ] && { [ "$STATE" = "2" ] || echo "$OUT" | grep -q "APPROVED"; }; then log "##RESULT 4 PASS"; else log "##RESULT 4 FAIL"; fi

# --- Step 5: alice assumes approved request -------------------------------
step "5 alice assume (sso-login --request-id)"
OUT=$("$CLUSTER/sso-login.sh" "$PROJ" alice "$PROXY" --request-id="$REQ_ID" 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC (note: sso-login.sh may exit !=0 on the --request-id path; verified via tsh status below)"
STAT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$PROXY" status --insecure 2>&1)
log "$STAT"
# Source of truth is the reissued cert, not sso-login.sh's exit code: it returns
# nonzero when an already-approved --request-id prints "Approval received"
# instead of a login URL, even though the cert is correctly augmented.
if echo "$STAT" | grep -q "node-access" && echo "$STAT" | grep -q "$REQ_ID"; then log "##RESULT 5 PASS"; else log "##RESULT 5 FAIL"; fi

# --- Step 6: alice SSHes to node (regression-critical) --------------------
step "6 alice ssh root@node"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$PROXY" ssh --insecure "root@$NODE_ID" 'echo USER=$USER' 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
if [ $RC -eq 0 ] && echo "$OUT" | grep -q "USER=root"; then log "##RESULT 6 PASS"; else log "##RESULT 6 FAIL"; fi

log "##DONE"
