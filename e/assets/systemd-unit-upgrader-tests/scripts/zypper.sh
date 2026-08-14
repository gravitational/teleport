#!/bin/bash

# Import utils.sh functions
source utils.sh

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

# initialize_repo initializes the teleport package repository with the provided
# name.
function initialize_repo() {
    : "${1:?repository_name was not provided}"
    local repository_name="$1"

    source /etc/os-release
    rpm --import "https://${REPO_DOMAIN_NAME}/gpg"
    VERSION_ID=$(echo "${VERSION_ID}" | cut -d'.' -f1)
    zypper "${ZYPPER_FLAGS[@]}" addrepo --refresh \
        --repo "$(rpm --eval "https://${REPO_DOMAIN_NAME}/$ID/$VERSION_ID/Teleport/%{_arch}/$RELEASE_CHANNEL/$VERSION_CHANNEL/teleport-zypper.repo")" \
        --name "${repository_name}"

    echo "successfully initialized teleport repository"
    zypper "${ZYPPER_FLAGS[@]}" repos "${repository_name}"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    local zypper_teleport_version
    zypper_teleport_version="${teleport_version//-/_}-1"
    zypper "${ZYPPER_FLAGS[@]}" install "teleport-ent-${zypper_teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport-ent ${teleport_version}"
}

# install_updater installs the teleport updater at the specified version.
function install_updater() {
    : "${1:?updater_version was not provided}"
    local updater_version="$1"

    local zypper_updater_version
    zypper_updater_version="${updater_version//-/_}-1"
    zypper "${ZYPPER_FLAGS[@]}" install "teleport-ent-updater-${zypper_updater_version}"
    verify_updater "${updater_version}"
    echo "successfully installed teleport-ent-updater ${updater_version}"
}

# update_system performs a system update
function update_system() {
    local expected_teleport_version
    expected_teleport_version="$(teleport version --raw)"

    # system update may return a non 0 exit code due to teleport-ent-updater post install
    zypper "${ZYPPER_FLAGS[@]}" update || true
    verify_teleport "${expected_teleport_version}"
}
