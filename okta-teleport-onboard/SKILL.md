---
name: okta-teleport-onboard
description: >-
  Onboard a Teleport cluster to Okta end-to-end (SSO + user sync + app/group
  sync) using the non-deprecated OAuth-for-Okta credential path. Creates the
  Okta SAML app + OAuth API Services app + scoped admin role, and enrolls the
  Teleport Okta integration via the OktaService.CreateIntegration RPC. SCIM is
  out of scope (user sync provides equivalent provisioning).
  PROTOTYPE — dev Okta org + publicly-reachable dev cluster only.
---

# Okta ↔ Teleport onboarding (prototype)

This is a recipe: work through the steps, running each call and adapting to what it
returns. It creates the Okta-side objects and enrolls the Teleport integration. It is
NOT a fixed script — read each response, and if a call fails, inspect the body and fix
the request before moving on.

**DEV ONLY** — dev cluster + dev Okta org. App/group sync is bidirectional and writes
back into Okta.

## Setup

- **Credentials:** `OKTA_ORG` (e.g. `https://dev-123.okta.com`) and `OKTA_SSWS` (an Okta
  admin API token). Take them from the environment, a creds file the user names (source
  it), or ask the user. **Never print or revoke the SSWS token** — it is the user's to
  manage; there is no reason for this skill to do either.
- **Auth on every Okta call:** `-H "Authorization: SSWS $OKTA_SSWS" -H "Accept:
  application/json" -H "Content-Type: application/json"` — *except* the SAML metadata
  fetch, which needs `Accept: application/xml` (Okta rejects JSON there).
- **Derive:**
  - `host` = the Teleport proxy host from `tsh status` (Profile URL), e.g. `dev.teleport.sh`
  - ACS / audience = `https://$host/v1/webapi/saml/acs/okta`
  - JWKS URL = `https://$host/v1/.well-known/jwks-okta`
- **Defaults:** SSO group `Everyone`; Access-List owner = logged-in tsh user; group/app
  import filters `*` (comma-separate to narrow, e.g. `admins,devs,interns`). Note an
  *empty* filter is treated as `*` (all), so "sync no apps" needs a non-matching filter,
  not an empty one.
- **Confirm before mutating:** derive the inputs, show the user what will be created on
  each side (and that app/group sync writes back to Okta), and get an explicit OK before
  the first write (step 2 onward).

## Onboard

**1. Preflight (read-only)**
```
curl -sS -o /dev/null -w '%{http_code}\n' -H "Authorization: SSWS $OKTA_SSWS" \
  "$OKTA_ORG/api/v1/apps?limit=1"                    # expect 200
curl -fsS "https://$host/v1/.well-known/jwks-okta" | jq '.keys|length'   # expect >= 1
```

**2. Create the SAML app (SSO).** `POST $OKTA_ORG/api/v1/apps` with body:
```json
{
  "label": "Teleport ($host)", "signOnMode": "SAML_2_0",
  "visibility": {"autoSubmitToolbar": false, "hide": {"iOS": false, "web": false}},
  "settings": {"signOn": {
    "ssoAcsUrl": "$ACS", "audience": "$ACS", "recipient": "$ACS", "destination": "$ACS",
    "subjectNameIdFormat": "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
    "subjectNameIdTemplate": "${user.userName}",
    "responseSigned": true, "assertionSigned": true,
    "signatureAlgorithm": "RSA_SHA256", "digestAlgorithm": "SHA256",
    "attributeStatements": [
      {"type": "EXPRESSION", "name": "username",
       "namespace": "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
       "values": ["user.profile.login"]},
      {"type": "GROUP", "name": "groups",
       "namespace": "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
       "filterType": "REGEX", "filterValue": ".*"}
    ]}}}
```
Capture `.id` (`0oa…` app id). **Gotchas:** the create fails without the `visibility`
block, and without a `namespace` on the `groups` statement.

