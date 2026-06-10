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
#
# Fix-testing knobs (write bad ULS rows directly into the auth backend over
# ssh, then exercise the bot-join path against them — useful for confirming
# that a candidate fix tolerates pre-existing bad ULS):
#   INJECT_BAD_ULS_SSH=user@auth-host   if set, inject a bad ULS for each bot
#                                       BEFORE creating it. unset = no inject.
#   INJECT_BAD_ULS_DB=/var/lib/teleport/backend/sqlite.db   path to sqlite db
#                                                           on the remote host
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

# inject_bad_uls $botname — write the exact bad ULS shape we observed during
# repro (null roles/traits, zero expiry) directly into the auth backend over
# ssh. No-op if INJECT_BAD_ULS_SSH is unset. The ULS is named bot-$botname,
# matching how Teleport derives the ULS name from the bot user. Written via
# INSERT OR REPLACE so a leftover row from a previous iteration is overwritten.
#
# Caveat: this writes to the backend behind the auth-server's back. The auth
# cache will see the new row when its next change-feed poll arrives, so a
# small delay between inject and join may be needed depending on cache
# refresh latency. Adjust DELAY_AFTER_RECREATE to tune.
INJECT_BAD_ULS_DB=${INJECT_BAD_ULS_DB:-/var/lib/teleport/backend/sqlite.db}
inject_bad_uls() {
  [[ -z "${INJECT_BAD_ULS_SSH:-}" ]] && return 0
  local botname=$1
  local username="bot-$botname"
  local key="/user_login_state/$username"
  local value
  value=$(jq -nc --arg n "$username" '{
    kind: "user_login_state",
    version: "v1",
    metadata: { name: $n, expires: "0001-01-01T00:00:00Z" },
    spec: {
      original_roles: null,
      original_traits: null,
      roles: null,
      traits: null,
      access_list_traits: null,
      user_type: "local"
    }
  }')
  # escape single quotes for the sqlite SQL literal
  local key_esc=${key//\'/\'\'}
  local value_esc=${value//\'/\'\'}
  # The sqlite backend schema (lib/backend/lite/lite.go):
  #   kv(key TEXT PK, modified INTEGER NOT NULL, expires DATETIME, value BLOB,
  #      revision TEXT NOT NULL DEFAULT "")
  #   events(id PK, type INTEGER NOT NULL, created INTEGER NOT NULL,
  #          kv_key TEXT, kv_modified INTEGER, kv_expires, kv_value, kv_revision)
  # We have to insert into both: the cache watches `events` for changes and
  # won't notice a kv-only insert. type=1 = OpPut. `modified` is a unix-nanos
  # logical clock; we approximate with strftime, which is fine since the
  # watcher only requires it to be monotonic per row.
  if ! ssh -o BatchMode=yes "$INJECT_BAD_ULS_SSH" \
        sudo sqlite3 "$INJECT_BAD_ULS_DB" <<EOF 2>&1
BEGIN;
INSERT OR REPLACE INTO kv(key, modified, expires, value, revision)
  VALUES ('$key_esc',
          CAST(strftime('%s','now') AS INTEGER) * 1000000000,
          NULL,
          '$value_esc',
          '');
INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision)
  SELECT 1, modified, key, modified, expires, value, revision
  FROM kv WHERE key = '$key_esc';
COMMIT;
EOF
  then
    echo "[inject] failed to insert bad ULS for $username" >&2
    return 1
  fi
}

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

    # Optional: poison the backend with a bad ULS for this bot before creating
    # it. No-op unless INJECT_BAD_ULS_SSH is set.
    if ! inject_bad_uls "$bot"; then
      echo "[worker $id iter $i] inject_bad_uls failed for $bot" >&2
      return 1
    fi

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
