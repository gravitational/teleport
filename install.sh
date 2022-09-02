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
# through it's package manager when available, or downloading a tarball otherwise.

set -euo pipefail

# require_curl checks if $CURL is set and exits with error
# if it is empty
require_curl() {
  if [ -z "$CURL" ]; then
    echo "This script requires either curl or wget in order to download files, please install one of these and try again."
    exit 1
  fi
}

# add Teleport repository keys
add_apt_key() {
  GPG_URL="https://apt.releases.teleport.dev/gpg"
  ASC_URL="https://deb.releases.teleport.dev/teleport-pubkey.asc"
  KEY_URL=$GPG_URL

  # check if we it is necessary to use legacy .asc key
  echo "version_id is $VERSION_ID"
  case "$ID" in
  ubuntu | pop | neon | zorin)
    if ! expr "$VERSION_ID" : "2.*" >/dev/null; then
      KEY_URL=$ASC_URL
    fi
    ;;
  debian | raspbian)
    if [ "$VERSION_ID" -lt 11 ]; then
      KEY_URL=$ASC_URL
    fi
    ;;
  linuxmint | parrot)
    if [ "$VERSION_ID" -lt 5 ]; then
      KEY_URL=$ASC_URL
    fi
    ;;
  elementary)
    if [ "$VERSION_ID" -lt 6 ]; then
      KEY_URL=$ASC_URL
    fi
    ;;
  esac

  echo "Downloading Teleport's public key..."
  $SUDO $CURL $KEY_URL | $SUDO tee /usr/share/keyrings/teleport-archive-keyring.asc >/dev/null
  $SUDO apt-get update

  SRC="deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] https://deb.releases.teleport.dev/ stable main"
  echo "$SRC" | $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
}

install_via_apt() {
  echo "Installing Teleport through apt-get"
  require_curl
  add_apt_key

  $SUDO apt-get update
  $SUDO apt-get install teleport
}

# installs the latest teleport via yum or dnf, if available
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

# download .tar.gz file via curl/wget, unzip it and run the install sript
install_via_curl() {
  require_curl
  ARCH=""
  TELEPORT_VERSION="v10.1.9" # TODO

  # detect architecture
  case $(uname -m) in
  x86_64)
    ARCH="amd64"
    ;;
  i386)
    ARCH="386"
    ;;
  armv7l)
    ARCH="arm"
    ;;
  aarch64)
    ARCH="arm64"
    ;;
  **)
    echo "Your system's architecture couldn't be determined. Please refer to the installation guide for more information:"
    echo "https://goteleport.com/docs/installation/"
    exit 1
    ;;
  esac

  TELEPORT_FILE_NAME="teleport-$TELEPORT_VERSION-linux-$ARCH-bin.tar.gz"
  TMP_PATH="/tmp/$TELEPORT_FILE_NAME"
  TMP_CHECKSUM="/tmp/$TELEPORT_FILE_NAME.sha256"

  echo "Downloading checksum..."
  $CURL "https://get.gravitational.com/$TELEPORT_FILE_NAME.sha256" | $SUDO tee $TMP_CHECKSUM >/dev/null

  echo "Downloading Teleport..."
  if type curl &>/dev/null; then
    $SUDO $CURL -o $TMP_PATH "https://get.gravitational.com/$TELEPORT_FILE_NAME"
  else
    $SUDO $CURL -O $TMP_PATH "https://get.gravitational.com/$TELEPORT_FILE_NAME"
  fi

  # TODO install sha256sum if not present
  # TODO fail if not match
  cd /tmp
  $SUDO sha256sum -c $TMP_CHECKSUM
  cd -

  $SUDO tar -xzf $TMP_PATH -C /tmp
  $SUDO /tmp/teleport/install
}

# wrap script in a function so a partial download
# doesn't execute
install_teleport() {
  echo "Teleport Installation Script"

  # exit if not on Linux
  if [[ $(uname) != "Linux" ]]; then
    echo "This script works only for Linux, please go to the downloads page to find a link to your operating system:"
    echo "https://goteleport.com/download/"
    exit 1
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
    CURL="curl -fL"
  elif type wget &>/dev/null; then
    CURL="wget -O-"
  fi

  # detect distro
  OSRELEASE=/etc/os-release
  ID=""
  ID_LIKE=""
  VERSION_ID=""
  if [[ -f "$OSRELEASE" ]]; then
    . $OSRELEASE
  fi

  # select install method based on distribution
  # if ID is debian derivate, run apt-get
  case "$ID" in
  debian | ubuntu | kali | linuxmint | pop | raspbian | neon | zorin | parrot | elementary)
    install_via_apt
    ;;
  # if ID is amazon Linux 2/RHEL/etc, run yum
  centos | rhel | fedora | rocky | almalinux | xenenterprise | ol | scientific | amzn)
    install_via_yum
    ;;
  *)
    # beforing downloading manually, double check if we didn't miss any debian or rh/fedora derived distros
    case "$ID_LIKE" in
    ubuntu | debian)
      install_via_apt
      ;;
    "rhel fedora" | fedora | "centos rhel fedora")
      install_via_yum
      ;;
    *)
      # if ID and ID_LIKE didn't return a supported distro, download through curl
      echo "There is no oficially supported package to your package manager. Downloading and installing Teleport via CURL."
      install_via_curl
      ;;
    esac
    ;;
  esac

  echo "$(teleport version) installed successfully!"
}

install_teleport
