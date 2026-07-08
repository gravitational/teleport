---
name: okta-teleport-onboard
description: >-
  Onboard a Teleport cluster to Okta end-to-end (SSO + user sync + app/group
  sync) using the non-deprecated OAuth-for-Okta credential path. The Okta side
  (SAML app, OAuth API-services app, scoped admin role/resource-set/binding) is
  provisioned by a Terraform module; the Teleport integration is then enrolled
  via the OktaService.CreateIntegration RPC. SCIM is out of scope (user sync
  provides equivalent provisioning). PROTOTYPE — dev Okta org + dev cluster only.
---

# Okta ↔ Teleport onboarding (prototype)

Two pieces: a **Terraform module** (`terraform/`) creates the Okta-side objects, and the
**`tp-enroll`** Go helper enrolls the Teleport integration from the module's outputs. Work
through the steps; adapt to what each command returns.

**DEV ONLY** — dev cluster + dev Okta org. App/group sync is bidirectional and writes back
into Okta.

## Setup
- **tsh:** logged in to the target (dev) cluster (`tsh status`). `tp-enroll` uses this profile;
  if it's expired you'll get an SSH/handshake error — re-login.
- **Credentials:** `OKTA_ORG_URL` (e.g. `https://dev-123.okta.com`) and `OKTA_API_TOKEN`
  (an Okta admin SSWS token). From the environment, a file the user names (`--env-file`/source
  it), or ask. **Never read, cat, or echo the token** — source the file to use it, don't print it.
- **Map to the okta provider's env** (it reads these): from `OKTA_ORG_URL` set
  `OKTA_ORG_NAME` = the subdomain and `OKTA_BASE_URL` = the rest
  (`https://integrator-1563488.okta.com` → `OKTA_ORG_NAME=integrator-1563488`,
  `OKTA_BASE_URL=okta.com`); `OKTA_API_TOKEN` is already the name the provider wants.
- **Derive:** `teleport_domain` = proxy host from `tsh status` (e.g. `jwardtest18.cloud.gravitational.io`).
- **Confirm before applying** — show the user `terraform plan` and get an OK before `apply`.

## Onboard
Run in `terraform/`. State + tfvars persist here (gitignored) for the deployment's life.

1. **Write `terraform.tfvars`** from the request:
   ```
   okta_org_url    = "$OKTA_ORG_URL"
   teleport_domain = "<proxy host>"
   sso_group       = "Everyone"          # or as requested
   ```
   (Sync group/app *filters* and the Access-List *owner* are NOT Terraform — they're
   `tp-enroll` flags in step 5.)

2. **Collision check (abort & report).** Before applying, look up the labels the module will
   create — `Teleport connector (<domain>)` and `Teleport API access (<domain>)` — e.g.
   `curl -sS -H "Authorization: SSWS $OKTA_API_TOKEN" "$OKTA_ORG_URL/api/v1/apps?q=Teleport%20connector"`.
   If a match exists that Terraform state doesn't already track, **stop and report which
   objects collide** — do not apply. (State makes normal re-runs idempotent; this only guards
   a fresh apply against pre-existing objects.)

3. **Plan & apply:**
   ```
   terraform init -input=false          # once
   terraform plan -input=false          # show this to the user; confirm
   terraform apply -auto-approve -input=false
   ```

4. **Read outputs:**
   ```
   terraform output -raw saml_metadata_url    # public {org}/app/{exk}/sso/saml/metadata
   terraform output -raw oauth_client_id
   ```

5. **Enroll Teleport** (from the repo root) — no curl/tctl OAuth path, so use the helper:
   ```
   GOTOOLCHAIN=local go run ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll \
     -proxy "<domain>:443" -org "$OKTA_ORG_URL" -oauth-id "<oauth_client_id>" \
     -metadata-url "<saml_metadata_url>" -owner "<owner>" \
     -group-filter "<glob or a,b,c>" -app-filter "<glob>"
   ```

6. **Verify:**
   ```
   tctl get plugins/okta --format=json | jq '.[].status.code'   # 1 = RUNNING
   tctl get saml/okta                                           # connector exists
   ```

## Teardown
Teleport FIRST (stops bidirectional sync before the Okta objects vanish), then destroy Okta:
```
tctl plugins delete okta
tctl plugins cleanup okta --no-dry-run          # removes Okta-sourced Access Lists + roles
tctl rm saml/okta                               # note: `rm`, not `delete`
tctl get users --format=json | jq -r '.[]|select(.metadata.labels."teleport.dev/origin"=="okta")|.metadata.name'  # then `tctl users rm` each
terraform destroy -auto-approve -input=false    # (in terraform/, with the provider env set)
```

## Guardrails
- Dev cluster + dev Okta org only; app/group sync writes back into Okta.
- Confirm the `terraform plan` before `apply`.
- Never read or revoke the bootstrap SSWS token — it's the user's to manage.

## Notes for maintainers
- Validated against an Okta integrator org: an SSWS token creates the IAM role/resource-set/
  binding through the okta provider (v4.13), and `okta_app_saml.entity_key` yields the public
  metadata URL Teleport can fetch (the provider's `metadata_url` is the SSWS-gated API URL → 403).
- The collision check is agent-side for now; a TF-native `check` block would be cleaner if the
  okta provider exposes list-returning data sources for these types (unverified).
