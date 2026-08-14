#!/bin/bash

# Import utils.sh functions
source utils.sh

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

# install_dependencies installs test dependencies. Ensure the
# source.list is correct for older versions of debian.
function install_dependencies() {
    source /etc/os-release
    if (( VERSION_ID <= 10 )); then
        sed -i 's|deb\.debian\.org|archive.debian.org/debian-archive|' /etc/apt/sources.list
    fi
    apt-get "${APT_FLAGS[@]}" update
    apt-get "${APT_FLAGS[@]}" install curl apt-file apt-utils
}

# initialize_repo initializes the teleport package repository with the provided
# name.
function initialize_repo() {
    : "${1:?repository_name was not provided}"
    local repository_name="$1"

    source /etc/os-release
    curl "https://${REPO_DOMAIN_NAME}/gpg" -o /usr/share/keyrings/teleport-archive-keyring.asc
    echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \
        https://${REPO_DOMAIN_NAME}/${ID?} ${VERSION_CODENAME?} ${RELEASE_CHANNEL}/${VERSION_CHANNEL}" \
        | tee "/etc/apt/sources.list.d/${repository_name}.list" > /dev/null

    # update index for teleport repo
    local source_list="/etc/apt/sources.list.d/${repository_name}.list"
    apt-get "${APT_FLAGS[@]}" update \
        -o Dir::Etc::sourcelist="${source_list}" \
        -o Dir::Etc::sourceparts="-" \
        -o APT::Get::List-Cleanup="0"

    echo "successfully initialized teleport repository"
    echo "# /etc/apt/sources.list.d/${repository_name}.list"
    cat "/etc/apt/sources.list.d/${repository_name}.list"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    apt-get "${APT_FLAGS[@]}" install "teleport-ent=${teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport-ent ${teleport_version}"
}

# install_updater installs the teleport updater at the specified version.
function install_updater() {
    : "${1:?updater_version was not provided}"
    local updater_version="$1"

    apt-get "${APT_FLAGS[@]}" install "teleport-ent-updater=${updater_version}"
    verify_updater "${updater_version}"
    echo "successfully installed teleport-ent-updater ${updater_version}"
}

# update_system performs a system update.
function update_system() {
    local expected_teleport_version
    expected_teleport_version="$(teleport version --raw)"

    # system update may return a non 0 exit code due to teleport-ent-updater post install
    apt-get "${APT_FLAGS[@]}" upgrade || true
    verify_teleport "${expected_teleport_version}"
}
