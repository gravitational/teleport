#!/bin/bash
# Copyright 2024 Gravitational, Inc

# This script detects the current Linux distribution and installs Teleport.

# The script is wrapped inside a function to protect against the connection being interrupted
# in the middle of the stream.

set -euo pipefail

# TELEPORT_VERSION is the target teleport version to install
TELEPORT_VERSION="{{.teleportVersion}}"

# download uses curl or wget to download a teleport binary
download() {
  URL=$1
  TMP_PATH=$2

  echo "Downloading $URL"
  if type curl &>/dev/null; then
    # shellcheck disable=SC2086
    $SUDO $CURL -o "$TMP_PATH" "$URL"
  else
    # shellcheck disable=SC2086
    $SUDO $CURL -O "$TMP_PATH" "$URL"
  fi
}

install_via_apt_get() {
  echo "Installing Teleport v$TELEPORT_VERSION via apt-get"

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
  TELEPORT_REPO=""

  if [[ "$IS_LEGACY" == 1 ]]; then
    if ! type gpg >/dev/null; then
      echo "Installing gnupg"
      $SUDO apt-get update
      $SUDO apt-get install -y gnupg
    fi
    TMP_KEY="$TEMP_DIR/teleport-pubkey.asc"
    download "https://deb.releases.teleport.dev/teleport-pubkey.asc" "$TMP_KEY"
    $SUDO apt-key add "$TMP_KEY"
    TELEPORT_REPO="deb https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} stable/cloud"
  else
    TMP_KEY="$TEMP_DIR/teleport-pubkey.gpg"
    download "https://apt.releases.teleport.dev/gpg" "$TMP_KEY"
    $SUDO cp "$TMP_KEY" /usr/share/keyrings/teleport-archive-keyring.asc
    TELEPORT_REPO="deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc]  https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} stable/cloud"
  fi

  echo "$TELEPORT_REPO" | $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
  $SUDO apt-get update
  $SUDO apt-get install -y --allow-change-held-packages "teleport-ent=$TELEPORT_VERSION" teleport-ent-updater
  $SUDO apt-mark hold teleport-ent > /dev/null
}

# $1 is the value of the $ID path segment in the YUM repo URL. In
# /etc/os-release, this is either the value of $ID or $ID_LIKE.
install_via_yum() {
  # shellcheck source=/dev/null
  source /etc/os-release

  # Get the major version from the version ID.
  VERSION_ID=$(echo "$VERSION_ID" | grep -Eo "^[0-9]+")

  echo "Installing Teleport v$TELEPORT_VERSION through yum"
  $SUDO yum install -y yum-utils
  $SUDO yum-config-manager --add-repo "$(rpm --eval "https://yum.releases.teleport.dev/$1/$VERSION_ID/Teleport/%{_arch}/stable/cloud/teleport-yum.repo")"
  $SUDO yum install -y --disablerepo="*" --enablerepo="teleport" --disableexcludes="teleport" "teleport-ent-$TELEPORT_VERSION" teleport-ent-updater
  $SUDO yum-config-manager --save --setopt "teleport.exclude=teleport-ent" > /dev/null
}

install_via_zypper() {
  # shellcheck source=/dev/null
  source /etc/os-release

  # Get the major version from the version ID.
  VERSION_ID=$(echo "$VERSION_ID" | grep -Eo "^[0-9]+")

  $SUDO zypper addrepo --refresh --repo $(rpm --eval "https://zypper.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/stable/cloud/teleport-zypper.repo")
  $SUDO zypper --gpg-auto-import-keys refresh teleport
  $SUDO zypper removelock "teleport-ent"
  $SUDO zypper install -y "teleport-ent-$TELEPORT_VERSION" teleport-ent-updater
  $SUDO zypper addlock "teleport-ent"
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
    echo "ERROR: Teleport requires Linux kernel version $MIN_VERSION+"
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
    echo "ERROR:  The installer requires a way to run commands as root."
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
  VERSION_CODENAME=""
  UBUNTU_CODENAME=""
  if [[ -f "$OS_RELEASE" ]]; then
    # shellcheck source=/dev/null
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
    echo "ERROR: Your system's architecture isn't officially supported or couldn't be determined."
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
  centos | rhel | amzn)
    install_via_yum "$ID"
    ;;
  sles)
    install_via_zypper
    ;;
  *)
    # before downloading manually, double check if we didn't miss any debian or
    # rh/fedora derived distros using the ID_LIKE var. Some $ID_LIKE values
    # include multiple distro names in an arbitrary order, so evaluate the first
    # one.
    case "$(echo "$ID_LIKE" | awk '{print $1}')" in
    ubuntu | debian)
      install_via_apt_get
      ;;
    centos | fedora | rhel)
      # There is no repository for "fedora", and there is no difference
      # between the repositories for "centos" and "rhel", so pick an arbitrary
      # one.
      install_via_yum rhel
      ;;
    *)
      echo "The system does not support a package manager, which is required for Teleport Enterprise Cloud."
      exit 1
      ;;
    esac
    ;;
  esac

  GREEN='\033[0;32m'
  COLOR_OFF='\033[0m'

  echo ""
  echo -e "${GREEN}$(teleport version) installed successfully!${COLOR_OFF}"
  echo ""
  echo "The following commands are now available:"
  if type teleport &>/dev/null; then
    echo "  teleport         - The daemon that runs the Auth Service, Proxy Service, and other Teleport services."
  fi
  if type tsh &>/dev/null; then
    echo "  tsh              - A tool that lets end users interact with Teleport."
  fi
  if type tctl &>/dev/null; then
    echo "  tctl             - An administrative tool that can configure the Teleport Auth Service."
  fi
  if type tbot &>/dev/null; then
    echo "  tbot             - Teleport Machine ID client."
  fi
  if type teleport-upgrade &>/dev/null; then
    echo "  teleport-upgrade - Teleport upgrade CLI tool"
  fi
}

reload_teleport() {
  # fail quietly when systemd is disabled. This is only relevant when testing
  # in a container.
  if systemctl status > /dev/null 2>&1; then
    $SUDO systemctl daemon-reload
    if systemctl is-active --quiet teleport.service; then
      # we fall back to restart in case the reload fails, for example if
      # the pidfile used for reloading is not valid; this can happen
      # across the upgrade to the Teleport version that started locking
      # the pidfile
      $SUDO systemctl reload teleport.service || $SUDO systemctl try-restart teleport.service
    fi
  fi
}

install_teleport
reload_teleport
