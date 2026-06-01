#!/usr/bin/env bash
# Self-contained runner for the direct-ssh scenario (no access request).
# alice gets node-access directly; she lists the node and SSHes in as root.
# Runs the whole lifecycle (up -> prep -> flow -> down) in one process so
# Claude Code prompts at most once. Writes a structured per-step log.
#
# Usage: run-direct-ssh.sh <auth> <proxy> <node> [tsh]
#   [tsh]  optional tsh CLIENT version; defaults to <proxy>. Set it to test a
#          client-only version skew against an otherwise-uniform cluster.
set -u

AUTH="$1"; PROXY="$2"; NODE="$3"; TSH="${4:-$PROXY}"
HERE="/Users/r0mant/.claude/skills/regression-test"
CLUSTER="$HERE/cluster"
LOG="$HERE/state/run-direct-ssh-${AUTH}_${PROXY}_${NODE}-tsh${TSH}.log"
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
log "##TSH_VERSION $TSH"
log "##RESULT up PASS"

# --- Prep: role -----------------------------------------------------------
step "prep-roles"
docker exec -i "${PROJ}-auth" tctl create -f - >>"$LOG" 2>&1 <<'EOF'
kind: role
version: v7
metadata:
  name: node-access
spec:
  allow:
    logins: [root]
    node_labels:
      env: ['test']
EOF
[ $? -eq 0 ] && log "##RESULT prep-roles PASS" || { log "##RESULT prep-roles FAIL"; exit 1; }

# --- Prep: user + SSO login (alice gets node-access directly) -------------
step "prep-users"
"$CLUSTER/setup-user.sh" "$PROJ" alice node-access >>"$LOG" 2>&1 && \
"$CLUSTER/sso-login.sh"  "$PROJ" alice "$TSH"       >>"$LOG" 2>&1
[ $? -eq 0 ] && log "##RESULT prep-users PASS" || { log "##RESULT prep-users FAIL"; exit 1; }

NODE_ID=$(docker exec "${PROJ}-auth" tctl get nodes --format=json 2>>"$LOG" | jq -r '.[0].metadata.name')
log "##NODE_ID $NODE_ID"
[ -n "$NODE_ID" ] && [ "$NODE_ID" != "null" ] || { log "##RESULT prep-users FAIL (no node id)"; exit 1; }

# --- Step 1: alice sees the node ------------------------------------------
# `tsh ls` displays the node hostname + labels, not the UUID, so assert on the
# env=test label (the role-relevant attribute) rather than the node UUID.
step "1 alice ls (sees node)"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$TSH" ls --insecure 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
if [ $RC -eq 0 ] && echo "$OUT" | grep -q "env=test"; then log "##RESULT 1 PASS"; else log "##RESULT 1 FAIL"; fi

# --- Step 2: alice SSHes to node (regression-critical) --------------------
step "2 alice ssh root@node"
OUT=$(to 30 "$CLUSTER/tsh.sh" "$PROJ" alice "$TSH" ssh --insecure "root@$NODE_ID" 'echo USER=$USER' 2>&1); RC=$?
log "$OUT"; log "##EXIT $RC"
if [ $RC -eq 0 ] && echo "$OUT" | grep -q "USER=root"; then log "##RESULT 2 PASS"; else log "##RESULT 2 FAIL"; fi

log "##DONE"
