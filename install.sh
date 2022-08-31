#!/bin/bash
# Copyright 2015-2022 Gravitational, Inc.
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

# This script detects the current Linux distribution and installs Teleport
# following the OS's conventions.

# TODO consider using a template and generating this script

# TODO let user know that this will not leave a teleport process running.
# If this is what they wish, they should go to the documentation
# This script only downloads and make sure teleport's files are in the correct place.

set -euo pipefail

install_teleport() {
  echo "installing teleport"

  # exit if not on Linux
  if [[ $(uname) != "Linux" ]]; then
    echo "This script works only for Linux"
    echo "TODO improve this message"
    exit
  fi

  IS_ROOT=""
  SUDO=""
  # check if can run as admin either by running as root or by
  # having 'sudo' or 'doas' available
  if [ "$(id -u)" = 0 ]; then
    # running as root, no need for sudo/doas
    IS_ROOT="YES"
    SUDO=""
  elif type sudo >/dev/null; then
    SUDO="sudo"
  elif type doas >/dev/null; then
    SUDO="doas"
  fi

  if [ -z "$SUDO" ] && [ -z "$IS_ROOT" ]; then
    echo "The installer requires a way to run commands as root."
    echo "Either run this script as root or install sudo/doas."
    exit 1
  fi

  echo $SUDO

  # set curl (curl | wget)
  CURL=""
  if type curl >/dev/null; then
    CURL="curl -fsSL"
  elif type wget >/dev/null; then
    CURL="wget -q -O-" # TODO double check these flags
  fi

  echo $CURL

  OSRELEASE=/etc/os-release
  # detect distro
  if [[ -f "$OSRELEASE" ]]; then
    . $OSRELEASE
  fi

  # select install method based on distribution
  # if ID is ubuntu/debian/(what else?), run apt
  case "$ID" in
  ubuntu)
    echo "ubuntu"
    # require curl
    # TODO determine if it is legacy
    $SUDO $CURL https://apt.releases.teleport.dev/gpg -o /usr/share/keyrings/teleport-archive-keyring.asc
    echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \
    https://apt.releases.teleport.dev/${ID?} ${VERSION_CODENAME?} stable/v10" |
      $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
    $SUDO apt-get update
    $SUDO apt-get install teleport
    ;;

  debian)
    echo "is debian debian!"
    $SUDO $CURL https://apt.releases.teleport.dev/gpg -o /usr/share/keyrings/teleport-archive-keyring.asc
    echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \
    https://apt.releases.teleport.dev/${ID?} ${VERSION_CODENAME?} stable/v10" |
      $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
    $SUDO apt-get update
    $SUDO apt-get install teleport
    ;;
  # if ID is amazon Linux 2/RHEL/(what else?), run yum
  rhel)
    echo "is redhat enterprise linux"
    ;;
  amazn)
    echo "is amazon linux"
    ;;
  *)
    # if ID is other, curl the tarball, unzip and install
    echo "is another distro"
    ;;
  esac

}

install_teleport
which teleport
