#!/bin/bash
# Copyright 2022 Gravitational, Inc

# This script detects the current Linux distribution and installs Teleport
# through its package manager, if supported, or downloading a tarball otherwise.
# We'll download Teleport from the official website and checksum it to make sure it was properly
# downloaded before executing.

# The script is wrapped inside a function to protect against the connection being interrupted
# in the middle of the stream.

# For more download options, head to https://goteleport.com/download/

set -euo pipefail

# download uses curl or wget to download a teleport binary
download() {
  URL=$1
  TMP_PATH=$2

  echo "Downloading $URL"
  if type curl &>/dev/null; then
    set -x
    # shellcheck disable=SC2086
    $SUDO $CURL -o "$TMP_PATH" "$URL"
  else
    set -x
    # shellcheck disable=SC2086
    $SUDO $CURL -O "$TMP_PATH" "$URL"
  fi
  set +x
}

install_via_apt_get() {
  echo "Installing Teleport v$TELEPORT_VERSION via apt-get"
  add_apt_key
  set -x
  $SUDO apt-get install -y "teleport$TELEPORT_SUFFIX=$TELEPORT_VERSION"
  set +x
  if [ "$TELEPORT_EDITION" = "cloud" ]; then
    set -x
    $SUDO apt-get install -y teleport-ent-updater
    set +x
  fi
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

  CHANNEL="stable/v${MAJOR}"
  if [ "$TELEPORT_EDITION" = "cloud" ]; then
    CHANNEL="stable/cloud"
  fi

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
    $SUDO apt-key add "$TMP_KEY"
    set +x
    TELEPORT_REPO="deb https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} ${CHANNEL}"
  else
    TMP_KEY="$TEMP_DIR/teleport-pubkey.gpg"
    download "https://apt.releases.teleport.dev/gpg" "$TMP_KEY"
    set -x
    $SUDO mkdir -p /etc/apt/keyrings
    $SUDO cp "$TMP_KEY" /etc/apt/keyrings/teleport-archive-keyring.asc
    set +x
    TELEPORT_REPO="deb [signed-by=/etc/apt/keyrings/teleport-archive-keyring.asc]  https://apt.releases.teleport.dev/${APT_REPO_ID?} ${APT_REPO_VERSION_CODENAME?} ${CHANNEL}"
  fi

  set -x
  echo "$TELEPORT_REPO" | $SUDO tee /etc/apt/sources.list.d/teleport.list >/dev/null
  set +x

  set -x
  $SUDO apt-get update
  set +x
}

# $1 is the value of the $ID path segment in the YUM repo URL. In
# /etc/os-release, this is either the value of $ID or $ID_LIKE.
install_via_yum() {
  # shellcheck source=/dev/null
  source /etc/os-release

  # Get the major version from the version ID.
  VERSION_ID=$(echo "$VERSION_ID" | grep -Eo "^[0-9]+")
  TELEPORT_MAJOR_VERSION="v$(echo "$TELEPORT_VERSION" | grep -Eo "^[0-9]+")"

  CHANNEL="stable/${TELEPORT_MAJOR_VERSION}"
  if [ "$TELEPORT_EDITION" = "cloud" ]; then
    CHANNEL="stable/cloud"
  fi

  if type dnf &>/dev/null; then
    echo "Installing Teleport v$TELEPORT_VERSION through dnf"
    $SUDO dnf install -y 'dnf-command(config-manager)'
    $SUDO dnf config-manager --add-repo "$(rpm --eval "https://yum.releases.teleport.dev/$1/$VERSION_ID/Teleport/%{_arch}/$CHANNEL/teleport-yum.repo")"
    $SUDO dnf install -y "teleport$TELEPORT_SUFFIX-$TELEPORT_VERSION"

    if [ "$TELEPORT_EDITION" = "cloud" ]; then
      $SUDO dnf install -y teleport-ent-updater
    fi

  else
    echo "Installing Teleport v$TELEPORT_VERSION through yum"
    $SUDO yum install -y yum-utils
    $SUDO yum-config-manager --add-repo "$(rpm --eval "https://yum.releases.teleport.dev/$1/$VERSION_ID/Teleport/%{_arch}/$CHANNEL/teleport-yum.repo")"
    $SUDO yum install -y "teleport$TELEPORT_SUFFIX-$TELEPORT_VERSION"

    if [ "$TELEPORT_EDITION" = "cloud" ]; then
      $SUDO yum install -y teleport-ent-updater
    fi
  fi
  set +x
}

