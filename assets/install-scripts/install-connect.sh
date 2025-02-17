#!/bin/bash
# Copyright 2022 Gravitational, Inc
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

# This script detects the current Linux distribution and installs Teleport Connect
# through its package manager, if supported, or downloading a tarball otherwise.
# We'll download Teleport from the official website and checksum it to make sure it was properly
# downloaded before executing.

# The script is wrapped inside a function to protect against the connection being interrupted
# in the middle of the stream.

# For more download options, head to https://goteleport.com/download/

set -euo pipefail

# download uses curl or wget to download a teleport connect binary
download() {
  URL=$1
  TMP_PATH=$2

  echo "Downloading $URL"
  if type curl &>/dev/null; then
    set -x
    $SUDO $CURL -o "$TMP_PATH" "$URL"
  else
    set -x
    $SUDO $CURL -O "$TMP_PATH" "$URL"
  fi
  set +x
}

install_via_apt_get() {
  echo "Installing Teleport Connect v$TELEPORT_VERSION via apt-get"
  add_apt_key
  set -x
  $SUDO apt-get install -y teleport-connect=$TELEPORT_VERSION
  set +x
}

add_apt_key() {
  APT_REPO_ID=$ID
  APT_REPO_VERSION_CODENAME=$VERSION_CODENAME
  IS_LEGACY=0

  # check if we must use legacy .asc key
  case "$ID" in
  ubuntu | pop | neon | zorin)
    if ! expr "$VERSION_ID" : "2.*" >/dev/null; then
      IS_LEGACY=1
    fi
    ;;
  debian | raspbian)
    if [ "$VERSION_ID" -lt 11 ]; then
      IS_LEGACY=1
    fi
    ;;
  linuxmint | parrot)
    if [ "$VERSION_ID" -lt 5 ]; then
      IS_LEGACY=1
    fi
    ;;
  elementary)
    if [ "$VERSION_ID" -lt 6 ]; then
      IS_LEGACY=1
    fi
    ;;
  kali)
    YEAR="$(echo "$VERSION_ID" | cut -f1 -d.)"
    if [ "$YEAR" -lt 2021 ]; then
      IS_LEGACY=1
    fi
    ;;
  esac

  if [[ "$IS_LEGACY" == 0 ]]; then
    # set APT_REPO_ID if necessary
    case "$ID" in
    linuxmint | kali | elementary | pop | raspbian | neon | zorin | parrot)
      APT_REPO_ID=$ID_LIKE
      ;;
    esac

    # set APT_REPO_VERSION_CODENAME if necessary
    case "$ID" in
    linuxmint | elementary | pop | neon | zorin)
      APT_REPO_VERSION_CODENAME=$UBUNTU_CODENAME
      ;;
    kali)
      APT_REPO_VERSION_CODENAME="bullseye"
      ;;
    parrot)
      APT_REPO_VERSION_CODENAME="buster"
      ;;
    esac
  fi

  echo "Downloading Teleport's PGP public key..."
  TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)
  MAJOR=$(echo "$TELEPORT_VERSION" | cut -f1 -d.)
  TELEPORT_REPO=""

  if [[ "$IS_LEGACY" == 1 ]]; then
    if ! type gpg >/dev/null; then
      echo "Installing gnupg"
      set -x
      $SUDO apt-get update
      $SUDO apt-get install -y gnupg
      set +x
    fi
    TMP_KEY="$TEMP_DIR/teleport-pubkey.asc"
    download "https://deb.releases.teleport.dev/teleport-pubkey.asc" "$TMP_KEY"
    set -x
    cat $TMP_KEY | $SUDO apt-key add -
    set +x
    TELEPORT_REPO="deb https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} stable/v${MAJOR}"
  else
    TMP_KEY="$TEMP_DIR/teleport-pubkey.gpg"
    download "https://apt.releases.teleport.dev/gpg" "$TMP_KEY"
    set -x
    $SUDO mkdir -p /etc/apt/keyrings
    cat $TMP_KEY | $SUDO tee /etc/apt/keyrings/teleport-archive-keyring.asc >/dev/null
    set +x
    TELEPORT_REPO="deb [signed-by=/etc/apt/keyrings/teleport-archive-keyring.asc]  https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} stable/v${MAJOR}"
  fi

  set -x
  echo "$TELEPORT_REPO" | $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
  set +x

  set -x
  $SUDO apt-get update
  set +x
}

