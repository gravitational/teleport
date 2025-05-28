#!/bin/bash

set -euo pipefail

# Verify required env is available
: "${INSTALLER:?INSTALLER env is missing}"
: "${RELEASE_CHANNEL:?RELEASE_CHANNEL env is missing}"
: "${ARTIFACT_TAG:?ARTIFACT_TAG env is missing}"
: "${REPO_DOMAIN_NAME:?REPO_DOMAIN_NAME env is missing}"
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
VERSION_CHANNEL="v${TELEPORT_VERSION%%.*}"

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

# test_upgrade_downgrade tests teleport upgrade and downgrade across a major packaging
# change in v17.2.9 and v16.4.29 that moves the teleport binaries and creates symlinks.
function test_upgrade_downgrade() {
    local version_number="${VERSION_CHANNEL//[!0-9]/}"
    # Determine version_release based on version_number
    local version_release=""
    local version_chanel=""
    if [[ "$version_number" -eq 17 || "$version_number" -eq 18 ]]; then
        version_release="17.2.9"
        version_chanel="v17"
    elif [[ "$version_number" -eq 16 ]]; then
        version_release="16.4.29"
        version_chanel="v16"
    elif [[ "$version_number" -eq 15 ]]; then
        version_release="15.4.29"
        version_chanel="v15"
    elif [[ "$version_number" -eq 14 ]]; then
        version_release="14.3.36"
        version_chanel="v14"
    else
        echo "VERSION_CHANNEL ($VERSION_CHANNEL) is not supported, skipping initialization."
        return
    fi

    run "initialize_repo" "teleport" "prod" "${REPO_DOMAIN_NAME}" "${version_chanel}"
    run "install_teleport" "${version_release}"
    run "install_package" "${TELEPORT_VERSION}"
    run "install_teleport" "${version_release}"
}

# verify_teleport verifies the installed version of teleport matches the provided
# expected_teleport_version.
function verify_teleport() {
    : "${1:?expected_teleport_version was not provided}"
    local expected_teleport_version="$1"

    local installed_teleport_version
    installed_teleport_version="$(teleport version --raw)"
    if [ "${installed_teleport_version}" != "${expected_teleport_version}" ]; then
        echo "Installed teleport version (${installed_teleport_version}) does not match expected version (${expected_teleport_version})"
        return 1
    fi
}
