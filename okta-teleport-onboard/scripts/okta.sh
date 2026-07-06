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
      ]}}}')"
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
      jwks_uri:$j }}}')"
  # DPoP: after creation, PUT the app with
  # settings.oauthClient.dpop_bound_access_tokens=false (field name unverified).
}

# Okta IAM — roles / resource sets / bindings. Validated against integrator org.
okta::create_admin_role()   { _okta POST /api/v1/iam/roles '{
  "label":"Teleport Sync","description":"Teleport Okta integration",
  "permissions":["okta.users.read","okta.users.appAssignment.manage",
    "okta.groups.read","okta.groups.members.manage",
    "okta.apps.read","okta.apps.assignment.manage"]}'; }
okta::create_resource_set() { _okta POST /api/v1/iam/resource-sets "$1"; }   # body
# Assign a custom role over a resource set to the service app, via a binding.
# The service-app principal is its OAuth client URL. Args: appId roleId resourceSetId
okta::assign_role()         { _okta POST "/api/v1/iam/resource-sets/$3/bindings" \
  "$(jq -n --arg r "$2" --arg m "${OKTA_ORG}/oauth2/v1/clients/$1" '{role:$r, members:[$m]}')"; }

# OK — deactivate is required before delete
okta::deactivate_app() { _okta POST "/api/v1/apps/$1/lifecycle/deactivate" ''; }
okta::delete_app()     { _okta DELETE "/api/v1/apps/$1"; }
okta::list_tokens()    { _okta GET /api/v1/api-tokens; }
okta::revoke_token()   { _okta DELETE "/api/v1/api-tokens/$1"; }             # tokenId
