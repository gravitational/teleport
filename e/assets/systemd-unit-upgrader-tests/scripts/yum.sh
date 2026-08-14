#!/bin/bash

# Import utils.sh functions
source utils.sh

# YUM_FLAGS defines a few yum command flags
#   -y assumes "yes"
#   -q runs in quiet mode to reduce noise
YUM_FLAGS=("-y" "-q")

# log_pre_install_info logs relevant package information before installation.
function log_pre_install_info() {
    : "${1:?package_name was not provided}"
    local package_name="$1"

    echo "Available versions of package:"
    yum --showduplicates -y list available "${package_name}"
    echo "Deployed package version:"
    yum -y list "${package_name}"
    echo "Package files:"
    repoquery -l --installed "${package_name}"
}

# install_dependencies installs test dependencies
function install_dependencies() {
    yum "${YUM_FLAGS[@]}" install yum-utils
}

# initialize_repo initializes the teleport package repository with the provided
# name.
function initialize_repo() {
    : "${1:?repository_name was not provided}"
    local repository_name="$1"

    source /etc/os-release
    rpm --import "https://${REPO_DOMAIN_NAME}/gpg"
    VERSION_ID=$(echo "${VERSION_ID}" | cut -d'.' -f1)
    yum-config-manager --add-repo "$(rpm --eval "https://${REPO_DOMAIN_NAME}/$ID/$VERSION_ID/Teleport/%{_arch}/$RELEASE_CHANNEL/$VERSION_CHANNEL/teleport-yum.repo")"
    # Rename the repository in the repo file
    sed -i "s/^\[.*\]/\[${repository_name}\]/" "/etc/yum.repos.d/teleport-yum.repo"

    echo "successfully initialized teleport repository"
    echo "# /etc/yum.repos.d/teleport-yum.repo"
    cat "/etc/yum.repos.d/teleport-yum.repo"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    local yum_teleport_version
    yum_teleport_version="$(echo "${teleport_version}" | sed 's/-/_/g')"
    yum "${YUM_FLAGS[@]}" install "teleport-ent-${yum_teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport-ent ${teleport_version}"
}

# install_updater installs the teleport updater at the specified version.
function install_updater() {
    : "${1:?updater_version was not provided}"
    local updater_version="$1"

    local yum_updater_version
    yum_updater_version="$(echo "${updater_version}" | sed 's/-/_/g')"
    yum "${YUM_FLAGS[@]}" install "teleport-ent-updater-${yum_updater_version}"
    verify_updater "${updater_version}"
    echo "successfully installed teleport-ent-updater ${updater_version}"
}

# update_system performs a system update
function update_system() {
    local expected_teleport_version
    expected_teleport_version="$(teleport version --raw)"

    # system update may return a non 0 exit code due to teleport-ent-updater post install
    yum "${YUM_FLAGS[@]}" upgrade || true
    verify_teleport "${expected_teleport_version}"
}
