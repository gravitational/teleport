#!/usr/bin/env bash
# Offboarding / teardown — self-reporting and re-runnable. Reports the outcome of
# each deletion (deleted / already gone / error) so the caller needs NO follow-up
# verification. ORDER MATTERS: delete the Teleport plugin FIRST so that removing
# Access Lists and Okta objects does not propagate back into Okta via bidirectional
# sync.
#
# Requires OKTA_ORG + OKTA_SSWS in env and an authenticated tsh session.
# Usage: cleanup.sh [saml-app-id svc-app-id role-id resource-set-id]
#   IDs default to those onboarding recorded in $OKTA_ONBOARD_STATE.
source "$(dirname "$0")/okta.sh"
set +e
[[ -f "$OKTA_ONBOARD_STATE" ]] && source "$OKTA_ONBOARD_STATE"
SAML_APP="${1:-${OKTA_SAML_APP_ID:-}}"; SVC_APP="${2:-${OKTA_SVC_APP_ID:-}}"
ROLE_ID="${3:-${OKTA_ROLE_ID:-}}";       RSET_ID="${4:-${OKTA_RSET_ID:-}}"

say() { printf '\n==> %s\n' "$1"; }
ok()  { printf '    OK  %-16s %s\n' "$1" "${2:-}"; }
er()  { printf '    ERR %-16s %s\n' "$1" "${2:-}"; }
okta_code() { curl -sS -o /dev/null -w '%{http_code}' -X "$1" \
  -H "Authorization: SSWS $OKTA_SSWS" "$OKTA_ORG$2"; }
report() { case "$1" in 2*) ok "$2" "deleted";; 404) ok "$2" "already gone";; *) er "$2" "HTTP $1";; esac; }

say "Teleport"
if tctl plugins delete okta >/dev/null 2>&1; then ok "plugin" "deleted / absent"; else er "plugin" "delete failed"; fi
out=$(tctl plugins cleanup okta --no-dry-run 2>&1)
if grep -q "Successfully cleaned up" <<<"$out"; then ok "access lists" "$(grep -c 'Kind:' <<<"$out") resource(s) removed"
elif grep -qE "currently active|doesn't need" <<<"$out"; then ok "access lists" "nothing to clean"
else er "access lists" "$(tail -1 <<<"$out")"; fi
if tctl rm saml/okta >/dev/null 2>&1; then ok "saml connector" "deleted"; else ok "saml connector" "absent"; fi
uc=0
for u in $(tctl get users --format=json 2>/dev/null \
    | jq -r '.[]|select(.metadata.labels."teleport.dev/origin"=="okta")|.metadata.name'); do
  tctl users rm "$u" >/dev/null 2>&1 && uc=$((uc+1))
done
ok "okta users" "$uc deleted"

say "Okta"
if [[ -n "$RSET_ID" && -n "$ROLE_ID" ]]; then
  report "$(okta_code DELETE "/api/v1/iam/resource-sets/$RSET_ID/bindings/$ROLE_ID")" "role binding"; fi
[[ -n "$RSET_ID" ]] && report "$(okta_code DELETE "/api/v1/iam/resource-sets/$RSET_ID")" "resource set"
[[ -n "$ROLE_ID" ]] && report "$(okta_code DELETE "/api/v1/iam/roles/$ROLE_ID")" "admin role"
for pair in "service app:$SVC_APP" "saml app:$SAML_APP"; do
  app=${pair##*:}; [[ -n "$app" ]] || continue
  okta_code POST "/api/v1/apps/$app/lifecycle/deactivate" >/dev/null
  report "$(okta_code DELETE "/api/v1/apps/$app")" "${pair%%:*}"
done

rm -f "$OKTA_ONBOARD_STATE"
say "Done — the bootstrap SSWS token is left untouched (user-managed)."
