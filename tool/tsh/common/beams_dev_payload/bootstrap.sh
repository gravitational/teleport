#!/bin/bash
# beams-dev successor bootstrap: runs ON the successor beam right after the
# handoff archive is extracted. Reinstalls the cron entries so the payload
# keeps running on the new beam (making handoff chains work even if the
# laptop stays offline across multiple expiries).
set -u
META_DIR="/home/beams/.beams-dev"
[ -f "$META_DIR/meta.env" ] || exit 1

rm -f "$META_DIR/handoff.done" "$META_DIR/successor" "$META_DIR/successor.pending" \
    "$META_DIR/handoff.lock" "$META_DIR/wip-push.lock" "$META_DIR/last-wip-push"
chmod +x "$META_DIR"/*.sh 2>/dev/null

if command -v crontab >/dev/null 2>&1; then
    crontab "$META_DIR/crontab" 2>>"$META_DIR/handoff.log"
elif ! pgrep -f beams-dev-poller >/dev/null 2>&1; then
    # No cron on this image: run a detached poller instead. The $0 marker
    # keeps re-installs from stacking duplicates. Each script invocation is
    # bounded so one hung run can never stop the loop for good.
    setsid bash -c "while true; do timeout -k 30 240 $META_DIR/wip-push.sh; timeout -k 60 3000 $META_DIR/handoff.sh; sleep 300; done" beams-dev-poller >/dev/null 2>&1 < /dev/null &
fi
echo "bootstrap complete" >> "$META_DIR/handoff.log"
