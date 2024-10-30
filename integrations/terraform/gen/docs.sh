#!/bin/bash

# If someone runs a debug GitHub Actions task
if [ -n "$RUNNER_DEBUG" ]; then
  set -x
fi

set -euo pipefail

info() {
    printf "\e[1;32m%s\e[0m\n" "$1"
}
error() {
    printf "\e[1;31m%s\e[0m\n" "$1"
    exit 1
}

VERSION="$1"

if [ -z "$VERSION" ]; then
  error "Version parameter is required"
fi

# Generator is installed by the "docs" Makefile target dependencies
GENERATOR="$PWD/bin/tfplugindocs"
if ! command -v "$GENERATOR" &> /dev/null
then
    error "can't find the generator: $GENERATOR"
fi

TFDIR="$(pwd)"
DOCSDIR="$(pwd)/../../docs/pages/reference/terraform-provider"
TMPDIR="$(mktemp -d)"

info "Generating provider's schema"

cd "$TMPDIR"
cat > main.tf << EOF
terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "= $VERSION"
    }
  }
}
EOF

terraform init
terraform providers schema -json > schema.json

info "Rendering markdown files"

$GENERATOR generate \
  --providers-schema "$TMPDIR/schema.json" \
  --provider-name "terraform.releases.teleport.dev/gravitational/teleport" \
  --rendered-provider-name "teleport" \
  --rendered-website-dir="$TMPDIR/docs" \
  --website-source-dir="$TFDIR/templates" \
  --provider-dir "$TFDIR" \
  --examples-dir="$TFDIR/examples" \
  --website-temp-dir="$TMPDIR/temp" \
  --hidden-attributes="id,kind,metadata.namespace,metadata.revision"

info "Converting .md files to .mdx"

cd "$TMPDIR/docs"
find . -iname '*.md' -type f -exec sh -c 'i="$1"; mv "$i" "${i%.md}.mdx"' shell {} \;
# renaming the resources and data-sources indexes because the names were reserved by the generator
mv "$TMPDIR/docs/resources-index.mdx" "$TMPDIR/docs/resources.mdx"
mv "$TMPDIR/docs/data-sources-index.mdx" "$TMPDIR/docs/data-sources.mdx"

info "Copying generated documentation into the teleport docs directory"

# Removing the apex terraform.mdx
rm -rf "$DOCSDIR" "$DOCSDIR.mdx"
cp -r "$TMPDIR/docs" "$DOCSDIR"
# unpacking the index to the apex terraform.mdx
mv "$DOCSDIR/index.mdx" "$DOCSDIR.mdx"

info "TF documentation successfully generated"