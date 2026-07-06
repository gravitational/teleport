#!/usr/bin/env bash
# Orchestrated Okta -> Teleport onboarding. ALL mutation logic lives here so the
# driving agent just derives inputs, confirms once, runs this, and relays output.
# Emits clean, structured progress; exits non-zero with a clear line on failure.
#
# Prereqs: OKTA_ORG + OKTA_SSWS in the environment (source ~/.okta-onboard.env),
# an authenticated tsh session, and this skill living inside the Teleport repo.
#
# Env inputs (defaults in parens):
#   PROXY         Teleport proxy host[:port]                     (required)
#   SSO_GROUP     Okta group granted SSO                         (Everyone)
#   ACL_OWNER     Access List default owner                      (logged-in tsh user)
#   GROUP_FILTER  Okta group import filter                       (*)
#   APP_FILTER    Okta app import filter                         (*)
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)   # <repo>/.claude/skills/okta-teleport-onboard/scripts
source "$here/okta.sh"
set +e   # this script checks each result explicitly via || die

: "${PROXY:?set PROXY to the Teleport proxy host, e.g. dev.teleport.sh:443}"
host=${PROXY%%:*}
SSO_GROUP=${SSO_GROUP:-Everyone}
GROUP_FILTER=${GROUP_FILTER:-*}
APP_FILTER=${APP_FILTER:-*}
ACL_OWNER=${ACL_OWNER:-$(tsh status 2>/dev/null | awk '/Logged in as:/{print $4}')}
LABEL=${LABEL:-Teleport ($host)}

say() { printf '\n==> %s\n' "$1"; }
ok()  { printf '    OK  %-14s %s\n' "$1" "${2:-}"; }
die() { printf '    ERR %s\n' "$1" >&2; exit 1; }
idof(){ jq -r '.id // empty'; }
err(){ jq -r '.errorSummary // .errorCode // "unknown error"'; }

say "Preflight"
[[ "$(okta::check_token)" == 200 ]] || die "Okta token rejected (check OKTA_SSWS)"
ok "okta token" "valid"
keys=$(curl -fsS "https://$host/v1/.well-known/jwks-okta" 2>/dev/null | jq '.keys|length' 2>/dev/null || echo 0)
[[ "${keys:-0}" -ge 1 ]] || die "JWKS endpoint has no keys at https://$host/v1/.well-known/jwks-okta"
ok "jwks endpoint" "$keys key(s)"
[[ -n "$ACL_OWNER" ]] || die "could not determine ACL owner; set ACL_OWNER"

say "SSO: SAML application"
resp=$(okta::create_saml_app "$LABEL" "https://$host/v1/webapi/saml/acs/okta")
APP_ID=$(idof <<<"$resp"); [[ -n "$APP_ID" ]] || die "SAML app: $(err <<<"$resp")"
ok "saml app" "$APP_ID"
gid=$(okta::find_group "$SSO_GROUP" | jq -r --arg n "$SSO_GROUP" '.[]|select(.profile.name==$n).id' | head -1)
[[ -n "$gid" ]] || die "Okta group not found: $SSO_GROUP"
okta::assign_group "$APP_ID" "$gid" >/dev/null
ok "assigned group" "$SSO_GROUP"
META=$(okta::saml_metadata_url "$APP_ID")
[[ "$META" == */app/exk*/sso/saml/metadata ]] || die "metadata URL malformed: '$META'"
ok "metadata url" "$META"

say "API access: OAuth service app"
resp=$(okta::create_service_app "$LABEL Sync" "https://$host/v1/.well-known/jwks-okta")
CLIENT_ID=$(idof <<<"$resp"); [[ -n "$CLIENT_ID" ]] || die "service app: $(err <<<"$resp")"
ok "service app" "$CLIENT_ID"
for s in okta.users.read okta.users.manage okta.groups.read okta.groups.manage okta.apps.read okta.apps.manage; do
  okta::grant_scope "$CLIENT_ID" "$s" >/dev/null || die "granting scope $s"
done
ok "granted scopes" "6"

say "Scoped admin role"
ROLE_ID=$(okta::create_admin_role | idof); [[ -n "$ROLE_ID" ]] || die "admin role create failed"
ok "admin role" "$ROLE_ID"
rs=$(jq -n --arg o "$OKTA_ORG" '{label:"Teleport Sync Resources",description:"Teleport Okta integration",
  resources:[($o+"/api/v1/users"),($o+"/api/v1/groups"),($o+"/api/v1/apps")]}')
RSET_ID=$(okta::create_resource_set "$rs" | idof); [[ -n "$RSET_ID" ]] || die "resource set create failed"
ok "resource set" "$RSET_ID"
okta::assign_role "$CLIENT_ID" "$ROLE_ID" "$RSET_ID" >/dev/null
ok "bound role" "to service app"

say "Teleport enrollment"
run_enroll() { ( cd "$repo" && GOTOOLCHAIN=local go run \
  ./.claude/skills/okta-teleport-onboard/scripts/tp-enroll "$@" ); }
run_enroll -proxy "$PROXY" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -validate-only \
  || die "credential validation failed (check JWKS trust / DPoP on the service app)"
ok "credentials" "validated"
run_enroll -proxy "$PROXY" -org "$OKTA_ORG" -oauth-id "$CLIENT_ID" -metadata-url "$META" \
  -owner "$ACL_OWNER" -group-filter "$GROUP_FILTER" -app-filter "$APP_FILTER" \
  || die "enrollment RPC failed"
ok "integration" "plugin=okta connector=okta"

say "Done — created objects (also saved to $OKTA_ONBOARD_STATE)"
printf '    SAML app      %s\n    Service app   %s\n    Admin role    %s\n    Resource set  %s\n' \
  "$APP_ID" "$CLIENT_ID" "$ROLE_ID" "$RSET_ID"
printf '    Teardown:     %s/cleanup.sh\n' "$here"
