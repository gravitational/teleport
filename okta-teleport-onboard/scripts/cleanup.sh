#!/usr/bin/env bash
# Offboarding / teardown. Re-runnable and best-effort (continues past already-
# deleted resources). ORDER MATTERS: the Teleport plugin is deleted FIRST so that
# removing Access Lists and Okta objects does not propagate back into Okta via
# bidirectional sync.
#
# Requires OKTA_ORG + OKTA_SSWS in the environment (Okta-side deletes) and an
# authenticated tsh/tctl session (Teleport-side deletes).
#
# Usage: cleanup.sh <saml-app-id> <service-app-id> <role-id> <resource-set-id>
#   The Teleport-side teardown needs no ids (it operates on plugin "okta" and
#   okta-origin resources); the ids identify the Okta objects this skill created.
source "$(dirname "$0")/okta.sh"
set +e   # teardown is best-effort; do not abort on an already-gone resource

SAML_APP="${1:-}"; SVC_APP="${2:-}"; ROLE_ID="${3:-}"; RSET_ID="${4:-}"

echo "== Teleport: delete plugin (stops bidirectional sync first) =="
tctl plugins delete okta

echo "== Teleport: clean up Okta-sourced Access Lists + roles =="
tctl plugins cleanup okta --no-dry-run

echo "== Teleport: delete SAML connector =="
tctl rm saml/okta

echo "== Teleport: delete okta-origin users =="
tctl get users --format=json 2>/dev/null \
  | jq -r '.[] | select(.metadata.labels."teleport.dev/origin"=="okta") | .metadata.name' \
  | while read -r u; do tctl users rm "$u"; done

echo "== Okta: remove binding -> resource set -> role =="
[[ -n "$RSET_ID" && -n "$ROLE_ID" ]] && okta::delete_binding "$RSET_ID" "$ROLE_ID" >/dev/null 2>&1
[[ -n "$RSET_ID" ]] && okta::delete_resource_set "$RSET_ID" >/dev/null 2>&1
[[ -n "$ROLE_ID" ]] && okta::delete_role "$ROLE_ID" >/dev/null 2>&1

echo "== Okta: deactivate + delete apps =="
for app in "$SVC_APP" "$SAML_APP"; do
  [[ -n "$app" ]] || continue
  okta::deactivate_app "$app" >/dev/null 2>&1
  okta::delete_app "$app" >/dev/null 2>&1
done

echo "Teardown complete. Revoke the bootstrap SSWS token (okta::revoke_token) when done."
