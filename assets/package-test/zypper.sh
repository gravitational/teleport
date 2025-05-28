#!/bin/bash

# ZYPPER_FLAGS defines a few zypper command flags
#   -n runs in non-interactive mode using default answers.
#   -q runs in quiet mode to reduce noise
ZYPPER_FLAGS=("-nq")

# log_pre_install_info logs relevant package information before installation.
function log_pre_install_info() {
    : "${1:?package_name was not provided}"
    local package_name="$1"

    zypper search -s "${package_name}"
}

# install_dependencies installs test dependencies
function install_dependencies() {
    zypper "${ZYPPER_FLAGS[@]}" install curl gawk
}

# initialize_repo initializes the teleport package repository with the provided name and type.
function initialize_repo() {
    : "${1:?repository_name was not provided}"

    local repository_name="$1"
    local repository_type="${2:-}"
    local repo_domain_name="${3:-$REPO_DOMAIN_NAME}"
    local version_channel="${4:-$VERSION_CHANNEL}"

    : "${repo_domain_name:?REPO_DOMAIN_NAME is required (argument or env variable)}"
    : "${version_channel:?VERSION_CHANNEL is required (argument or env variable)}"

    source /etc/os-release
    rpm --import "https://${repo_domain_name}/gpg"
    VERSION_ID=$(echo "${VERSION_ID}" | cut -d'.' -f1)

    # Append repository_type to repository_name if it's not empty.
    if [[ -n "$repository_type" ]]; then
        repository_name="${repository_name}-${repository_type}"
    fi

    local repo_url
    repo_url="$(rpm --eval "https://${repo_domain_name}/$ID/$VERSION_ID/Teleport/%{_arch}/$RELEASE_CHANNEL/$version_channel")"

    zypper "${ZYPPER_FLAGS[@]}" addrepo --refresh "$repo_url" "$repository_name"

    echo "successfully initialized teleport repository: ${repository_name}"
    zypper "${ZYPPER_FLAGS[@]}" refresh "$repository_name"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    local zypper_teleport_version
    zypper_teleport_version="$(echo "${teleport_version}" | sed 's/-/_/g')-1"
    zypper "${ZYPPER_FLAGS[@]}" install --allow-downgrade --oldpackage "teleport-${zypper_teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport ${teleport_version}"
}

# install_package installs teleport by path.
function install_package() {
: "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    zypper "${ZYPPER_FLAGS[@]}" --no-gpg-checks install --allow-downgrade --oldpackage "teleport-${teleport_version}-1.$(rpm --eval '%{_arch}').rpm"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport teleport-${teleport_version}-1.$(rpm --eval '%{_arch}').rpm"
}
