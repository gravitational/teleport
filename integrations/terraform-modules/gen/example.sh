#!/bin/bash

set -euo pipefail

usage() {
    cat <<EOF >&2
Usage: gen/example.sh <module_name> <remote_system> <example_name>

Creates the expected Terraform module example scaffold at:
  ./teleport/<module_name>/<remote_system>/examples/

Examples:
  example.sh discovery aws single-account

EOF
}

if [[ $# -lt 3 ]]; then
    usage
    exit 2
fi

MODULE_NAME="${1}"
REMOTE_SYSTEM="${2}"
EXAMPLE_NAME="${3}"

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
check_arg "${EXAMPLE_NAME}" "example_name"

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
module_dir="${script_dir}/../teleport/${MODULE_NAME}/${REMOTE_SYSTEM}"
example_dir="${module_dir}/examples/${EXAMPLE_NAME}"
docs_title_file="${example_dir}/docs_title"
docs_description_file="${example_dir}/docs_description"
readme_file="${example_dir}/README.md"

mkdir -p "${example_dir}"

touch "${example_dir}/"{main,outputs,variables,versions}.tf

if [ ! -e "${docs_title_file}" ]; then
    echo "TODO: write a short, single line title for docs gen" > "${docs_title_file}"
fi

if [ ! -e "${docs_description_file}" ]; then
    echo "TODO: write a short, single line description for docs gen" > "${docs_description_file}"
fi

if [ ! -e "${readme_file}" ]; then
    cat >"${readme_file}" <<EOF
## TODO: write a title

TODO: write an overview

EOF
fi
