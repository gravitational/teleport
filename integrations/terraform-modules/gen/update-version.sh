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
cat > "${script_dir}"/../teleport/container-service/aws/teleport_version_variable.tf <<EOF
variable "teleport_version" {
  default     = "${VERSION}"
  description = <<EOD
The version of Teleport to deploy.
Generally, the version of Teleport should be controlled by using the appropriate version of this module.
This variable is intended for development usage.
EOD
  type        = string
}
EOF
