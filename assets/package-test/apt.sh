#!/bin/bash

# DEBIAN_FRONTEND when set to noninteractive, instructs debconf to run non-interactively
# and accepts default answers.
export DEBIAN_FRONTEND=noninteractive

# APT_FLAGS defines a few apt-get command flags
#   -y assumes "yes"
#   -qq runs in quiet mode to reduce noise
#   -o=Dpkg::Use-Pty=0 disables use of pty to further reduce noise
APT_FLAGS=("-yqq" "-o=Dpkg::Use-Pty=0")

# log_pre_install_info logs relevant package information before installation.
function log_pre_install_info() {
    : "${1:?package_name was not provided}"
    local package_name="$1"

    echo "Available versions of package:"
    apt-cache policy "${package_name}"
    echo "Package files:"
    apt-file list "${package_name}"
}

# install_dependencies installs test dependencies.
function install_dependencies() {
    apt-get "${APT_FLAGS[@]}" update
    apt-get "${APT_FLAGS[@]}" install curl apt-file apt-utils
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
    curl -fsSL "https://${repo_domain_name}/gpg" -o /usr/share/keyrings/teleport-archive-keyring.asc

    # Append repository_type to repository_name if it's not empty.
    if [[ -n "$repository_type" ]]; then
        repository_name="${repository_name}-${repository_type}"
    fi

    # Add repo with computed values
    echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \
        https://${repo_domain_name}/${ID?} ${VERSION_CODENAME?} ${RELEASE_CHANNEL}/${version_channel}" \
        | tee "/etc/apt/sources.list.d/${repository_name}.list" > /dev/null

    # Update index for teleport repo
    local source_list="/etc/apt/sources.list.d/${repository_name}.list"
    apt-get "${APT_FLAGS[@]}" update \
        -o Dir::Etc::sourcelist="${source_list}" \
        -o Dir::Etc::sourceparts="-" \
        -o APT::Get::List-Cleanup="0"

    echo "successfully initialized teleport repository as ${repository_name}"
    echo "# /etc/apt/sources.list.d/${repository_name}.list"
    cat "/etc/apt/sources.list.d/${repository_name}.list"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    apt-get "${APT_FLAGS[@]}" install --allow-downgrades "teleport=${teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport ${teleport_version}"
}

# install_package installs teleport by path.
function install_package() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    dpkg -i "teleport_${teleport_version}_$(dpkg --print-architecture).deb"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport teleport_${teleport_version}_$(dpkg --print-architecture).deb"
}