install_via_yum() {
  TEMP_DIR=$(mktemp -d -t teleport-connect-XXXXXXXXXX)

  ARCH_RPM=$ARCH
  case $ARCH in
  amd64)
    ARCH_RPM="x86_64"
    ;;
  esac

  TELEPORT_FILENAME="teleport-connect-${TELEPORT_VERSION}.${ARCH_RPM}.rpm"
  URL="https://cdn.teleport.dev/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" "$TMP_CHECKSUM"

  set -x
  cd "$TEMP_DIR"
  $SUDO $SHA_COMMAND -c "$TMP_CHECKSUM"
  cd -

  if type dnf &>/dev/null; then
    echo "Installing Teleport Connect v$TELEPORT_VERSION through dnf"
    $SUDO dnf -y install "${TEMP_DIR}/${TELEPORT_FILENAME}"
  else
    echo "Installing Teleport Connect v$TELEPORT_VERSION through yum"
    $SUDO yum -y localinstall "${TEMP_DIR}/${TELEPORT_FILENAME}"
  fi
  set +x
}

# download .tar.gz file via curl/wget, unzip it and run the install sript
install_via_curl() {
  TEMP_DIR=$(mktemp -d -t teleport-connect-XXXXXXXXXX)

  TELEPORT_FILENAME="teleport-connect-v$TELEPORT_VERSION-linux-$ARCH.tar.gz"
  URL="https://cdn.teleport.dev/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" "$TMP_CHECKSUM"

  set -x
  cd "$TEMP_DIR"
  $SUDO $SHA_COMMAND -c "$TMP_CHECKSUM"
  cd -

  $SUDO tar -xzf "${TEMP_DIR}/${TELEPORT_FILENAME}" -C "$TEMP_DIR"
  $SUDO "$TEMP_DIR/teleport/install"
  set +x
}

# wrap script in a function so a partially downloaded script
# doesn't execute
install_teleport() {
  # exit if not on Linux
  if [[ $(uname) != "Linux" ]]; then
    echo "ERROR: This script works only for Linux. Please go to the downloads page to find the proper installation method for your operating system:"
    echo "https://goteleport.com/download/"
    exit 1
  fi

  KERNEL_VERSION=$(uname -r)
  MIN_VERSION="2.6.23"
  if [ $MIN_VERSION != "$(echo -e "$MIN_VERSION\n$KERNEL_VERSION" | sort -V | head -n1)" ]; then
    echo "ERROR: Teleport Connect requires Linux kernel version $MIN_VERSION+"
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
    echo "ERROR: The installer requires a way to run commands as root."
    echo "Either run this script as root or install sudo/doas."
    exit 1
  fi

  # require curl/wget
  CURL=""
  if type curl &>/dev/null; then
    CURL="curl -fL"
  elif type wget &>/dev/null; then
    CURL="wget"
  fi
  if [ -z "$CURL" ]; then
    echo "ERROR: This script requires either curl or wget in order to download files. Please install one of them and try again."
    exit 1
  fi

  # require shasum/sha256sum
  SHA_COMMAND=""
  if type shasum &>/dev/null; then
    SHA_COMMAND="shasum -a 256"
  elif type sha256sum &>/dev/null; then
    SHA_COMMAND="sha256sum"
  else
    echo "ERROR: This script requires sha256sum or shasum to validate the download. Please install it and try again."
    exit 1
  fi

  # detect distro
  OS_RELEASE=/etc/os-release
  ID=""
  ID_LIKE=""
  if [[ -f "$OS_RELEASE" ]]; then
    # Skip checking the os release file
    # shellcheck source=/dev/null
    . $OS_RELEASE
  fi

  # detect architecture
  ARCH=""
  case $(uname -m) in
  x86_64)
    ARCH="amd64"
    ;;
  **)
    echo "ERROR: Teleport Connect is currently only supported on amd64."
    echo "Please refer to the installation guide for more information:"
    echo "https://goteleport.com/docs/installation/"
    exit 1
    ;;
  esac

  # select install method based on distribution
  # if ID is debian derivate, run apt-get
  case "$ID" in
  debian | ubuntu | kali | linuxmint | pop | raspbian | neon | zorin | parrot | elementary)
    install_via_apt_get
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
      install_via_apt_get
      ;;
    "rhel fedora" | fedora | "centos rhel fedora")
      install_via_yum
      ;;
    *)
      # if ID and ID_LIKE didn't return a supported distro, download through curl
      echo "There is no officially supported package to your package manager. Downloading and installing Teleport Connect via curl."
      install_via_curl
      ;;
    esac
    ;;
  esac

  GREEN='\033[0;32m'
  COLOR_OFF='\033[0m'

  echo ""
  echo -e "${GREEN}Teleport Connect $TELEPORT_VERSION installed successfully!${COLOR_OFF}"
  echo "run \`teleport-connect\` to start using Teleport Connect."
}

TELEPORT_VERSION=""
if [ $# -ge 1 ] && [ -n "$1" ]; then
  TELEPORT_VERSION=$1
else
  echo "ERROR: Please provide the version you want to install (e.g., 10.2.1)."
  exit 1
fi
install_teleport
