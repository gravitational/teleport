#!/bin/bash

set -euo pipefail

usage() {
    cat <<EOF >&2
Usage: gen/module.sh <module_name> <remote_system>

Creates the expected Terraform module scaffold at:
  ./teleport/<module_name>/<remote_system>

Examples:
  module.sh discovery aws
  module.sh discovery azure

EOF
}

if [[ $# -lt 2 ]]; then
    usage
    exit 2
fi

MODULE_NAME="${1}"
REMOTE_SYSTEM="${2}"

check_arg() {
case "${1}" in
    "")
        usage
        echo "error: <${2}> must be non-empty" >&2
        exit 2
        ;;
    */* | . | ..)
        usage
        echo "error: <${2}> must be a single path segment" >&2
        exit 2
        ;;
esac
}

check_arg "${MODULE_NAME}" "module_name"
check_arg "${REMOTE_SYSTEM}" "remote_system"

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
module_dir="${script_dir}/../teleport/${MODULE_NAME}/${REMOTE_SYSTEM}"
examples_dir="${module_dir}/examples"
license_file="${module_dir}/LICENSE"
readme_file="${module_dir}/README.md"

mkdir -p "${examples_dir}"

cp "${script_dir}/apache2_license" "${license_file}"

if [ ! -e "${readme_file}" ]; then
    cat >"${readme_file}" <<EOF
## TODO: write a title

TODO: write an overview

## Prerequisites
<!-- lint ignore absolute-docs-links -->
- [Configure Teleport Terraform Provider](https://goteleport.com/docs/configuration/terraform-provider/)
- [Configure ${REMOTE_SYSTEM} Terraform provider](TODO: link the remote provider configuration docs)

## Examples

Refer to the [examples](./examples) for example usage of this module.

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).
EOF
fi
