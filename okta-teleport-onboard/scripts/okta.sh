#!/usr/bin/env bash
# Okta management-API primitives for the onboarding skill.
# Requires: OKTA_ORG, OKTA_SSWS in the environment; curl, jq.
# Confident primitives are marked OK. Payloads marked ⚠️ are best-effort and
# MUST be validated against the target org on first run (read the JSON error
# body and adjust) — do not assume they are final.
set -euo pipefail

: "${OKTA_ORG:?set OKTA_ORG e.g. https://dev-123.okta.com}"
: "${OKTA_SSWS:?set OKTA_SSWS bootstrap token}"

MANAGED_LABEL="teleport.dev/managed-by=okta-onboard-skill"

_okta() { # _okta METHOD PATH [json-body]
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method"
    -H "Authorization: SSWS ${OKTA_SSWS}"
    -H "Accept: application/json" -H "Content-Type: application/json")
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" "${OKTA_ORG}${path}"
}

# --- onboarding state: create helpers record their .id here for teardown ---
: "${OKTA_ONBOARD_STATE:=$HOME/.okta-onboard.state}"

okta::state_set() { # KEY VALUE — idempotent upsert into the state file (no mv: some
  local key="$1" val="$2" rest=""              # environments alias mv/cp to interactive)
  if [[ -f "$OKTA_ONBOARD_STATE" ]]; then
    rest=$(grep -v "^${key}=" "$OKTA_ONBOARD_STATE" || true)
  fi
  { if [[ -n "$rest" ]]; then printf '%s\n' "$rest"; fi
    printf '%s=%s\n' "$key" "$val"; } > "$OKTA_ONBOARD_STATE"
  chmod 600 "$OKTA_ONBOARD_STATE"
}

# Read JSON on stdin, record its .id under KEY, pass the JSON through unchanged.
okta::_emit_id() { # KEY
  local key="$1" json id
  json=$(cat)
  printf '%s' "$json"
  id=$(printf '%s' "$json" | jq -r '.id // empty' 2>/dev/null || true)
  if [[ -n "$id" ]]; then okta::state_set "$key" "$id"; fi
}

# OK — auth check
okta::check_token() {
  curl -sS -o /dev/null -w '%{http_code}\n' \
    -H "Authorization: SSWS ${OKTA_SSWS}" "${OKTA_ORG}/api/v1/apps?limit=1"
}

# OK — custom SAML 2.0 app. groups=GROUP/REGEX(.*), username=EXPRESSION(login).
okta::create_saml_app() { # label acsUrl
  local label="$1" acs="$2"
  _okta POST /api/v1/apps "$(jq -n --arg l "$label" --arg acs "$acs" '{
    label:$l, signOnMode:"SAML_2_0",
    visibility:{autoSubmitToolbar:false, hide:{iOS:false, web:false}},
    settings:{signOn:{
      ssoAcsUrl:$acs, audience:$acs, recipient:$acs, destination:$acs,
      subjectNameIdFormat:"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
      subjectNameIdTemplate:"${user.userName}",
      responseSigned:true, assertionSigned:true,
      signatureAlgorithm:"RSA_SHA256", digestAlgorithm:"SHA256",
      attributeStatements:[
        {type:"EXPRESSION", name:"username",
         namespace:"urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
         values:["user.profile.login"]},
        {type:"GROUP", name:"groups",
         namespace:"urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
         filterType:"REGEX", filterValue:".*"}
      ]}}}')" | okta::_emit_id OKTA_SAML_APP_ID
}

# OK — derive the PUBLIC SAML metadata URL that Teleport fetches unauthenticated.
# NB: it uses the SAML IdP config id (exk...) from the metadata entityID, NOT the
# app id (0oa...) and NOT ._links.metadata.href (that API endpoint is SSWS-gated,
# so Teleport's anonymous fetch of it returns 403).
okta::saml_metadata_url() { # appId
  local idp
  idp=$(_okta GET "/api/v1/apps/$1/sso/saml/metadata" \
    | grep -oE 'entityID="[^"]+"' | head -1 \
    | sed -E 's#entityID="https?://www\.okta\.com/##; s#".*##')
  printf '%s/app/%s/sso/saml/metadata\n' "$OKTA_ORG" "$idp"
}

# OK
okta::find_group()   { _okta GET "/api/v1/groups?q=$(jq -rn --arg q "$1" '$q|@uri')"; }
okta::assign_group() { _okta PUT "/api/v1/apps/$1/groups/$2" '{}'; }         # appId groupId
okta::grant_scope()  { _okta POST "/api/v1/apps/$1/grants" \
  "$(jq -n --arg s "$2" '{scopeId:$s, issuer:$ENV.OKTA_ORG}')"; }           # appId scope

# ⚠️ OAuth API Services (service) app. Verify: application_type/service template,
# jwks binding, and private_key_jwt on THIS org's API version.
okta::create_service_app() { # label jwksUri
  local label="$1" jwks="$2"
  _okta POST /api/v1/apps "$(jq -n --arg l "$label" --arg j "$jwks" '{
    name:"oidc_client", label:$l, signOnMode:"OPENID_CONNECT",
    credentials:{oauthClient:{
      token_endpoint_auth_method:"private_key_jwt", autoKeyRotation:false}},
    settings:{oauthClient:{
      application_type:"service",
      grant_types:["client_credentials"],
      response_types:["token"],
      jwks_uri:$j }}}')" | okta::_emit_id OKTA_SVC_APP_ID
  # DPoP: after creation, PUT the app with
  # settings.oauthClient.dpop_bound_access_tokens=false (field name unverified).
}

# Okta IAM — roles / resource sets / bindings. Validated against integrator org.
okta::create_admin_role()   { _okta POST /api/v1/iam/roles '{
  "label":"Teleport Sync","description":"Teleport Okta integration",
  "permissions":["okta.users.read","okta.users.appAssignment.manage",
    "okta.groups.read","okta.groups.members.manage",
    "okta.apps.read","okta.apps.assignment.manage"]}' | okta::_emit_id OKTA_ROLE_ID; }
okta::create_resource_set() { _okta POST /api/v1/iam/resource-sets "$1" | okta::_emit_id OKTA_RSET_ID; }   # body
# Assign a custom role over a resource set to the service app, via a binding.
# The service-app principal is its OAuth client URL. Args: appId roleId resourceSetId
okta::assign_role()         { _okta POST "/api/v1/iam/resource-sets/$3/bindings" \
  "$(jq -n --arg r "$2" --arg m "${OKTA_ORG}/oauth2/v1/clients/$1" '{role:$r, members:[$m]}')"; }
# Teardown (validated): delete in dependency order binding -> resource set -> role.
okta::delete_binding()      { _okta DELETE "/api/v1/iam/resource-sets/$1/bindings/$2"; }  # rsId roleId
okta::delete_resource_set() { _okta DELETE "/api/v1/iam/resource-sets/$1"; }               # rsId
okta::delete_role()         { _okta DELETE "/api/v1/iam/roles/$1"; }                        # roleId

# OK — deactivate is required before delete
okta::deactivate_app() { _okta POST "/api/v1/apps/$1/lifecycle/deactivate" ''; }
okta::delete_app()     { _okta DELETE "/api/v1/apps/$1"; }
okta::list_tokens()    { _okta GET /api/v1/api-tokens; }
okta::revoke_token()   { _okta DELETE "/api/v1/api-tokens/$1"; }             # tokenId
