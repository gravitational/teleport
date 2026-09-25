#!/usr/bin/env bash

# Teleport
# Copyright (C) 2026 Gravitational, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

# Installs the Teleport Connect DEB and RPM packages from
# web/packages/teleterm/build/release on Docker OS images with the oldest
# supported glibc and loads the Electron binary to ensure compatibility.

set -euo pipefail

# All test images ship glibc 2.28, the oldest version supported by Connect.
# The getconf checks below guard against image updates silently raising
# that version.
readonly DEB_DISTROS=(
  "debian/eol:buster-slim"
)
readonly RPM_DISTROS=(
  "oraclelinux:8"
)

shopt -s nullglob
deb_packages=(web/packages/teleterm/build/release/*.deb)
rpm_packages=(web/packages/teleterm/build/release/*.rpm)
shopt -u nullglob

if ((${#deb_packages[@]} != 1 || ${#rpm_packages[@]} != 1)); then
  echo "Expected exactly one Teleport Connect DEB and RPM package" >&2
  exit 1
fi

deb="$(realpath "${deb_packages[0]}")"
rpm="$(realpath "${rpm_packages[0]}")"
readonly deb rpm

echo "============ Pulling images ============"
# Cache images in parallel to speed up the process.
printf '%s\0' "${DEB_DISTROS[@]}" "${RPM_DISTROS[@]}" | xargs -0 -P 10 -n 1 docker pull

for distro in "${DEB_DISTROS[@]}"; do
  echo "============ Installing Teleport Connect DEB on ${distro} ============"
  docker run --rm \
    --env DEBIAN_FRONTEND=noninteractive \
    --volume "${deb}:/tmp/teleport-connect.deb:ro" \
    "${distro}" \
    bash -euxo pipefail -c '
      test "$(getconf GNU_LIBC_VERSION)" = "glibc 2.28"
      apt-get -o Acquire::Check-Valid-Until=false update
      apt-get install --yes --no-install-recommends /tmp/teleport-connect.deb
      ELECTRON_RUN_AS_NODE=1 teleport-connect --version
    '
done

for distro in "${RPM_DISTROS[@]}"; do
  echo "============ Installing Teleport Connect RPM on ${distro} ============"
  docker run --rm \
    --volume "${rpm}:/tmp/teleport-connect.rpm:ro" \
    "${distro}" \
    bash -euxo pipefail -c '
      test "$(getconf GNU_LIBC_VERSION)" = "glibc 2.28"
      dnf install --assumeyes /tmp/teleport-connect.rpm
      ELECTRON_RUN_AS_NODE=1 teleport-connect --version
    '
done
