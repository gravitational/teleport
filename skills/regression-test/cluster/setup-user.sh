#!/usr/bin/env bash
# setup-user.sh — register a Teleport user with the mock OIDC IdP and a
# matching per-user OIDC connector. Doesn't perform tsh login itself —
# pair with `sso-login.sh` for the actual login.
#
# Usage: setup-user.sh <project> <user> <roles>
#
#   <project>   project name returned by up.sh
#   <user>      username (e.g. alice)
#   <roles>     comma-separated Teleport role names the user should get
#
# Side effects:
#   - Creates a Teleport user record (so access requests can be filed).
#   - Adds <user> to cluster/mock-oidc-users.json so the mock IdP returns
#     the right group claims for them.
#   - Restarts mock-oidc container (no-op since it re-reads the file on
#     every /token request).
#   - Creates an OIDC connector named <user>-oidc that maps the user's
#     groups claim to the given roles.

set -euo pipefail

PROJECT="${1:?usage: setup-user.sh <project> <user> <roles>}"
USER_NAME="${2:?usage: setup-user.sh <project> <user> <roles>}"
ROLES="${3:?usage: setup-user.sh <project> <user> <roles>}"

cd "$(dirname "$0")"

# Per-project users file (created by up.sh); keeps parallel clusters isolated.
USERS_FILE="mock-oidc-users-${PROJECT}.json"
[[ -f "$USERS_FILE" ]] || echo '{}' > "$USERS_FILE"

# Use the first role as the "groups" claim — claims_to_roles below maps it back.
# (Multiple roles work too; we just need one claim value to anchor the mapping.)
PRIMARY_ROLE="${ROLES%%,*}"

python3 -c "
import json, sys
path = '$USERS_FILE'
with open(path) as f: data = json.load(f)
data['$USER_NAME'] = {'groups': ['$PRIMARY_ROLE']}
with open(path, 'w') as f: json.dump(data, f, indent=2)
"

# Do NOT pre-create a local Teleport user. OIDC sign-in refuses to claim an
# existing local user with the same name ("local user with name X already
# exists. Either change email in OIDC identity or remove local user and try
# again."). The user is created on-the-fly the first time they SSO-log-in.

# Build the claims_to_roles block from the comma-separated roles.
IFS=',' read -ra ROLE_ARRAY <<< "$ROLES"
CLAIMS_TO_ROLES=""
for r in "${ROLE_ARRAY[@]}"; do
  CLAIMS_TO_ROLES+="
    - claim: groups
      value: ${PRIMARY_ROLE}
      roles: [${r}]"
done

docker exec -i "${PROJECT}-auth" tctl create -f - <<EOF
kind: oidc
version: v3
metadata:
  name: ${USER_NAME}-oidc
spec:
  display: "Mock OIDC (${USER_NAME})"
  issuer_url: http://mock-oidc:8080/${USER_NAME}
  client_id: teleport-regression
  client_secret: regression-test-client-secret
  redirect_url: https://teleport-proxy:3080/v1/webapi/oidc/callback
  username_claim: sub
  client_redirect_settings:
    # Permit HTTP callbacks to anything in the regression subnet space — the
    # sso-login.sh script pins each actor to a static IP under 10.99.<slot>.0/24,
    # where <slot> varies per parallel cluster, so allow the whole 10.99.0.0/16.
    insecure_allowed_cidr_ranges: ["10.99.0.0/16"]
  claims_to_roles:${CLAIMS_TO_ROLES}
EOF

echo "user $USER_NAME registered (roles=$ROLES, connector=${USER_NAME}-oidc)" >&2
