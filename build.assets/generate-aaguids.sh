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
#
# Needs the gh CLI, to resolve the dataset revision the generated files record.

set -euo pipefail

REPO="passkeydeveloper/passkey-authenticator-aaguids"
DATASET="combined_aaguid.json"

cd "$(dirname "$0")/.."

dataset="$(mktemp -t combined_aaguid.XXXXXX.json)"
trap 'rm -f "$dataset"' EXIT

# Resolve the revision first and fetch pinned to it, so the commit stamped into the generated files is
# the one they were actually built from rather than whatever main pointed at a moment later.
echo "Resolving the latest $REPO commit touching $DATASET"
read -r commit committed < <(
  gh api "repos/$REPO/commits?path=$DATASET&per_page=1" \
    --jq '.[0] | "\(.sha) \(.commit.committer.date)"'
)

url="https://raw.githubusercontent.com/$REPO/$commit/$DATASET"
echo "Fetching $url"
curl --fail --silent --show-error --location --output "$dataset" "$url"

AAGUID_SOURCE_COMMIT="$commit" AAGUID_SOURCE_DATE="${committed%%T*}" \
  node web/packages/design/src/AuthenticatorIcon/script/generate.mjs "$dataset"

# The emitted TypeScript has to satisfy the repo's formatting check.
pnpm exec oxfmt \
  web/packages/design/src/AuthenticatorIcon/icons.ts \
  web/packages/design/src/AuthenticatorIcon/authenticatorIcons.ts
