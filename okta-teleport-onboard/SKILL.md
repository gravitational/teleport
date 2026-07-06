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

Expected invocation: *"Onboard `<cluster>` using this skill. Okta creds are in
`<file>`."* Everything else is derived, confirmed once, then executed by a single
orchestrator (`scripts/onboard.sh`) that emits clean progress — you relay it, you
do not re-run the individual helpers by hand unless the orchestrator fails.

## Interaction protocol (UX) — read first
Drive this as a guided, confirm-once-then-execute flow.

- **Derive, don't interrogate.** Infer `PROXY` from `tsh status`, `OKTA_ORG`/`OKTA_SSWS`
  from the creds file, and default SSO group `Everyone`, ACL owner = logged-in user,
  filters `*`. Only ask the user when something can't be derived.
- **Gate A — confirm the plan (BLOCKING, before running the orchestrator).** Present a
  summary card and use `AskUserQuestion` for explicit approval. The card shows: target
  cluster + proxy, Okta org, SSO group(s), Access List owner(s), import filters, and
  the objects that will be created on each side — plus the warning that app/group sync
  is **bidirectional and writes back into Okta**. Options: "Proceed", "Change an input",
  "Cancel". Run nothing that mutates until "Proceed". (Preflight is the orchestrator's
  first, read-only phase and aborts safely on its own.)
- **Gate C — confirm teardown (BLOCKING).** Before `cleanup.sh`, list what will be
  deleted (plugin, connector, N Access Lists, N okta-origin users, the Okta apps +
  role + resource set) and require explicit approval — teardown is destructive.
- **Bootstrap token: NEVER revoke.** The SSWS token is the user's to manage. The skill
  must never list, revoke, or delete it, and there are deliberately no helpers to do so.
- **Formatting.** Relay the orchestrator's `==> section` / `OK` lines as a clean
  progress log; keep a `✅/⏳/⬜` step checklist. Present created objects as a
  `Resource | ID` table. Never print the SSWS token — refer to it as "(provided)".
- **No internal narration.** The user cares about the task, not the mechanics. Do NOT
  narrate tool use ("let me read the script", "running shell command", "reading file"),
  and do NOT pre-read the skill's own scripts — they are validated; run them directly.
  Surface only task-level progress (e.g. "✓ plugin deleted", "✓ 7 Access Lists removed"),
  the plan card, questions, and the summary — never how you execute them. The scripts
  self-report each result (deleted / already gone / error / OK); trust their output and
  do NOT run extra verification or inspection commands afterward.
- **On failure.** The orchestrator stops at the first error with an `ERR <reason>` line.
  Surface it and ask via `AskUserQuestion` how to proceed (retry / adjust / abort). Do
  not silently re-run or paper over it.
- **Closing summary.** Resource table + plugin status + imported-user count + the
  teardown command (`scripts/cleanup.sh`, no args).

## Preconditions
1. `tsh status` succeeds and the identity can manage plugins (editor/admin).
2. Target is a **dev** cluster and a **dev** Okta org. Refuse otherwise — app/group
   sync writes back into Okta and can remove users from Okta groups.
3. The cluster proxy is reachable from Okta over the public internet (Okta must fetch
   the JWKS URL).
4. A bootstrap **SSWS** token exists (Okta admin with app-management + IAM-admin
   rights), available via the creds file.

## Run
1. **Load creds + derive inputs** (one shell block; keeps the token out of the log):
   ```
   set -a; source ~/.okta-onboard.env; set +a      # OKTA_ORG + OKTA_SSWS
   ```
   Derive `PROXY` from `tsh status`; default `SSO_GROUP=Everyone`,
   `ACL_OWNER=<logged-in user>`, `GROUP_FILTER=*`, `APP_FILTER=*`.
2. **Gate A** — render the confirmation card and get "Proceed".
3. **Execute** the orchestrator and relay its progress (single call):
   ```
   set -a; source ~/.okta-onboard.env; set +a
   PROXY="<host:443>" SSO_GROUP="Everyone" ACL_OWNER="<user>" \
     .claude/skills/okta-teleport-onboard/scripts/onboard.sh
   ```
   It runs preflight → SAML app + group + metadata → OAuth service app + scopes →
   scoped role + resource set + binding → Teleport validate + enroll, records the
   created IDs to `$OKTA_ONBOARD_STATE`, and prints them. On non-zero exit, see the
   `ERR` line and follow the on-failure protocol.
4. **Verify:**
   - `tctl get plugins/okta --format=json | jq '.[].status.code'` → `1` (RUNNING).
   - `tctl get saml/okta` exists.
   - After a sync cycle, okta-origin users appear:
     `tctl get users --format=json | jq '[.[]|select(.metadata.labels."teleport.dev/origin"=="okta")]|length'`.
5. **Summarize** per the closing-summary rule.

## Teardown / offboarding
Confirm via Gate C first — destructive. `scripts/cleanup.sh` needs **no args**:
onboarding recorded the object IDs to `$OKTA_ONBOARD_STATE` (default
`~/.okta-onboard.state`) and cleanup reads them (positional
`<saml-app> <svc-app> <role> <rset>` override). Enforced order, because bidirectional
sync is on:
1. `tctl plugins delete okta` — delete the plugin FIRST so later deletions don't
   propagate back into Okta.
2. `tctl plugins cleanup okta --no-dry-run` — remove Okta-sourced Access Lists +
   generated roles. Refuses to run while the plugin is active, hence step 1 first.
3. `tctl rm saml/okta` — connector is NOT auto-deleted (`tctl rm`, not `delete`).
4. Delete okta-origin Teleport users (cleanup doesn't touch users).
5. Okta: role binding → resource set → custom role, then deactivate+delete both apps.
`cleanup.sh` reports each deletion's outcome (deleted / already gone / error), so just
relay its output — run no follow-up verification. Re-runnable. The bootstrap SSWS token
is left untouched.

## Manual / debugging
`scripts/onboard.sh` is just an ordered driver over `scripts/okta.sh` helpers
(`okta::create_saml_app`, `saml_metadata_url`, `create_service_app`, `grant_scope`,
`create_admin_role`, `create_resource_set`, `assign_role`) plus the `tp-enroll` Go
helper. When the orchestrator fails, reproduce the single failing helper by hand to
inspect the raw response, fix the helper, and re-run `onboard.sh` (it upserts state
and Okta calls are create-or-adjust).

## Known validation points (isolated to the Okta service-app + IAM calls)
- OIDC **service app** creation body + JWKS `jwks_uri` / `private_key_jwt` binding.
- The DPoP-disable field name on the app.
- Okta **IAM** roles / resource-sets / bindings (newer API surface).
These don't affect SSO/user-sync/app-group-sync correctness once the client ID is
trusted; they only determine how scoped the service app is. Validated against an Okta
integrator org; re-verify on a materially different org version.
