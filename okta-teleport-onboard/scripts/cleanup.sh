#!/usr/bin/env bash
# Teardown for re-runnable prototyping. Removes the Teleport plugin+connector and
# the Okta objects created by this skill. Okta object ids are passed in because
# custom SAML/service apps aren't reliably filterable by label via the search API.
#
# Usage: cleanup.sh <saml-app-id> [service-app-id ...]
set -euo pipefail
source "$(dirname "$0")/okta.sh"

echo "Deleting Teleport Okta plugin and connector..."
tctl plugins delete okta 2>/dev/null || true
tctl delete saml/okta 2>/dev/null || true

for app_id in "$@"; do
  echo "Deactivating+deleting Okta app ${app_id}..."
  okta::deactivate_app "$app_id" >/dev/null 2>&1 || true
  okta::delete_app "$app_id" >/dev/null 2>&1 || true
done

echo "Remove the custom admin role and resource set in the Okta admin UI, or"
echo "extend this script with their ids once the IAM payloads are validated."
