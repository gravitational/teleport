#!/usr/bin/env bash
# Orchestrated Okta -> Teleport onboarding. Self-contained: sources the creds file,
# derives its own inputs, and owns ALL happy-path presentation.
#
# SAFETY: nothing is created unless you pass --run explicitly. A bare invocation,
# --help, or any unrecognized argument prints usage and exits WITHOUT mutating.
#
# Prereqs: an authenticated tsh session; creds file with OKTA_ORG + OKTA_SSWS
# (default ~/.okta-onboard.env, override with OKTA_ENV_FILE); skill inside the repo.
#
# Usage:
#   onboard.sh --plan   print the deterministic plan card; create nothing
#   onboard.sh --run    perform the onboarding (creates Okta + Teleport objects)
#   onboard.sh --help   show usage
#
# Inputs are auto-derived; override any via env if needed:
#   PROXY (from `tsh status`), SSO_GROUP (Everyone), ACL_OWNER (logged-in user),
#   GROUP_FILTER (*), APP_FILTER (*)   —  comma-separated filters allowed.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)

usage() { cat <<EOF
Usage: onboard.sh <mode>
  --plan   show the plan; creates nothing
  --run    perform the onboarding (creates Okta + Teleport objects)
  --help   show this message
Nothing is created unless you pass --run explicitly.
EOF
}

# Explicit-mode dispatch. Unknown/empty args NEVER mutate.
MODE=usage; rc=0
case "${1:-}" in
  --plan) MODE=plan ;;
  --run)  MODE=run ;;
  -h|--help|"") MODE=usage ;;
  *) printf 'onboard.sh: unknown argument: %s\n\n' "${1:-}" >&2; rc=2 ;;
esac
[[ "$MODE" == usage ]] && { usage; exit "$rc"; }

# creds + helpers (only reached for --plan / --run)
env_file=${OKTA_ENV_FILE:-$HOME/.okta-onboard.env}
if [[ -z "${OKTA_ORG:-}" || -z "${OKTA_SSWS:-}" ]] && [[ -f "$env_file" ]]; then
  set -a; source "$env_file"; set +a
fi
source "$here/okta.sh"
set +e   # this script checks each result explicitly via || die

say() { printf '\n==> %s\n' "$1"; }
ok()  { printf '    OK  %-14s %s\n' "$1" "${2:-}"; }
er()  { printf '    !!  %-14s %s\n' "$1" "${2:-}"; }   # non-fatal warning
die() { printf '    ERR %s\n' "$1" >&2; exit 1; }
idof(){ jq -r '.id // empty'; }
err(){ jq -r '.errorSummary // .errorCode // "unknown error"'; }

PROXY="${PROXY:-$(tsh status 2>/dev/null | grep -oE 'https://[^ ]+' | head -1 | sed 's#^https://##')}"
[[ -n "$PROXY" ]] || die "could not determine PROXY from tsh (run 'tsh login', or set PROXY)"
host=${PROXY%%:*}
SSO_GROUP=${SSO_GROUP:-Everyone}
GROUP_FILTER=${GROUP_FILTER:-*}
APP_FILTER=${APP_FILTER:-*}
ACL_OWNER=${ACL_OWNER:-$(tsh status 2>/dev/null | awk '/Logged in as:/{print $4}')}
LABEL=${LABEL:-Teleport ($host)}

print_plan() {
  cat <<EOF

Okta -> Teleport onboarding — PLAN (nothing created yet)

  Cluster / proxy    $host  ($PROXY)
  Okta org           $OKTA_ORG
  SSO group(s)       $SSO_GROUP
  Access List owner  $ACL_OWNER
  Group filter       $GROUP_FILTER
  App filter         $APP_FILTER

  Will create — Okta:
    - SAML 2.0 app (SSO) + assign group "$SSO_GROUP"
    - OAuth API Services app (private_key_jwt, JWKS trust)
    - custom IAM admin role + resource set + role binding
  Will create — Teleport:
    - saml/okta connector + Okta integration plugin
    - user sync + app/group sync (bidirectional); synced Access Lists + users

  WARNING: app/group sync is BIDIRECTIONAL and writes back into Okta — membership
           changes in Teleport add/remove users from Okta groups. SCIM is out of scope.
EOF
}

[[ "$MODE" == plan ]] && { print_plan; exit 0; }

# ---- MODE=run: perform the onboarding ----
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

say "Verify"
code=$(tctl get plugins/okta --format=json 2>/dev/null | jq -r '.[].status.code // "?"')
case "$code" in 1) ok "plugin status" "RUNNING";; *) er "plugin status" "code=$code";; esac
tctl get saml/okta >/dev/null 2>&1 && ok "connector" "saml/okta present" || er "connector" "saml/okta MISSING"
uc=$(tctl get users --format=json 2>/dev/null | jq '[.[]|select(.metadata.labels."teleport.dev/origin"=="okta")]|length' 2>/dev/null)
ok "synced users" "${uc:-0} so far (grows over the next sync cycle)"

say "Done — created objects (also saved to $OKTA_ONBOARD_STATE)"
printf '    SAML app      %s\n    Service app   %s\n    Admin role    %s\n    Resource set  %s\n' \
  "$APP_ID" "$CLIENT_ID" "$ROLE_ID" "$RSET_ID"
printf '    Teardown:     %s/cleanup.sh\n' "$here"
