---
name: okta-teleport-onboard
description: >-
  Onboard a Teleport cluster to Okta end-to-end (SSO + user sync + app/group
  sync) using the non-deprecated OAuth-for-Okta credential path. Given one
  bootstrap Okta SSWS token and an authenticated tsh session, it creates the
  Okta SAML app and OAuth API Services app, wires JWKS trust, and enrolls the
  Teleport Okta integration via the OktaService.CreateIntegration RPC. SCIM is
  intentionally out of scope (user sync provides equivalent provisioning).
  PROTOTYPE — dev Okta org + publicly-reachable dev cluster only.
---

# Okta ↔ Teleport onboarding (prototype)

## When to use
The user wants to stand up the Teleport Okta integration without clicking
through the web wizard. Scope: SSO, user sync, app/group sync (bidirectional).
NOT SCIM.

## Hard preconditions — verify before any mutation
1. `tsh status` succeeds and the identity can manage plugins (editor/admin).
2. The target is a **dev** cluster and a **dev** Okta org. Refuse otherwise —
   app/group sync writes back into Okta (bidirectional) and can remove users
   from Okta groups.
3. The cluster proxy is reachable from Okta over the public internet (required
   so Okta can fetch the JWKS URL).
4. A bootstrap **SSWS** token is available, created by an Okta admin with app-
   management and IAM-admin (roles/resource-sets) rights.

Collect and confirm these inputs with the user, then echo them back before
proceeding:
- `OKTA_ORG` (e.g. `https://dev-12345.okta.com`)
- `OKTA_SSWS` (bootstrap token — treated as sensitive, never logged)
- `PROXY` (e.g. `dev.teleport.sh:443`)
- SSO group(s) to assign to the app
- Access List default owner(s) (≥1, required by app/group sync)
- optional group/app import filters

Derived automatically:
- ACS/audience: `https://<PROXY-host>/v1/webapi/saml/acs/okta`
- JWKS URL: `https://<PROXY-host>/v1/.well-known/jwks-okta`

`source okta-teleport-onboard/scripts/okta.sh` exposes helpers used below.
Run every mutating call through the agent, inspect each response, and STOP on
the first unexpected error — do not blindly continue.

## Procedure

### 0. Preflight
- `okta::check_token` → expect HTTP 200.
- `curl -fsS https://<PROXY-host>/v1/.well-known/jwks-okta | jq .keys` → expect a
  non-empty JWKS. If empty/404, the cluster lacks the Okta CA; stop.

### 1. SSO — custom SAML 2.0 app
- `okta::create_saml_app "<label>" "https://<PROXY-host>/v1/webapi/saml/acs/okta"`; record `APP_ID` (`.id`).
- `METADATA_URL=$(okta::saml_metadata_url "$APP_ID")` — the PUBLIC metadata URL
  (`{org}/app/{exk-id}/sso/saml/metadata`). Do NOT use `._links.metadata.href`;
  that API endpoint is SSWS-gated and Teleport fetches metadata anonymously (403).
- Assign each SSO group: `okta::assign_group "$APP_ID" "$GROUP_ID"`
  (resolve group ids via `okta::find_group "<name>"`).

### 2. OAuth API Services app  ⚠️ payload unvalidated — see scripts/okta.sh
- `okta::create_service_app "<label>" "https://<PROXY-host>/v1/.well-known/jwks-okta"`
- Record `CLIENT_ID` (`.credentials.oauthClient.client_id`).
- Disable DPoP and confirm `token_endpoint_auth_method=private_key_jwt` with the
  JWKS URL bound. **Read the response**; if a field is rejected, adjust per the
  error and retry — do not paper over it.
- Grant scopes: `okta::grant_scope "$SERVICE_APP_ID" <scope>` for
  `okta.users.read okta.users.manage okta.groups.read okta.groups.manage okta.apps.read okta.apps.manage`.

### 3. Scoped admin access  ⚠️ IAM API unvalidated — see scripts/okta.sh
- `okta::create_admin_role` (user/group/app view+manage perms) → `ROLE_ID`.
- `okta::create_resource_set` including the SAML app + in-scope apps/groups →
  `RSET_ID`.
- `okta::assign_role "$SERVICE_APP_ID" "$ROLE_ID" "$RSET_ID"`.
- If the IAM endpoints block the prototype, fall back to assigning a built-in
  admin role to the service app and note the reduced scoping to the user.

### 4. Validate credentials (no writes)
The helper is part of the Teleport Go module (no own go.mod), so run it from the
checkout root `$TELEPORT_REPO` to build against the cluster's proto. It lives under
`.claude/` — a dot-dir Go excludes from `./...` wildcards but runs fine by explicit path.
```
(cd "$TELEPORT_REPO" && go run ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll \
  -proxy "$PROXY" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -validate-only)
```
Must succeed before enrolling. A failure here means Okta can't verify Teleport's
JWT — recheck the JWKS trust and DPoP settings in step 2.

### 5. Enroll the Teleport integration
```
(cd "$TELEPORT_REPO" && go run ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll \
  -proxy "$PROXY" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -metadata-url "$METADATA_URL" \
  -owner "<owner1>,<owner2>" -group-filter "<glob>" -app-filter "<glob>")
```
This one RPC creates the SAML connector (from `-metadata-url`), the plugin, and
the OAuth credential, with user sync + app/group sync + bidirectional sync on.

### 6. Verify
- `tctl get plugins/okta --format=json | jq '.[].status'` → `RUNNING`.
- `tctl get saml/okta` exists.
- After a sync cycle, `tctl get users --format=json | jq '.[]|select(.metadata.labels."teleport.dev/origin"=="okta")|.metadata.name'` shows imported users.

### 7. Revoke the bootstrap token
The runtime now authenticates via the OAuth client ID; the SSWS token is no
longer needed. `okta::revoke_token` (or delete it in the Okta admin UI). Confirm
sync still succeeds afterward.

## Teardown
`okta-teleport-onboard/scripts/cleanup.sh <saml-app-id> <service-app-id>` removes
the Teleport plugin + connector and the Okta apps created here. Re-runnable.

## Known first-run validation points (isolated to steps 2–3)
- Okta OIDC **service app** creation body + binding the JWKS `jwks_uri` and
  `private_key_jwt` via API.
- The DPoP-disable field name on the app.
- Okta **IAM** roles / resource-sets / assignment endpoints (newer surface).
These do not affect SSO/user-sync/app-group-sync correctness once the client ID
is trusted; they only determine how scoped the service app is.
