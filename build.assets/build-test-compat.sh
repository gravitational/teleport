#!/usr/bin/env bash

# Teleport
# Copyright (C) 2023  Gravitational, Inc.
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

# This script runs Teleport binaries using different Docker OS images
# to ensure compatibility. It mainly checks for missing library symbols,
# not shared libraries not installed by default in different OSes
# and ensure that Glibc version is sufficient.

DISTROS=(
  "ubuntu:14.04"
  "ubuntu:16.04"
  "ubuntu:18.04"
  "ubuntu:20.04"
  "ubuntu:22.04"
  "centos:7"
  "centos:8"
  "debian:10"
  "debian:11"
  "debian:12"
  # Distroless Debian fails because of missing libgcc_s.so.1
  # https://github.com/gravitational/teleport/issues/14538
  #"gcr.io/distroless/base-debian12"
  "gcr.io/distroless/cc-debian11"
  "gcr.io/distroless/cc-debian12"
  "amazonlinux:1"
  "amazonlinux:2"
  "amazonlinux:2023"
  "archlinux"
  "oraclelinux:7"
  "oraclelinux:8"
  "oraclelinux:9"
  "fedora:34"
  "fedora:latest"
)

# Global variable to propagate error code from all commands.
# It will be set to non-zero value if any of run commands returns an error.
EXIT_CODE=0

echo "============ Pulling images ============"
# Cache images in parallel to speed up the process.
printf '%s\0' "${DISTROS[@]}" | xargs -0 -P 10 -I{} docker pull {}

for DISTRO in "${DISTROS[@]}";
do
  echo "============ Checking ${DISTRO} ============"

  printf '%s\0' "teleport" "tsh" "tctl" "tbot" | xargs -0 -P 4 -I{} bash -c "docker run -v ${PWD}:/app \"$DISTRO\" \"/app/build/{}\" version"
  EXIT_CODE=$((EXIT_CODE || $?))
done

exit $EXIT_CODE
