#!/bin/bash
# beams-dev dead-man's push: runs from cron on the beam every few minutes.
# Snapshots the working tree (WITHOUT touching the index, HEAD, or the working
# tree itself) and pushes it to a wip ref on origin, so work done after the
# developer's laptop went offline survives the beam's 24h expiry.
set -u
META_DIR="/home/beams/.beams-dev"
# shellcheck source=/dev/null
. "$META_DIR/meta.env" || exit 0

# Never block: a credential prompt or a stalled push must not wedge the
# poller loop that also drives the self-handoff.
export GIT_TERMINAL_PROMPT=0
export GIT_SSH_COMMAND="ssh -oBatchMode=yes"

exec 8>"$META_DIR/wip-push.lock"
flock -n 8 || exit 0

cd "$REPO_DIR" 2>/dev/null || exit 0

# `git stash create` builds a commit of the dirty tree without any side
# effects; empty output means the tree is clean and HEAD is the snapshot.
SNAPSHOT="$(git stash create 2>/dev/null || true)"
[ -z "$SNAPSHOT" ] && SNAPSHOT="$(git rev-parse HEAD 2>/dev/null)" || true
[ -z "$SNAPSHOT" ] && exit 0

LAST_FILE="$META_DIR/last-wip-push"
LAST="$(cat "$LAST_FILE" 2>/dev/null || true)"
[ "$LAST" = "$SNAPSHOT" ] && exit 0

if timeout -k 30 120 git push origin "+$SNAPSHOT:refs/heads/beams-wip/$WORKSPACE_ID" >/dev/null 2>&1; then
    echo "$SNAPSHOT" > "$LAST_FILE"
fi
