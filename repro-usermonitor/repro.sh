#!/usr/bin/env bash
# Reproduce the Okta usermonitor bug where rapid bot delete/recreate races
# the monitor and a stale UserLoginState resource leaks into the new bot's
# identity, breaking the first `tbot start`.
#
# Requirements:
#   - The patched auth server is running (see ../e/lib/auth/plugin.go HACK).
#   - The runner bot has been created (see README.md).
#   - tbot-runner.yaml has been pointed at your proxy and run at least once
#     so ./runner-identity/identity exists.
#
# Usage:
#   PROXY=teleport.example.com:443 ./repro.sh
#
# Knobs (env vars, all optional):
#   ITERATIONS=50              cycles per worker
#   PARALLEL=3                 concurrent workers
#   DELAY_BEFORE_RECREATE=0.5  seconds between rm and next add  (try 0, 0.1, 1, 5)
#   DELAY_AFTER_RECREATE=0     seconds between add and tbot join (try 0, 0.1)
#   RUNNER_IDENTITY=./runner-identity/identity
#   TBOT=./build/tbot          path to tbot binary
#   TCTL=./build/tctl          path to tctl binary (must match deployed auth version)
set -uo pipefail

PROXY=${PROXY:?set PROXY=host:port}
RUNNER_IDENTITY=${RUNNER_IDENTITY:-$(pwd)/runner-identity/identity}
ITERATIONS=${ITERATIONS:-50}
PARALLEL=${PARALLEL:-3}
DELAY_BEFORE_RECREATE=${DELAY_BEFORE_RECREATE:-0.5}
DELAY_AFTER_RECREATE=${DELAY_AFTER_RECREATE:-0}
TBOT=${TBOT:-../build/tbot}
TCTL=${TCTL:-../build/tctl}

if [[ ! -f "$RUNNER_IDENTITY" ]]; then
  echo "runner identity not found at $RUNNER_IDENTITY — run 'tbot start -c tbot-runner.yaml' first" >&2
  exit 1
fi

tctl_cmd() { "$TCTL" --identity "$RUNNER_IDENTITY" --auth-server "$PROXY" "$@"; }

# Sweep any repro-victim-* bots left behind by a previous run so they don't
# collide with this one. Failures here are non-fatal — if the list call
# breaks, we just press on and let `bots add` complain.
sweep_leftovers() {
  local -a stale=()
  while IFS= read -r name; do
    [[ -n "$name" ]] && stale+=("$name")
  done < <(tctl_cmd bots ls --format=json 2>/dev/null \
            | jq -r '.[] | .metadata.name // .status.user_name // empty' \
            | grep -E '^repro-victim-' || true)
  if (( ${#stale[@]} == 0 )); then
    echo "no leftover repro-victim-* bots"
    return 0
  fi
  echo "removing ${#stale[@]} leftover bot(s):"
  printf '  %s\n' "${stale[@]}"
  for name in "${stale[@]}"; do
    tctl_cmd bots rm "$name" >/dev/null 2>&1 || true
  done
}

sweep_leftovers

worker() {
  local id=$1
  local bot addout token tmp logfile rc
  for ((i = 0; i < ITERATIONS; i++)); do
    bot="repro-victim-${id}-${i}"

    # `--legacy` keeps the join method as a plain `token` (not bound-keypair)
    # so we can hand it straight to `tbot start --token=…`. `--format=json`
    # gives us a stable shape to pull the generated token out of.
    if ! addout=$(tctl_cmd bots add "$bot" \
          --roles=usermonitor-repro-victim \
          --legacy \
          --ttl=1h \
          --format=json 2>&1); then
      echo "[worker $id iter $i] bots add failed for $bot: $addout" >&2
      return 1
    fi
    token=$(printf '%s' "$addout" | jq -r '.token_id // .tokenID // .token // empty')
    if [[ -z "$token" ]]; then
      echo "[worker $id iter $i] could not parse token from 'bots add' output:" >&2
      printf '%s\n' "$addout" | sed 's/^/  /' >&2
      return 1
    fi

    (( $(echo "$DELAY_AFTER_RECREATE > 0" | bc -l) )) && sleep "$DELAY_AFTER_RECREATE"

    tmp=$(mktemp -d)
    logfile="$tmp/tbot.log"
    if ! "$TBOT" start identity \
          --oneshot \
          --proxy-server="$PROXY" \
          --token="$token" \
          --join-method=token \
          --storage=memory:// \
          --destination="$tmp/out" 2>&1; then
      rc=$?
      echo
      echo "==================== BUG REPRODUCED ====================" >&2
      echo "[worker $id iter $i] tbot join failed (rc=$rc) for bot $bot" >&2
      echo "-- tbot log --" >&2
      sed 's/^/  /' "$logfile" >&2
      echo "-- bot resource --" >&2
      tctl_cmd get "bot/$bot" 2>&1 | sed 's/^/  /' >&2
      echo
      echo "NOTE: tctl has no get verb for user_login_state. To check whether" >&2
      echo "      a stale ULS leaked, inspect the auth-server backend directly," >&2
      echo "      e.g. on a sqlite backend:" >&2
      echo "        sqlite3 /var/lib/teleport/backend/sqlite.db \\" >&2
      echo "          \"select key,value from kv where key like '%user_login_state%bot-$bot%';\"" >&2
      echo "      Leave the failing bot in place until you've collected this." >&2
      echo "========================================================" >&2
      return 2
    fi

    tctl_cmd bots rm "$bot" >/dev/null 2>&1 || true
    rm -rf "$tmp"
    sleep "$DELAY_BEFORE_RECREATE"
  done
  echo "[worker $id] completed $ITERATIONS iterations clean"
}

echo "starting $PARALLEL workers x $ITERATIONS iterations (delay before recreate=${DELAY_BEFORE_RECREATE}s)"
pids=()
for ((p = 0; p < PARALLEL; p++)); do
  worker "$p" &
  pids+=($!)
done

fail=0
for pid in "${pids[@]}"; do
  if ! wait "$pid"; then fail=1; fi
done
exit $fail
