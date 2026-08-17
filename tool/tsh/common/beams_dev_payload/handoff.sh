#!/bin/bash
# beams-dev self-handoff: runs from cron/poller on the beam every few minutes.
#
# When this beam is close to its 24h expiry and no laptop-side (warm) handoff
# has happened, the beam creates its own successor using its delegated
# identity (`tsh` is preinstalled and authenticated on every beam), copies the
# workspace + Claude sessions + this payload across, and leaves a pointer so
# the developer's client adopts the successor on next wake.
#
# Invariant: at most TWO beams ever serve a workspace — this one and one
# in-flight successor. The successor is recorded in successor.pending the
# moment it exists and is reused on retries (and deleted after repeated
# seeding failures) rather than leaked; after a completed handoff this beam
# deletes itself. Every tsh call is bounded by `timeout` so a hung call
# becomes a retryable failure instead of wedging the poller forever.
#
# Usage: handoff.sh [keepalive]
#   keepalive — skip the post-handoff self-delete. Passed by the laptop-driven
#   warm handoff, which verifies the successor and deletes this beam itself.
set -u
KEEPALIVE="${1:-}"
META_DIR="/home/beams/.beams-dev"
# shellcheck source=/dev/null
. "$META_DIR/meta.env" || exit 0

LOCK="$META_DIR/handoff.lock"
DONE="$META_DIR/handoff.done"
PENDING="$META_DIR/successor.pending"

log() { echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') $*" >> "$META_DIR/handoff.log"; }
fail() { log "$*"; exit 1; }

NOW="$(date +%s)"
REMAINING=$((EXPIRES_AT - NOW))

# After a completed handoff this beam is drained and only wastes quota. Defer
# while an agent is mid-task with runway left; otherwise delete ourselves.
self_destruct() {
    [ -n "$KEEPALIVE" ] && return 0
    if [ -z "${SELF_ALIAS:-}" ]; then
        log "no SELF_ALIAS in meta.env; cannot self-delete (beam will expire on its own)"
        return 0
    fi
    if [ "$REMAINING" -gt 600 ] && pgrep -f 'claude|codex' >/dev/null 2>&1; then
        log "handoff done; agent session still active — deferring self-delete"
        return 0
    fi
    log "handoff complete; deleting drained beam $SELF_ALIAS"
    timeout -k 30 120 tsh beams rm "$SELF_ALIAS" >>"$META_DIR/handoff.log" 2>&1
}

if [ -f "$DONE" ]; then
    self_destruct
    exit 0
fi

# Hand off when less than 90 minutes of life remain.
[ "$REMAINING" -gt 5400 ] && exit 0

# An agent mid-task must not have the beam rotated under it: defer while
# there is still runway (the poller retries), but inside the last 30 minutes
# rotate anyway — losing the in-flight turn beats losing the workspace.
if [ "$REMAINING" -gt 1800 ] && pgrep -f 'claude|codex' >/dev/null 2>&1; then
    exit 0
fi

exec 9>"$LOCK"
flock -n 9 || exit 0

# Resume an in-flight successor before ever creating a new one: retries must
# never accumulate beams. After 3 failed seeding attempts the successor is
# presumed broken and deleted; creating a replacement is blocked until that
# delete succeeds (or expiry is so close that saving the workspace wins).
NEW_UUID=""
NEW_ALIAS=""
ATTEMPTS=0
if [ -f "$PENDING" ]; then
    read -r NEW_UUID NEW_ALIAS ATTEMPTS < "$PENDING" || true
    ATTEMPTS="${ATTEMPTS:-0}"
    if [ -z "$NEW_ALIAS" ]; then
        rm -f "$PENDING"
        NEW_UUID=""
        ATTEMPTS=0
    elif [ "$ATTEMPTS" -ge 3 ]; then
        log "successor $NEW_ALIAS failed $ATTEMPTS seeding attempts; deleting it"
        if timeout -k 30 120 tsh beams rm "$NEW_ALIAS" >>"$META_DIR/handoff.log" 2>&1; then
            rm -f "$PENDING"; NEW_UUID=""; NEW_ALIAS=""; ATTEMPTS=0
        elif [ "$REMAINING" -lt 1800 ]; then
            log "cannot delete $NEW_ALIAS and expiry is close; abandoning it to save the workspace"
            rm -f "$PENDING"; NEW_UUID=""; NEW_ALIAS=""; ATTEMPTS=0
        else
            fail "delete of broken successor $NEW_ALIAS failed; retrying before creating a replacement"
        fi
    else
        log "resuming seeding of pending successor $NEW_ALIAS (attempt $((ATTEMPTS + 1)))"
    fi
fi

if [ -z "$NEW_ALIAS" ]; then
    log "expiry in ${REMAINING}s; creating successor beam"
    ADD_OUT="$(timeout -k 30 600 tsh beams add --no-console -f json 2>>"$META_DIR/handoff.log")" || fail "tsh beams add failed"
    NEW_ALIAS="$(printf '%s' "$ADD_OUT" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/')"
    NEW_UUID="$(printf '%s' "$ADD_OUT" | grep -o '"uuid"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/')"
    if [ -z "$NEW_ALIAS" ] || [ -z "$NEW_UUID" ]; then
        fail "could not parse successor id/uuid from: $ADD_OUT"
    fi
    log "successor beam: $NEW_ALIAS ($NEW_UUID)"
fi

# Record the attempt up front so even a hard kill mid-seed is accounted for.
printf '%s %s %s\n' "$NEW_UUID" "$NEW_ALIAS" "$((ATTEMPTS + 1))" > "$PENDING"

# Claim the successor for this workspace before any data lands: if seeding
# dies here, the claim lets the client reuse (or reap) the beam instead of
# leaving an anonymous orphan, and makes the failure attributable.
timeout -k 30 60 tsh beams exec "$NEW_ALIAS" "mkdir -p $META_DIR && echo $WORKSPACE_ID > $META_DIR/claim" \
    >>"$META_DIR/handoff.log" 2>&1 || fail "claiming successor $NEW_ALIAS failed"

REPO_NAME="$(basename "$REPO_DIR")"

# The successor inherits meta.env via the archive, so point it at its own
# identity/expiry for the tar, then restore ours IMMEDIATELY after — leaving
# the successor's values in place on a failed run would make every later tick
# think there are ~23h left and silently stop retrying.
sed -i \
    -e "s/^EXPIRES_AT=.*/EXPIRES_AT=$((NOW + 82800))/" \
    -e "s/^SELF_UUID=.*/SELF_UUID=$NEW_UUID/" \
    -e "s/^SELF_ALIAS=.*/SELF_ALIAS=$NEW_ALIAS/" \
    "$META_DIR/meta.env"

ARCHIVE="/tmp/beams-dev-handoff.tgz"
tar -czf "$ARCHIVE" -C /home/beams \
    "$REPO_NAME" .beams-dev \
    $( [ -d /home/beams/.claude ] && echo .claude ) \
    2>>"$META_DIR/handoff.log"
TAR_RC=$?

sed -i \
    -e "s/^EXPIRES_AT=.*/EXPIRES_AT=$EXPIRES_AT/" \
    -e "s/^SELF_UUID=.*/SELF_UUID=${SELF_UUID:-}/" \
    -e "s/^SELF_ALIAS=.*/SELF_ALIAS=${SELF_ALIAS:-}/" \
    "$META_DIR/meta.env"

# tar exits 1 when a file changed while being read (an agent or the wip-push
# may touch the tree) — the archive is still usable; only >1 is fatal.
[ "$TAR_RC" -gt 1 ] && fail "tar failed (rc=$TAR_RC)"

timeout -k 60 900 tsh beams scp -q "$ARCHIVE" "$NEW_ALIAS:$ARCHIVE" >>"$META_DIR/handoff.log" 2>&1 \
    || fail "scp to successor failed"
timeout -k 60 600 tsh beams exec "$NEW_ALIAS" "tar -xzf $ARCHIVE -C /home/beams && rm -f $ARCHIVE && /home/beams/.beams-dev/bootstrap.sh" \
    >>"$META_DIR/handoff.log" 2>&1 || fail "successor bootstrap failed"

# Leave a pointer for the client (read while this beam is still alive), mark
# the handoff done so it never runs twice, then delete ourselves.
printf '%s %s\n' "$NEW_UUID" "$NEW_ALIAS" > "$META_DIR/successor"
rm -f "$PENDING"
touch "$DONE"
log "handoff to $NEW_ALIAS complete"
self_destruct
