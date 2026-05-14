# Okta usermonitor / bot delete-recreate repro

## Theory
The Okta usermonitor watches `KindUser` events and, on Put, calls
`authServer.CallLoginHooks(ctx, user)` for the user — which goes through the
UserLoginState builder. If a bot user is rapidly deleted and recreated, the
event handler may run against the *deleted* user and create or leave behind a
`user_login_state` resource that no longer matches the freshly-created bot,
breaking the next `tbot` join.

See [e/lib/okta/usermonitor/usermonitor.go:336](../e/lib/okta/usermonitor/usermonitor.go#L336)
and [e/lib/okta/usermonitor/usermonitor.go:388](../e/lib/okta/usermonitor/usermonitor.go#L388).

## Auth-server patch
Located in [e/lib/auth/plugin.go](../e/lib/auth/plugin.go), search for
`HACK(usermonitor-repro)`. It unconditionally starts the user monitor at the
end of `RegisterAuthServices`, so no Okta plugin / credentials are required.

Rebuild and deploy the auth server. Don't ship the patch.

## Bootstrap (on the homelab, once)

```bash
# 1. Create roles.
tctl create -f runner-role.yaml
tctl create -f victim-role.yaml

# 2. Create the runner bot and its long-lived join token.
tctl bots add usermonitor-repro-runner \
     --roles=usermonitor-repro-runner \
     --token=usermonitor-repro-runner \
     --ttl=720h

# 3. Edit tbot-runner.yaml — set proxy_server to your proxy address.

# 4. Start the runner tbot in a separate terminal (or under systemd).
tbot start -c tbot-runner.yaml
# Wait until ./runner-identity/identity exists.
```

## Run the repro

```bash
PROXY=teleport.example.com:443 ./repro.sh
```

Useful knobs (env vars):
- `ITERATIONS` (default 50) — cycles per worker
- `PARALLEL` (default 3) — concurrent workers
- `DELAY_BEFORE_RECREATE` (default 0.5) — seconds between `bots rm` and the
  next `bots add`. Try 0, 0.1, 1, 5 to find the window where the bug fires.
- `DELAY_AFTER_RECREATE` (default 0) — seconds between `bots add` and the
  `tbot` join. Try 0 vs 0.1 to give the monitor time to (mis-)process the
  put event before tbot joins.

The script tears workers down on the first failure and dumps:
- the failing tbot log,
- the bot resource,
- a pointer for inspecting `user_login_state` directly out of the auth
  backend (tctl has no get verb for that kind). On a sqlite backend:

  ```
  sqlite3 /var/lib/teleport/backend/sqlite.db \
    "select key,value from kv where key like '%user_login_state%bot-<name>%';"
  ```

  A stale ULS for a freshly-created bot is the suspected smoking gun.

## Cleanup

```bash
# Bots created by the script are deleted as they go, but a failed run
# leaves the last bot in place; sweep them with:
tctl get bots --format=json | jq -r '.[] | select(.metadata.name | startswith("repro-victim-")) | .metadata.name' \
  | xargs -r -n1 tctl bots rm

# Stale UserLoginStates can't be enumerated via tctl — inspect the backend
# directly on the auth host (sqlite example):
#   sqlite3 /var/lib/teleport/backend/sqlite.db \
#     "select key from kv where key like '%user_login_state%bot-repro-victim-%';"
```

To reset ULS state between runs (stop teleport first so the cache doesn't
fight you):

```bash
# Inspect what's about to be removed.
sqlite3 /var/lib/teleport/backend/sqlite.db \
  "select key from kv where key like '%user_login_state%bot-repro-victim-%';"

# Drop only the repro-victim ULS rows.
sqlite3 /var/lib/teleport/backend/sqlite.db \
  "delete from kv where key like '%user_login_state%bot-repro-victim-%';"

# Nuclear: drop *all* user_login_state rows (only safe in a homelab — this
# will force every user to have their login hooks re-run on next login).
sqlite3 /var/lib/teleport/backend/sqlite.db \
  "delete from kv where key like '%user_login_state%';"
```
