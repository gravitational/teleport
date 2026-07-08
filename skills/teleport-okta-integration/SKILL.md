---
name: teleport-okta-integration
description: Enroll the Teleport Okta integration end-to-end (SSO + user sync + app/group sync). 
---

# Teleport Okta integration

This skill helps to enroll the Teleport Okta integration with an Okta organisation.

**NOTE:** This is a prototype. Surface any issues to the user so they can be addressed.

A Terraform module (`terraform/`) provisions the Okta objects (SAML app, OAuth API-services app, and scoped admin role, resource set + binding) **and** the Teleport SAML connector.
Then `tctl plugins install okta` is used to install and configure the Teleport Okta plugin using the Terraform outputs.

Work through each of the steps in order. Adapt based on the output of the commands.

## Setup
- **tsh:** Ensure user is logged in to the Teleport cluster (`tsh status`). 
  Both `tctl` and the Terraform **teleport** provider use this profile. 
  If the profile is expired the commands will fail with an auth error, prompt user to re-login.
- **tctl:** Depends on a new `--oauth-client-id` flag for `tctl plugins install okta` which hasn't been released.
  Verify the users version has this flag. If not, direct them to `jward/wip-okta-teleport-onboard-skill` branch to rebuild `build/tctl`.
- **Inline literals:** Resolve every value and use the literal value in each command.
  Never pass shell variables (e.g. `$OKTA_ORG_URL`) or command substitution (e.g. `$(terraform output ...)`).
  Run a `terraform output` command, read the value, then use it literally in the next command.
  Literal commands can be allow-listed and auto-approved; expansions can't. Only exception: the SSWS token: never inline it, source it for `terraform`.
- **Provider install:** If `terraform init` reports *no available releases match `~> 18.0`*, a
  local implicit mirror (`~/.terraform.d/plugins`) may be shadowing the registry with dev builds,
  prompt the user to remove it or bypass it with `TF_CLI_CONFIG_FILE` pointing at a file containing `provider_installation { direct {} }`.
- **Credentials (Okta):** The Okta org URL (e.g. `https://dev-123.okta.com`) and an Okta admin **SSWS token**.
  The org URL is non-secret, put it in `terraform.tfvars` as a literal; the module splits it into the `provider "okta"` block's `org_name`/`base_url`. 
  The token is secret: keep it in the `OKTA_API_TOKEN` environment variable (from the environment, a file the user names, or ask) and **source it only for the `terraform` commands**.
  The okta provider is the only consumer of the API token (`tctl` uses the tsh profile). **Never read, cat, echo, or inline the token.**
- **Derive:** `teleport_domain` if not explicitly provided by user, use proxy host from `tsh status` (e.g. `jwardtest18.cloud.gravitational.io`).
- **Confirm before applying:** Always show the user `terraform plan` and get an OK before `apply`.

## Enroll
Run Terraform in `terraform/`. tfstate and tfvars persist there.

1. **Write `terraform.tfvars`** (via editor, literal values, no shell variables):
   ```
   okta_org_url    = "https://<org>.okta.com"
   teleport_domain = "<proxy host>"
   sso_group       = "Everyone"          # or as requested by user
   ```
   Sync group/app *filters* and the Access-List *owner* are NOT Terraform, they're flags to `tctl plugins install` in a later step.

2. **Plan & apply** creates the Okta objects **and** the Teleport `okta` SAML connector
   (the connector's `entity_descriptor_url` references the Okta app, so Terraform orders it after the app; Teleport fetches the metadata at apply time):
   ```
   terraform init -input=false          # once
   terraform plan -input=false          # show this to the user and get confirmation
   terraform apply -auto-approve -input=false
   ```

3. **Read outputs** (the connector consumes the metadata URL internally; the plugin install needs these two):
   ```
   terraform output -raw oauth_client_id
   terraform output -raw saml_app_id
   ```

4. **Install the Okta plugin** (user + app/group + Access-List sync, authenticating over OAuth):
   Substitute the literal values from Setup and step 3 — no `$VARS`, no `$(...)`:
   ```
   tctl plugins install okta \
     --org https://<org>.okta.com \
     --saml-connector okta \
     --app-id <saml_app_id> \
     --oauth-client-id <oauth_client_id> \
     --owner <owner> \
     -g '<group-filter>' -a '<app-filter>'
   ```
   - `--saml-connector okta` references the connector Terraform just created.
   - `--oauth-client-id` (not `--api-token`) selects the OAuth-for-Okta credential path, the SSWS token is NOT stored on the cluster.
   - `--app-id` is **required**.
   - Sync flags (`--users-sync`/`--appgroup-sync`/`--accesslist-sync`) default on.
   - `--owner` is required because Access-List sync needs >=1 owner.
   - `-g`/`-a` scope app/group sync (`*` = all, or globs; `^pattern$` for full regex). Omitting a filter syncs everything.

5. **Verify:**
   ```
   tctl get plugins/okta --format=json | jq '.[].status.code'   # 1 = RUNNING
   tctl get users --format=json | jq '[.[]|select(.metadata.labels."teleport.dev/origin"=="okta")]|length'  # synced users > 0
   tctl get saml/okta                                           # connector exists
   ```

## Teardown
Delete the plugin FIRST (stops bidirectional sync); then `terraform destroy` removes the SAML connector and all Okta objects:
```
tctl plugins delete okta                        # stops sync (before destroying the connector)
tctl plugins cleanup okta --no-dry-run          # removes Okta-sourced Access Lists + roles
tctl get users --format=json | jq -r '.[]|select(.metadata.labels."teleport.dev/origin"=="okta")|.metadata.name'  # then `tctl users rm` each
terraform destroy -auto-approve -input=false    # removes the connector + all Okta objects (source OKTA_API_TOKEN first; also needs tsh login)
```

## Guardrails
- **Confirm** the `terraform plan` before `apply`.
- **Never** read or revoke the Okta SSWS token.

## Notes for maintainers
- The SSWS token is the only value kept in the environment (`OKTA_API_TOKEN`). 
  The okta provider's `org_name`/`base_url` are derived from `okta_org_url` (tfvars) in `main.tf` locals.
  This keeps commands literal and allow-listable. The token is the only value that must be sourced, and only for `terraform`.
- `tctl plugins install okta --oauth-client-id` is a new flag and writes an OAuth `PluginStaticCredentials` 
  (purpose label `okta-oauth-client-id`, matching the enterprise `OktaService.CreateIntegration` builder) instead of an SSWS API token.
- The connector's `attributes_to_roles` maps `var.sso_group` to the built-in `okta-requester` role. That role must exist. 
  `tctl plugins cleanup` deletes the role, so re-enrolling after a teardown on the same cluster needs it recreated.