install_via_zypper() {
  # shellcheck source=/dev/null
  source /etc/os-release

  # Get the major version from the version ID.
  VERSION_ID=$(echo "$VERSION_ID" | grep -Eo "^[0-9]+")
  TELEPORT_MAJOR_VERSION="v$(echo "$TELEPORT_VERSION" | grep -Eo "^[0-9]+")"

  CHANNEL="stable/${TELEPORT_MAJOR_VERSION}"
  if [ "$TELEPORT_EDITION" = "cloud" ]; then
    CHANNEL="stable/cloud"
  fi

  $SUDO rpm --import https://zypper.releases.teleport.dev/gpg
  $SUDO zypper addrepo --refresh --repo "$(rpm --eval "https://zypper.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/$CHANNEL/teleport-zypper.repo")"
  $SUDO zypper --gpg-auto-import-keys refresh teleport
  $SUDO zypper install -y "teleport$TELEPORT_SUFFIX"

  if [ "$TELEPORT_EDITION" = "cloud" ]; then
    $SUDO zypper install -y teleport-ent-updater
  fi

  set +x
}


# download .tar.gz file via curl/wget, unzip it and run the install script
install_via_curl() {
  TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)

  TELEPORT_FILENAME="teleport$TELEPORT_SUFFIX-v$TELEPORT_VERSION-linux-$ARCH-bin.tar.gz"
  URL="https://cdn.teleport.dev/${TELEPORT_FILENAME}"
  download "${URL}" "${TEMP_DIR}/${TELEPORT_FILENAME}"

  TMP_CHECKSUM="${TEMP_DIR}/${TELEPORT_FILENAME}.sha256"
  download "${URL}.sha256" "$TMP_CHECKSUM"

  set -x
  cd "$TEMP_DIR"
  # shellcheck disable=SC2086
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
  # Some $ID_LIKE values include multiple distro names in an arbitrary order, so
  # evaluate the first one.
  ID_LIKE="${ID_LIKE%% *}"

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
    # rh/fedora derived distros using the ID_LIKE var.
    case "${ID_LIKE}" in
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
      if [ "$TELEPORT_EDITION" = "cloud" ]; then
        echo "The system does not support a package manager, which is required for Teleport Enterprise Cloud."
        exit 1
      fi

      # if ID and ID_LIKE didn't return a supported distro, download through curl
      echo "There is no officially supported package for your package manager. Downloading and installing Teleport via curl."
      install_via_curl
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
    echo "  teleport        - The daemon that runs the Auth Service, Proxy Service, and other Teleport services."
  fi
  if type tsh &>/dev/null; then
    echo "  tsh             - A tool that lets end users interact with Teleport."
  fi
  if type tctl &>/dev/null; then
    echo "  tctl            - An administrative tool that can configure the Teleport Auth Service."
  fi
  if type tbot &>/dev/null; then
    echo "  tbot            - Teleport Machine ID client."
  fi
  if type fdpass-teleport &>/dev/null; then
    echo "  fdpass-teleport - Teleport Machine ID client."
  fi
  if type teleport-update &>/dev/null; then
    echo "  teleport-update - Teleport auto-update agent."
  fi
}

# The suffix is "-ent" if we are installing a commercial edition of Teleport and
# empty for Teleport Community Edition.
TELEPORT_SUFFIX=""
TELEPORT_VERSION=""
TELEPORT_EDITION=""
if [ $# -ge 1 ] && [ -n "$1" ]; then
  TELEPORT_VERSION=$1
else
  echo "ERROR: Please provide the version you want to install (e.g., 10.1.9)."
  exit 1
fi

if ! echo "$1" |  grep -qE "[0-9]+\.[0-9]+\.[0-9]+"; then
  echo "ERROR: The first parameter must be a version number, e.g., 10.1.9."
  exit 1
fi

if [ $# -ge 2 ] && [ -n "$2" ]; then
  TELEPORT_EDITION=$2

  case $TELEPORT_EDITION in
      enterprise | cloud)
      	  TELEPORT_SUFFIX="-ent"
	  ;;
      # An empty edition defaults to OSS.
      oss | "" )
	  ;;
      *)
        echo 'ERROR: The second parameter must be "oss", "cloud", or "enterprise".'
        exit 1
      ;;
  esac
fi
install_teleport
