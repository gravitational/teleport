#!/bin/bash
#
# Regenerates the WebAuthn authenticator name and logo tables from the community
# passkey-authenticator-aaguids dataset.
#
# Emits:
#   lib/auth/webauthn/aaguid/aaguids.json                  AAGUID -> name, embedded by the Go server
#   web/packages/design/src/AuthenticatorIcon/icons.ts     vendor logo asset exports
#   web/packages/design/src/AuthenticatorIcon/authenticatorIcons.ts   AAGUID -> logo
#   web/packages/design/src/AuthenticatorIcon/assets/      the logos themselves
#
# The image work stays in node: it decodes base64 data URIs, deduplicates them by content hash and
# applies the overrides in AuthenticatorIcon/script/overrides.mjs.

set -euo pipefail

UPSTREAM_URL="https://raw.githubusercontent.com/passkeydeveloper/passkey-authenticator-aaguids/main/combined_aaguid.json"

cd "$(dirname "$0")/.."

dataset="$(mktemp -t combined_aaguid.XXXXXX.json)"
trap 'rm -f "$dataset"' EXIT

echo "Fetching $UPSTREAM_URL"
curl --fail --silent --show-error --location --output "$dataset" "$UPSTREAM_URL"

node web/packages/design/src/AuthenticatorIcon/script/generate.mjs "$dataset"

# The emitted TypeScript has to satisfy the repo's formatting check.
pnpm exec oxfmt \
  web/packages/design/src/AuthenticatorIcon/icons.ts \
  web/packages/design/src/AuthenticatorIcon/authenticatorIcons.ts
