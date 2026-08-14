#!/bin/bash

# conf dir is the default teleport upgrade config directory
conf_dir="/etc/teleport-upgrade.d"

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

# verify_updater verifies the install version of the updater matches the provided
# expected_updater_version.
function verify_updater() {
    : "${1:?expected_updater_version was not provided}"
    local expected_updater_version="$1"

    local installed_updater_version
    installed_updater_version="$(teleport-upgrade version --raw)"
    installed_updater_version="${installed_updater_version#v}"
    if [ "${installed_updater_version}" != "${expected_updater_version}" ]; then
        echo "Installed updater version (${installed_updater_version}) does not match expected version (${expected_updater_version})"
        return 1
    fi
}

# update_teleport performs a teleport update via the teleport updater
function update_teleport() {
    local update_endpoint
    update_endpoint="$(cat "${conf_dir}/endpoint")"
    local update_teleport_version
    update_teleport_version="$(curl "https://${update_endpoint}/version")"
    update_teleport_version="${update_teleport_version#v}"

    teleport-upgrade force
    verify_teleport "${update_teleport_version}"
    echo "successfully updated teleport-ent to ${update_teleport_version}"
}

# set_config configures the specified config value.
function set_config() {
    : "${1:?name was not provided}"
    : "${2:?value was not provided}"
    local name="$1"
    local value="$2"

    # Ensure config directory exists.
    if [ ! -d "${conf_dir}" ]; then
        mkdir -p "${conf_dir}"
    fi

    echo "${value}" > "${conf_dir}/${name}"
    echo "set ${name}=${value}"
}
