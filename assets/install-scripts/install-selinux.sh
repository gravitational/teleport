#!/bin/bash
# Copyright 2025 Gravitational, Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script installs the SELinux module for Teleport SSH.

set -euo pipefail

if [[ ! -f "/usr/share/selinux/devel/Makefile" ]]; then
    echo "SELinux Makefile not found, please install selinux-policy-devel"
    exit 1
fi

TELEPORT="teleport"
TELEPORT_ARGS=""

while getopts "c:t:" opt; do
    case "${opt}" in
        c)
            TELEPORT_ARGS="-c ${OPTARG}"
            ;;
        t)
            TELEPORT="${OPTARG}"
            ;;
        *)
            echo "Usage: $0 [-c config_path] [-t teleport_path]"
            exit 2
            ;;
    esac
done

# Write SELinux module source to a temporary directory
WORK_DIR="$(mktemp -d -t teleport-selinux.XXXXXXXX)"
trap 'rm -rf "${WORK_DIR}"' EXIT INT TERM

"${TELEPORT}" selinux-ssh module-source > "${WORK_DIR}/teleport_ssh.te"
"${TELEPORT}" selinux-ssh file-contexts ${TELEPORT_ARGS} > "${WORK_DIR}/teleport_ssh.fc"
DIRS=$(${TELEPORT} selinux-ssh dirs ${TELEPORT_ARGS})

# Build SELinux module
pushd "${WORK_DIR}"
make -f /usr/share/selinux/devel/Makefile teleport_ssh.pp
semodule -i teleport_ssh.pp

# Ensure necessary directories exist and are labeled correctly
while IFS= read -r dir; do
    # shellcheck disable=SC2174
    mkdir -p -m 0750 "${dir}"
    restorecon -rv "${dir}"
done <<< "$DIRS"

popd
rm -rf "${WORK_DIR}"
