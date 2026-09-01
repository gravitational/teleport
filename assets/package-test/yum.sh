#!/bin/bash

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

    # Add the repository.
    yum-config-manager --add-repo "$(rpm --eval "https://${repo_domain_name}/$ID/$VERSION_ID/Teleport/%{_arch}/$RELEASE_CHANNEL/$version_channel/teleport-yum.repo")"

    # Append repository_type to repository_name if it's not empty.
    if [[ -n "$repository_type" ]]; then
        repository_name="${repository_name}-${repository_type}"
    fi

    # Rename the repository in the repo file.
    sed -i "s/^\[.*\]/\[${repository_name}\]/" "/etc/yum.repos.d/teleport-yum.repo"

    # Rename the file itself to match the repository type.
    mv "/etc/yum.repos.d/teleport-yum.repo" "/etc/yum.repos.d/${repository_name}.repo"

    echo "successfully initialized teleport repository as ${repository_name}"
    echo "# /etc/yum.repos.d/${repository_name}.repo"
    cat "/etc/yum.repos.d/${repository_name}.repo"
}

# install_teleport installs teleport at the specified version.
function install_teleport() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    local yum_teleport_version
    yum_teleport_version="$(echo "${teleport_version}" | sed 's/-/_/g')"
    yum "${YUM_FLAGS[@]}" install -y "teleport-${yum_teleport_version}"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport ${teleport_version}"
}

# install_package installs teleport by path.
function install_package() {
    : "${1:?teleport_version was not provided}"
    local teleport_version="$1"

    yum "${YUM_FLAGS[@]}" install -y "teleport-${teleport_version}-1.$(rpm --eval '%{_arch}').rpm"
    verify_teleport "${teleport_version}"
    echo "successfully installed teleport teleport-${teleport_version}-1.$(rpm --eval '%{_arch}').rpm"
}
