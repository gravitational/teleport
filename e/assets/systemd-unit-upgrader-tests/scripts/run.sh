#!/bin/bash

set -euo pipefail

# Verify required env is available
: "${INSTALLER:?INSTALLER env is missing}"
: "${RELEASE_CHANNEL:?RELEASE_CHANNEL env is missing}"
: "${VERSION_CHANNEL:?VERSION_CHANNEL env is missing}"
: "${ARTIFACT_TAG:?ARTIFACT_TAG env is missing}"
: "${REPO_DOMAIN_NAME:?REPO_DOMAIN_NAME env is missing}"
: "${AUTO_UPDATES_DOMAIN_NAME:?AUTO_UPDATES_DOMAIN_NAME env is missing}"
: "${PACKAGE_TO_TEST:?PACKAGE_TO_TEST env is missing}"

# Import the compatible test scripts for the specified installer.
case "${INSTALLER}" in
    apt)
        source apt.sh
        ;;
    yum)
        source yum.sh
        ;;
    zypper)
        source zypper.sh
        ;;
    *)
        echo "unsupported installer ${INSTALLER}"
        exit 1
        ;;
esac

# Remove 'v' prefix from versions
TELEPORT_VERSION="${ARTIFACT_TAG#v}"
UPDATER_VERSION="${ARTIFACT_TAG#v}"

# Configure installer specific repo domain name
REPO_DOMAIN_NAME="${INSTALLER}.${REPO_DOMAIN_NAME}"

# run runs the function while logging the step
function run() {
    : "${1:?func was not provided}"
    local func="$1"

    echo "===${func}==="
    shift
    "${func}" "$@"
}

# test_pre_install initializes the teleport repo and outputs package information.
function test_pre_install() {
    run "initialize_repo" "teleport"
    run "log_pre_install_info" "${PACKAGE_TO_TEST}" || true
}

# test_teleport_basic tests a basic teleport installation
function test_teleport_basic() {
    run "initialize_repo" "teleport"
    run "install_teleport" "${TELEPORT_VERSION}"
}

# test_updater_basic tests a basic updater installation
function test_updater_basic() {
    # Set the initial teleport version to an arbitrary version
    export TELEPORT_VERSION="14.3.17"

    run "initialize_repo" "teleport"
    run "install_teleport" "${TELEPORT_VERSION}"
    run "install_updater" "${UPDATER_VERSION}"
    run "set_config" "installer" "${INSTALLER}"
    run "set_config" "endpoint" "${AUTO_UPDATES_DOMAIN_NAME}/v1/${RELEASE_CHANNEL}/${VERSION_CHANNEL}"
    run "update_teleport"
    run "update_system"
}

# test_installer_omitted tests the updater when installer config is missing
function test_installer_omitted() {
    # Set the initial teleport version to an arbitrary version
    export TELEPORT_VERSION="14.3.17"

    run "initialize_repo" "teleport"
    run "install_teleport" "${TELEPORT_VERSION}"
    run "install_updater" "${UPDATER_VERSION}"
    run "set_config" "installer" ""
    run "set_config" "endpoint" "${AUTO_UPDATES_DOMAIN_NAME}/v1/${RELEASE_CHANNEL}/${VERSION_CHANNEL}"
    run "update_teleport"
}

# test_installer_misconfigured tests the updater when installer config is misconfigured
function test_installer_misconfigured() {
    # Set the initial teleport version to an arbitrary version
    export TELEPORT_VERSION="14.3.17"

    run "initialize_repo" "teleport"
    run "install_teleport" "${TELEPORT_VERSION}"
    run "install_updater" "${UPDATER_VERSION}"
    run "set_config" "installer" "invalid"
    run "set_config" "endpoint" "${AUTO_UPDATES_DOMAIN_NAME}/v1/${RELEASE_CHANNEL}/${VERSION_CHANNEL}"
    run "update_teleport"
}
