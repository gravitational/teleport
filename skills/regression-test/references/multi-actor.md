# Multi-actor reference

Most regression scenarios need multiple identities (e.g. a requester + a reviewer for access requests). Some flows — `tsh login --request-id=<id>` in particular — require a real, non-impersonated user session. `tctl auth sign` identity files don't qualify (they're admin-signed and tagged as impersonator), and bootstrapping local password+MFA non-interactively is fragile.

To get clean user sessions via plain `tsh`, this skill runs a tiny mock OIDC IdP alongside the Teleport cluster. Each test user gets a per-user OIDC connector; logins go through the standard OIDC flow.

## Topology

- `mock-oidc` — a container running `cluster/mock-oidc/server.py` (built into image `regression-mock-oidc` on first `up.sh` invocation). Serves `/.well-known/openid-configuration`, `/jwks`, `/authorize`, and `/token` under a per-user issuer path (e.g. `http://mock-oidc:8080/alice`).
- `cluster/mock-oidc-users.json` — per-user `groups` claim mapping. Updated by `setup-user.sh` as users are registered; the mock re-reads it on every `/token` request.
- Per-user OIDC connectors named `<user>-oidc` in Teleport. Each maps the mock's `groups` claim to a set of Teleport roles.

## Registering a user

```
./cluster/setup-user.sh <project> <user> <comma-separated-roles>
```

This:
1. Adds `<user>` to `mock-oidc-users.json` with `groups: [<primary-role>]`.
2. Creates the Teleport user record via `tctl users add` (best-effort — needed only for access-request RBAC name resolution).
3. Registers an OIDC connector `<user>-oidc` whose `claims_to_roles` maps that group back to the requested roles.

## Logging in

```
./cluster/sso-login.sh <project> <user> <proxy-version> [extra-tsh-login-args...]
```

This:
1. Starts `tsh login --auth=<user>-oidc --browser=none --bind-addr=...` in a transient docker container, with the actor's persistent `<project>-<user>-home` volume mounted at `$HOME`.
2. Reads the login URL that tsh prints.
3. Spawns a sidecar curl container to walk the OIDC redirect chain: Teleport → mock IdP → Teleport callback → tsh's bind-addr.
4. Waits for tsh to exit (success means it received the cert and wrote the profile).

The redirect chain is fully non-interactive because the mock IdP auto-issues an authorization code at `/authorize` (no UI). Extra args (e.g. `--request-id=<id>`) are passed through to `tsh login`.

## Running ordinary tsh commands

Once an actor's volume has a saved profile, `cluster/tsh.sh <project> <actor> <version> <args>` runs subsequent tsh commands re-using the cached cert. No re-prompt, no SSO ceremony — same as a normal user after login.

## Re-login for access-request assumption

`tsh login --request-id=<id>` reissues the cert with the requested role added. Run it through `sso-login.sh` (not `tsh.sh`), because it re-triggers the SSO flow:

```
./cluster/sso-login.sh <project> alice <proxy-version> --request-id=<id>
```

## Admin operations

For cluster-admin work (creating roles, querying audit state, approving from auth's side), continue to use `docker exec <project>-auth tctl ...`. The OIDC machinery is for actor identities, not admin operations.

## Caveats

- The mock OIDC server uses HTTP, not HTTPS. Teleport accepts this for testing; in production OIDC issuers must be HTTPS.
- Each test user needs its own OIDC connector. This adds a one-line cost per user but keeps the mock IdP config dead simple.
- Per-actor volumes (`<project>-<user>-home`) persist across `sso-login.sh` calls within a run; they're removed by `cluster/down.sh`.
