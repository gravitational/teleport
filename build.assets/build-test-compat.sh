#!/usr/bin/env bash
#
# /*
# Copyright 2022 Gravitational, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */
#

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
  "debian:8"
  "debian:9"
  "debian:10"
  "debian:11"
  # Distroless Debian fails because of missing libgcc_s.so.1
  # https://github.com/gravitational/teleport/issues/14538
  #"gcr.io/distroless/base-debian11"
  "gcr.io/distroless/cc"
  "amazonlinux:1"
  "amazonlinux:2"
  "archlinux"
  "oraclelinux:7"
  "oraclelinux:8"
  "fedora:34"
  "fedora:latest"
)

# Global variable to propagate error code from all commands.
# It will be set to non-zero value if any of run commands returns an error.
EXIT_CODE=0

# Run binary in a Docker container and propagate returned exit code.
#
# Arguments:
# $1 - distro name
# $2 - binary to run
function run_docker {
  docker run --rm -v"$(pwd)":/teleport "$1" "${@:2}"
  EXIT_CODE=$((EXIT_CODE || $?))
}

for DISTRO in "${DISTROS[@]}";
do
  echo "============ Checking ${DISTRO} ============"
  docker pull "${DISTRO}"

  run_docker "$DISTRO" /teleport/build/teleport version
  run_docker "$DISTRO" /teleport/build/tsh version
  run_docker "$DISTRO" /teleport/build/tctl version
  run_docker "$DISTRO" /teleport/build/tbot version
done

exit $EXIT_CODE