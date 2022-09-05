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

# download uses curl or wget to download a teleport binary
download() {
  URL=$1
  TMP_PATH=$2

  echo "Downloading $URL"

  if type curl &>/dev/null; then
    $SUDO $CURL -o $TMP_PATH $URL
  else
    $SUDO $CURL -O $TMP_PATH $URL
  fi
}

install_via_dpkg() {
  TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)

  ARCH_DPKG=$ARCH
  case $ARCH in
  386)
    ARCH_DPKG="i386"
    ;;
  esac

  TELEPORT_FILENAME="teleport_${TELEPORT_VERSION}_${ARCH_DPKG}.deb"
  URL="https://get.gravitational.com/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" $TMP_CHECKSUM

  cd $TEMP_DIR
  $SUDO $SHA_COMMAND -c $TMP_CHECKSUM
  cd -

  echo "Installing Teleport through dpkg"
  $SUDO dpkg -i "${TEMP_DIR}/${TELEPORT_FILENAME}"
}

# installs the latest teleport via yum or dnf, if available
install_via_yum() {
  TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)

  ARCH_RPM=$ARCH
  case $ARCH in
  amd64)
    ARCH_RPM="x86_64"
    ;;
  386)
    ARCH_RPM="i386"
    ;;
  esac

  TELEPORT_FILENAME="teleport-${TELEPORT_VERSION}-1.${ARCH_RPM}.rpm"
  URL="https://get.gravitational.com/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" $TMP_CHECKSUM

  cd $TEMP_DIR
  $SUDO $SHA_COMMAND -c $TMP_CHECKSUM
  cd -

  if type dnf &>/dev/null; then
    echo "Installing Teleport through dnf"
    $SUDO dnf -y install "${TEMP_DIR}/${TELEPORT_FILENAME}"
  else
    echo "Installing Teleport through yum"
    $SUDO yum -y localinstall "${TEMP_DIR}/${TELEPORT_FILENAME}"
  fi

}

# download .tar.gz file via curl/wget, unzip it and run the install sript
install_via_curl() {
  TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)

  TELEPORT_FILENAME="teleport-v$TELEPORT_VERSION-linux-$ARCH-bin.tar.gz"
  URL="https://get.gravitational.com/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" $TMP_CHECKSUM

  cd $TEMP_DIR
  $SUDO $SHA_COMMAND -c $TMP_CHECKSUM
  cd -

  $SUDO tar -xzf "${TEMP_DIR}/${TELEPORT_FILENAME}" -C $TEMP_DIR
  $SUDO "$TEMP_DIR/teleport/install"
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

  # check if can run as admin either by running as root or by
  # having 'sudo' or 'doas' installed
  IS_ROOT=""
  SUDO=""
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

  # require curl/wget
  CURL=""
  if type curl &>/dev/null; then
    CURL="curl -fL"
  elif type wget &>/dev/null; then
    CURL="wget -O-"
  fi
  if [ -z "$CURL" ]; then
    echo "This script requires either curl or wget in order to download files, please install one of them and try again."
    exit 1
  fi

  # require shasum/sha256sum
  SHA_COMMAND=""
  if type shasum &>/dev/null; then
    SHA_COMMAND="shasum -a 256"
  elif type sha256sum &>/dev/null; then
    SHA_COMMAND="sha256sum"
  else
    echo "This script requires sha256sum or shasum to validate the download. Please install it and try again."
    exit 1
  fi

  # detect distro
  OS_RELEASE=/etc/os-release
  ID=""
  ID_LIKE=""
  VERSION_ID=""
  if [[ -f "$OS_RELEASE" ]]; then
    . $OS_RELEASE
  fi

  # detect architecture
  ARCH=""
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
    echo "Your system's architecture isn't oficially supported or couldn't be determined."
    echo "Please refer to the installation guide for more information:"
    echo "https://goteleport.com/docs/installation/"
    exit 1
    ;;
  esac

  # select install method based on distribution
  # if ID is debian derivate, run apt-get
  case "$ID" in
  debian | ubuntu | kali | linuxmint | pop | raspbian | neon | zorin | parrot | elementary)
    install_via_dpkg
    ;;
  # if ID is amazon Linux 2/RHEL/etc, run yum
  centos | rhel | fedora | rocky | almalinux | xenenterprise | ol | scientific | amzn)
    install_via_yum
    ;;
  *)
    # before downloading manually, double check if we didn't miss any
    # debian or rh/fedora derived distros using the ID_LIKE var
    case "$ID_LIKE" in
    ubuntu | debian)
      install_via_dpkg
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

TELEPORT_VERSION=""
if [ $# -ge 1 ] && [ -n "$1" ]; then
  TELEPORT_VERSION=$1
else
  echo "Please provide the version you want to in stall. E.g.: 10.1.9"
  exit 1
fi
install_teleport
