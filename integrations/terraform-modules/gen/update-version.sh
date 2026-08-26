#!/bin/bash

set -euo pipefail

usage() {
    cat <<EOF >&2
Usage: update-version.sh <version>

Updates source to match the current version.

Examples:
  version.sh 19.0.0

EOF
}

if [[ $# -ne 1 ]]; then
    usage
    exit 2
fi

VERSION="${1}"

if [[ -z "${VERSION}" ]]; then
    usage
    echo "error: <version> must be non-empty" >&2
    exit 2
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
modules=(
    "teleport/container-service/aws"
    "teleport/db-agent/aws"
)

for module in "${modules[@]}"; do
    cat > "${script_dir}/../${module}/release_version.tf" <<EOF
# This file is auto-generated and should not be edited directly.
# Instead, update integrations/terraform-modules/gen/update-version.sh and then
# run make -C integrations/terraform-modules update-version

locals {
  # module_version is updated by gen/update-version.sh for each release.
  # tflint-ignore: terraform_unused_declarations
  module_version = "${VERSION}"
}
EOF
done
