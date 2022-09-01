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

# require_curl checks if $CURL is set and exits with error
# if it is empty
require_curl() {
  if [ -z "$CURL" ]; then
    echo "This script requires either curl or wget in order to download files"
    exit 1
  fi
}

install_via_apt() {
  echo "Installing Teleport through apt-get"
  require_curl

  echo "Downloading Teleport's PGP public key..."
  $SUDO $CURL https://apt.releases.teleport.dev/gpg | $SUDO tee /usr/share/keyrings/teleport-archive-keyring.asc >/dev/null

  SRC="deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] https://deb.releases.teleport.dev/ stable main"
  echo "$SRC" | $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
  
  $SUDO apt-get update
  $SUDO apt-get install teleport
}

# install_via_yum installs latest teleport via yum or dnf
install_via_yum() {
  if type dnf &>/dev/null; then
    echo "Installing Teleport through dnf"
    $SUDO dnf config-manager --add-repo https://rpm.releases.teleport.dev/teleport.repo
    $SUDO dnf install teleport -y
  else
    echo "Installing Teleport through yum"
    $SUDO yum-config-manager --add-repo https://rpm.releases.teleport.dev/teleport.repo
    $SUDO yum install teleport -y
  fi

}

install_via_curl() {
  require_curl

  # TODO save to a /tmp file instead
  $CURL https://get.gravitational.com/teleport-v10.1.4-linux-arm-bin.tar.gz.sha256
  $CURL -O https://get.gravitational.com/teleport-v10.1.4-linux-arm-bin.tar.gz
  sha256sum -a 256 teleport-v10.1.4-linux-arm-bin.tar.gz
  tar -xzf teleport-v10.1.4-linux-arm-bin.tar.gz
  $SUDO ./teleport/install
}

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
  elif type sudo &>/dev/null; then
    SUDO="sudo"
  elif type doas &>/dev/null; then
    SUDO="doas"
  fi

  if [ -z "$SUDO" ] && [ -z "$IS_ROOT" ]; then
    echo "The installer requires a way to run commands as root."
    echo "Either run this script as root or install sudo/doas."
    exit 1
  fi

  # set curl (curl | wget)
  CURL=""
  if type curl &>/dev/null; then
    CURL="curl -fsSL"
  elif type wget &>/dev/null; then
    CURL="wget -q -O-" # TODO double check these flags
  fi

  # detect distro
  OSRELEASE=/etc/os-release
  ID=""
  ID_LIKE=""
  if [[ -f "$OSRELEASE" ]]; then
    . $OSRELEASE
  fi

  # select install method based on distribution
  # if ID is ubuntu/debian/(what else?), run apt
  case "$ID" in
  debian | ubuntu | kali | linuxmint | pop | raspbian | neon | zorin | parrot | elementary)
    install_via_apt
    ;;
  # if ID is amazon Linux 2/RHEL/(what else?), run yum
  centos | rhel | fedora | rocky | almalinux | xenenterprise | ol | scientific) # todo add amazn back
    install_via_yum
    ;;
  *)
    # beforing downloading manually, check if we didn't miss any debian or rh/fedora derived distros
    case "$ID_LIKE" in
    ubuntu | debian)
      install_via_apt
      ;;
    "rhel fedora" | fedora | "centos rhel fedora")
      install_via_yum
      ;;
    *)
      # if ID and ID_LIKE didn't return a supported distro, download through curl
      echo "There is no oficially supported package to $ID distribution. Downloading and installying Teleport via curl"
      install_via_curl
      ;;
    esac
    ;;
  esac

  echo "$(teleport version) installed successfully."
}

install_teleport