**3. Get the public metadata URL** (Teleport fetches it anonymously):
```
curl -sS -H "Authorization: SSWS $OKTA_SSWS" -H "Accept: application/xml" \
  "$OKTA_ORG/api/v1/apps/$APP_ID/sso/saml/metadata" \
  | grep -oE 'entityID="[^"]+"' | head -1 | sed -E 's#.*okta\.com/##; s#".*##'   # -> exk...
```
Metadata URL = `$OKTA_ORG/app/<exk-id>/sso/saml/metadata`. **Gotchas:** the public URL
uses the SAML **IdP config id** (`exk…`) from the entityID — NOT the app id, and NOT
`_links.metadata.href` (that API URL is SSWS-gated → Teleport's anonymous fetch gets 403).

**4. Assign the SSO group.** Resolve the id, then assign:
```
curl -sS <auth> "$OKTA_ORG/api/v1/groups?q=$SSO_GROUP" | jq -r '.[]|select(.profile.name=="'"$SSO_GROUP"'").id'
curl -sS <auth> -X PUT "$OKTA_ORG/api/v1/apps/$APP_ID/groups/$GID" -d '{}'
```

**5. Create the OAuth API-services app** (Teleport's API credential).
`POST $OKTA_ORG/api/v1/apps` with body:
```json
{
  "name": "oidc_client", "label": "Teleport Sync", "signOnMode": "OPENID_CONNECT",
  "credentials": {"oauthClient": {"token_endpoint_auth_method": "private_key_jwt", "autoKeyRotation": false}},
  "settings": {"oauthClient": {"application_type": "service",
    "grant_types": ["client_credentials"], "response_types": ["token"], "jwks_uri": "$JWKS"}}}
```
Capture `.id` = the OAuth **client id**. Confirm `dpop_bound_access_tokens` is false
(Teleport doesn't support DPoP). No client secret is issued — Teleport signs JWTs with
its own Okta CA and the app trusts the JWKS URL.

**6. Grant API scopes.** For each of `okta.users.read okta.users.manage okta.groups.read
okta.groups.manage okta.apps.read okta.apps.manage`:
```
curl -sS <auth> -X POST "$OKTA_ORG/api/v1/apps/$APP_ID/grants" \
  -d '{"scopeId": "okta.users.read", "issuer": "'"$OKTA_ORG"'"}'
```

**7. Scoped admin role + resource set + binding.**
```
# role
POST $OKTA_ORG/api/v1/iam/roles
  {"label":"Teleport Sync","permissions":["okta.users.read","okta.users.appAssignment.manage",
   "okta.groups.read","okta.groups.members.manage","okta.apps.read","okta.apps.assignment.manage"]}
# resource set (all users/groups/apps)
POST $OKTA_ORG/api/v1/iam/resource-sets
  {"label":"Teleport Sync Resources","resources":["$OKTA_ORG/api/v1/users","$OKTA_ORG/api/v1/groups","$OKTA_ORG/api/v1/apps"]}
# bind the role over the resource set to the service app
POST $OKTA_ORG/api/v1/iam/resource-sets/$RSET_ID/bindings
  {"role":"$ROLE_ID","members":["$OKTA_ORG/oauth2/v1/clients/$CLIENT_ID"]}
```
**Gotcha:** assignment is a resource-set **binding** — `POST /apps/{id}/roles` returns
E0000022. The member is the service app's OAuth **client** URL, not the app id.

**8. Enroll the Teleport integration.** There is no curl/`tctl` path for the OAuth
enrollment, so use the one bundled compiled helper — it calls `ValidateClientCredentials`
then `CreateIntegration` (which builds the SAML connector from the metadata URL, the
plugin, and the credential). From the Teleport repo root:
```
GOTOOLCHAIN=local go run ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll \
  -proxy "$host:443" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -validate-only
GOTOOLCHAIN=local go run ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll \
  -proxy "$host:443" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -metadata-url "$META" \
  -owner "<owner>" -group-filter "<glob>" -app-filter "<glob>"
```

**9. Verify.**
```
tctl get plugins/okta --format=json | jq '.[].status.code'   # 1 = RUNNING
tctl get saml/okta                                           # connector exists
# okta-origin users appear after a sync cycle
```

## Teardown

Delete the plugin FIRST so later deletions don't propagate into Okta via bidirectional sync:
```
tctl plugins delete okta                       # stops sync
tctl plugins cleanup okta --no-dry-run         # removes Okta-sourced Access Lists + roles (refuses while plugin active)
tctl rm saml/okta                              # connector is NOT auto-deleted; note `rm`, not `delete`
tctl get users --format=json | jq -r '.[]|select(.metadata.labels."teleport.dev/origin"=="okta")|.metadata.name'  # then `tctl users rm` each
```
Then Okta, in order — binding, resource set, role, then each app (deactivate before delete):
```
DELETE $OKTA_ORG/api/v1/iam/resource-sets/$RSET_ID/bindings/$ROLE_ID
DELETE $OKTA_ORG/api/v1/iam/resource-sets/$RSET_ID
DELETE $OKTA_ORG/api/v1/iam/roles/$ROLE_ID
POST   $OKTA_ORG/api/v1/apps/$APP/lifecycle/deactivate   then   DELETE $OKTA_ORG/api/v1/apps/$APP
```

## Guardrails
- Dev cluster + dev Okta org only; app/group sync writes back into Okta.
- Confirm the plan with the user before the first mutation.
- Never print or revoke the bootstrap SSWS token.
