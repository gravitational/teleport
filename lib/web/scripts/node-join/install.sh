#!/bin/bash
set -euo pipefail
SCRIPT_NAME="teleport-installer"

# default values
ALIVE_CHECK_DELAY=3
CONNECTIVITY_TEST_METHOD=""
COPY_COMMAND="cp"
DISTRO_TYPE=""
IGNORE_CONNECTIVITY_CHECK="${TELEPORT_IGNORE_CONNECTIVITY_CHECK:-false}"
LAUNCHD_CONFIG_PATH="/Library/LaunchDaemons"
LAUNCHD_PLIST_FILE="com.goteleport.teleport.plist"
LOG_FILENAME="$(mktemp -t ${SCRIPT_NAME}.log.XXXXXXXXXX)"
MACOS_STDERR_LOG="/var/log/teleport-stderr.log"
MACOS_STDOUT_LOG="/var/log/teleport-stdout.log"
SYSTEMD_UNIT_PATH="/lib/systemd/system/teleport.service"
TARGET_PORT_DEFAULT=443
TELEPORT_ARCHIVE_PATH='{{.packageName}}'
TELEPORT_BINARY_DIR="/usr/local/bin"
TELEPORT_BINARY_LIST="teleport tctl tsh"
TELEPORT_CONFIG_PATH="/etc/teleport.yaml"
TELEPORT_DATA_DIR="/var/lib/teleport"
TELEPORT_DOCS_URL="https://goteleport.com/docs/"
# TELEPORT_FORMAT contains the Teleport installation formats.
# The value is dynamically computed unless OVERRIDE_FORMAT it set.
# Possible values are:
# - "deb"
# - "rpm"
# - "tarball"
# - "updater"
TELEPORT_FORMAT=""

# initialise variables (because set -u disallows unbound variables)
f=""
l=""
DISABLE_TLS_VERIFICATION=false
NODENAME=$(hostname)
IGNORE_CHECKS=false
OVERRIDE_FORMAT=""
QUIET=false
APP_INSTALL_DECISION=""
INTERACTIVE=false

# the default value of each variable is a templatable Go value so that it can
# optionally be replaced by the server before the script is served up
TELEPORT_VERSION='{{.version}}'
TELEPORT_PACKAGE_NAME='{{.packageName}}'
# UPDATER_STYLE holds the Teleport updater style.
# Supported values are "none", "" (same as "none"), "package", and "binary".
UPDATER_STYLE='{{.installUpdater}}'
REPO_CHANNEL='{{.repoChannel}}'
TARGET_HOSTNAME='{{.hostname}}'
TARGET_PORT='{{.port}}'
JOIN_TOKEN='{{.token}}'
JOIN_METHOD='{{.joinMethod}}'
JOIN_METHOD_FLAG=""
[ -n "$JOIN_METHOD" ] && JOIN_METHOD_FLAG="--join-method ${JOIN_METHOD}"

# inject labels into the configuration
LABELS="{{.labels}}"
LABELS_FLAG=()
[ -n "$LABELS" ] && LABELS_FLAG=(--labels "${LABELS}")

# When all stanza generators have been updated to use the new
# `teleport <service> configure` commands CA_PIN_HASHES can be removed along
# with the script passing it in in `join_tokens.go`.
CA_PIN_HASHES='{{.caPinsOld}}'
CA_PINS='{{.caPins}}'
ARG_CA_PIN_HASHES=""
APP_INSTALL_MODE='{{.appInstallMode}}'
APP_NAME='{{.appName}}'
APP_URI='{{.appURI}}'
DB_INSTALL_MODE='{{.databaseInstallMode}}'
DISCOVERY_INSTALL_MODE='{{.discoveryInstallMode}}'

# usage message
# shellcheck disable=SC2086
usage() { echo "Usage: $(basename $0) [-v teleport_version] [-h target_hostname] [-p target_port] [-j join_token] [-c ca_pin_hash]... [-q] [-l log_filename] [-a app_name] [-u app_uri] " 1>&2; exit 1; }
while getopts ":v:h:p:j:c:f:ql:ika:u:" o; do
    case "${o}" in
        v)  TELEPORT_VERSION=${OPTARG};;
        h)  TARGET_HOSTNAME=${OPTARG};;
        p)  TARGET_PORT=${OPTARG};;
        j)  JOIN_TOKEN=${OPTARG};;
        c)  ARG_CA_PIN_HASHES="${ARG_CA_PIN_HASHES} ${OPTARG}";;
        f)  f=${OPTARG}; if [[ ${f} != "tarball" && ${f} != "deb" && ${f} != "rpm" ]]; then usage; fi;;
        q)  QUIET=true;;
        l)  l=${OPTARG};;
        i)  IGNORE_CHECKS=true; COPY_COMMAND="cp -f";;
        k)  DISABLE_TLS_VERIFICATION=true;;
        a)  APP_INSTALL_MODE=true && APP_NAME=${OPTARG};;
        u)  APP_INSTALL_MODE=true && APP_URI=${OPTARG};;
        *)  usage;;
    esac
done
shift $((OPTIND-1))

if [[ "${ARG_CA_PIN_HASHES}" != "" ]]; then
    CA_PIN_HASHES="${ARG_CA_PIN_HASHES}"
fi

# function to construct a go template variable
# go's template parser is a bit finicky, so we dynamically build the value one character at a time
construct_go_template() {
    OUTPUT="{"
    OUTPUT+="{"
    OUTPUT+="."
    OUTPUT+="${1}"
    OUTPUT+="}"
    OUTPUT+="}"
    echo "${OUTPUT}"
}

# check whether we are root, exit if not
assert_running_as_root() {
    if ! [ "$(id -u)" = 0 ]; then
        echo "This script must be run as root." 1>&2
        exit 1
    fi
}

# function to check whether variables are either blank or set to the default go template value
# (because they haven't been set by the go script generator or a command line argument)
# returns 1 if the variable is set to a default/zero value
# returns 0 otherwise (i.e. it needs to be set interactively)
check_variable() {
    VARIABLE_VALUE="${!1}"
    GO_TEMPLATE_NAME=$(construct_go_template "${2}")
    if [[ "${VARIABLE_VALUE}" == "" ]] || [[ "${VARIABLE_VALUE}" == "${GO_TEMPLATE_NAME}" ]]; then
        return 1
    fi
    return 0
}

# function to check whether a provided value is "truthy" i.e. it looks like you're trying to say "yes"
is_truthy() {
    declare -a TRUTHY_VALUES
    TRUTHY_VALUES=("y" "Y" "yes" "YES" "ye" "YE" "yep" "YEP" "ya" "YA")
    CHECK_VALUE="$1"
    for ARRAY_VALUE in "${TRUTHY_VALUES[@]}"; do [[ "${CHECK_VALUE}" == "${ARRAY_VALUE}" ]] && return 0; done
    return 1
}

# function to read input until the value you get is non-empty
read_nonblank_input() {
    INPUT=""
    VARIABLE_TO_ASSIGN="$1"
    shift
    PROMPT="$*"
    until [[ "${INPUT}" != "" ]]; do
        echo -n "${PROMPT}"
        read -r INPUT
    done
    printf -v "${VARIABLE_TO_ASSIGN}" '%s' "${INPUT}"
}

# error if we're not root
assert_running_as_root

# set/read values interactively if not provided
# users will be prompted to enter their own value if all the following are true:
# - the current value is blank, or equal to the default Go template value
# - the value has not been provided by command line argument
! check_variable TELEPORT_VERSION version && INTERACTIVE=true && read_nonblank_input TELEPORT_VERSION "Enter Teleport version to install (without v): "
! check_variable TARGET_HOSTNAME hostname && INTERACTIVE=true && read_nonblank_input TARGET_HOSTNAME "Enter target hostname to connect to: "
! check_variable TARGET_PORT port && INTERACTIVE=true && { echo -n "Enter target port to connect to [${TARGET_PORT_DEFAULT}]: "; read -r TARGET_PORT; }
! check_variable JOIN_TOKEN token && INTERACTIVE=true && read_nonblank_input JOIN_TOKEN "Enter Teleport join token as provided: "
! check_variable CA_PIN_HASHES caPins && INTERACTIVE=true && read_nonblank_input CA_PIN_HASHES "Enter CA pin hash (separate multiple hashes with spaces): "
[ -n "${f}" ] && OVERRIDE_FORMAT=${f}
[ -n "${l}" ] && LOG_FILENAME=${l}
# if app service mode is not set (or is the default value) and we are running interactively (i.e. the user has provided some input already),
# prompt the user to choose whether to enable app_service
if [[ "${INTERACTIVE}" == "true" ]]; then
    if ! check_variable APP_INSTALL_MODE appInstallMode; then
        APP_INSTALL_MODE="false"
        echo -n "Would you like to enable and configure Teleport's app_service, to use Teleport as a reverse proxy for a web application? [y/n, default: n] "
        read -r APP_INSTALL_DECISION
        if is_truthy "${APP_INSTALL_DECISION}"; then
            APP_INSTALL_MODE="true"
        fi
    fi
fi
# prompt for extra needed values if we're running in app service mode
if [[ "${APP_INSTALL_MODE}" == "true" ]]; then
    ! check_variable APP_NAME appName && read_nonblank_input APP_NAME "Enter app name to install (must be DNS-compatible; less than 63 characters, no spaces, only - or _ as punctuation): "
    ! check_variable APP_URI appURI && read_nonblank_input APP_URI "Enter app URI (the host running the Teleport app service must be able to connect to this): "
    # generate app public addr by concatenating values
    APP_PUBLIC_ADDR="${APP_NAME}.${TARGET_HOSTNAME}"
fi

# set default target port if value not provided
if [[ "${TARGET_PORT}" == "" ]]; then
    TARGET_PORT=${TARGET_PORT_DEFAULT}
fi

# clear log file if provided
if [[ "${LOG_FILENAME}" != "" ]]; then
    if [ -f "${LOG_FILENAME}" ]; then
        echo -n "" > "${LOG_FILENAME}"
    fi
fi

# log functions
log_date() { echo -n "$(date '+%Y-%m-%d %H:%M:%S %Z')"; }
log() {
    LOG_LINE="$(log_date) [${SCRIPT_NAME}] $*"
    if [[ ${QUIET} != "true" ]]; then
        echo "${LOG_LINE}"
    fi
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> "${LOG_FILENAME}"
    fi
}
# writes a line with no timestamp or starting data, always prints
log_only() {
    LOG_LINE="$*"
    echo "${LOG_LINE}"
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> "${LOG_FILENAME}"
    fi
}
# writes a line by itself as a header
log_header() {
    LOG_LINE="$*"
    echo ""
    echo "${LOG_LINE}"
    echo ""
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> "${LOG_FILENAME}"
    fi
}
# important log lines, print even when -q (quiet) is passed
log_important() {
    LOG_LINE="$(log_date) [${SCRIPT_NAME}] ---> $*"
    echo "${LOG_LINE}"
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> "${LOG_FILENAME}"
    fi
}
log_cleanup_message() {
    log_only "This script does not overwrite any existing settings or Teleport installations."
    log_only "Please clean up by running any of the following steps as necessary:"
    if is_using_systemd; then
        log_only "- stop teleport's service"
        log_only "  - systemctl stop teleport"
    fi
    log_only "- stop any running Teleport processes"
    log_only "  - pkill -f teleport"
    log_only "- remove any data under ${TELEPORT_DATA_DIR}, along with the directory itself"
    log_only "  - rm -rf ${TELEPORT_DATA_DIR}"
    log_only "- remove any configuration at ${TELEPORT_CONFIG_PATH}"
    log_only "  - rm -f ${TELEPORT_CONFIG_PATH}"
    if check_exists apt; then
        log_only "- remove teleport package"
        log_only "  - apt remove teleport"
    elif check_exists yum; then
        log_only "- remove teleport package"
        log_only "  - yum remove teleport"
    elif check_exists dnf; then
        log_only "- remove teleport package"
        log_only "  - dnf remove teleport"
    elif check_exists zypper; then
        log_only "- remove teleport package"
        log_only "  - zypper remove teleport"
    else
        log_only "- remove any Teleport binaries (${TELEPORT_BINARY_LIST}) installed under ${TELEPORT_BINARY_DIR}"
        for BINARY in ${TELEPORT_BINARY_LIST}; do EXAMPLE_DELETE_COMMAND+="${TELEPORT_BINARY_DIR}/${BINARY} "; done
        log_only "  - rm -f ${EXAMPLE_DELETE_COMMAND}"
    fi
    if is_macos_host; then
        log_only "- unload and remove Teleport launchd config ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}"
        log_only "  - launchctl unload ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}"
        log_only "  - rm -f ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}"
    fi
    log_only "Run this installer again when done."
    log_only
}

# other functions
# check whether a named program exists
check_exists() { NAME=$1; if type "${NAME}" >/dev/null 2>&1; then return 0; else return 1; fi; }
# checks for the existence of a list of named binaries and exits with error if any of them don't exist
check_exists_fatal() {
    for TOOL in "$@"; do
        if ! check_exists "${TOOL}"; then
            log_important "Error: cannot find ${TOOL} - it needs to be installed"
            exit 1
        fi
    done
}
# check connectivity to the given host/port and make a request to see if Teleport is listening
# uses the global variable CONNECTIVITY_TEST_METHOD to return the name of the checker, as return
# values aren't really a thing that exists in bash
check_connectivity() {
    HOST=$1
    PORT=$2
    # check with nc
    if check_exists nc; then
        CONNECTIVITY_TEST_METHOD="nc"
        if nc -z -w3 "${HOST}" "${PORT}" >/dev/null 2>&1; then return 0; else return 1; fi
    # if there's no nc, check with telnet
    elif check_exists telnet; then
        CONNECTIVITY_TEST_METHOD="telnet"
        if echo -e '\x1dclose\x0d' | telnet "${HOST}" "${PORT}" >/dev/null 2>&1; then return 0; else return 1; fi
    # if there's no nc or telnet, try and use /dev/tcp
    elif [ -f /dev/tcp ]; then
        CONNECTIVITY_TEST_METHOD="/dev/tcp"
        if (head -1 < "/dev/tcp/${HOST}/${PORT}") >/dev/null 2>&1; then return 0; else return 1; fi
    else
        return 255
    fi
}
# check whether a teleport DEB is already installed and exit with error if so
check_deb_not_already_installed() {
    check_exists_fatal dpkg awk
    DEB_INSTALLED=$(dpkg -l | awk '{print $2}' | grep -E ^teleport || true)
    if [[ ${DEB_INSTALLED} != "" ]]; then
        log_important "It looks like there is already a Teleport DEB package installed (name: ${DEB_INSTALLED})."
        log_important "You will need to remove that package before using this script."
        exit 1
    fi
}
# check whether a teleport RPM is already installed and exit with error if so
check_rpm_not_already_installed() {
    check_exists_fatal rpm
    RPM_INSTALLED=$(rpm -qa | grep -E ^teleport || true)
    if [[ ${RPM_INSTALLED} != "" ]]; then
        log_important "It looks like there is already a Teleport RPM package installed (name: ${RPM_INSTALLED})."
        log_important "You will need to remove that package before using this script."
        exit 1
    fi
}
# function to check if given variable is set
check_set() {
    CHECK_KEY=${1} || true
    CHECK_VALUE=${!1} || true
    if [[ "${CHECK_VALUE}" == "" ]]; then
        log "Required variable ${CHECK_KEY} is not set"
        exit 1
    else
        log "${CHECK_KEY}: ${CHECK_VALUE}"
    fi
}
# checks that teleport binary can be found in path and runs 'teleport version'
check_teleport_binary() {
    FOUND_TELEPORT_VERSION=$(${TELEPORT_BINARY_DIR}/teleport version)
    if [[ "${FOUND_TELEPORT_VERSION}" == "" ]]; then
        log "Cannot find Teleport binary"
        return 1
    else
        log "Found: ${FOUND_TELEPORT_VERSION}";
        return 0
    fi
}
# wrapper to download with curl
download() {
    URL=$1
    OUTPUT_PATH=$2
    CURL_COMMAND="curl -fsSL --retry 5 --retry-delay 5"
    # optionally allow disabling of TLS verification (can be useful on older distros
    # which often have an out-of-date set of CA certificate bundle which won't validate)
    if [[ ${DISABLE_TLS_VERIFICATION} == "true" ]]; then
        CURL_COMMAND+=" -k"
    fi
    log "Running ${CURL_COMMAND} ${URL}"
    log "Downloading to ${OUTPUT_PATH}"
    # handle errors with curl
    if ! ${CURL_COMMAND} -o "${OUTPUT_PATH}" "${URL}"; then
        log_important "curl error downloading ${URL}"
        log "On an older OS, this may be related to the CA certificate bundle being too old."
        log "You can pass the hidden -k flag to this script to disable TLS verification - this is not recommended!"
        exit 1
    fi
    # check that the file has a non-zero size as an extra validation
    check_exists_fatal wc xargs
    FILE_SIZE="$(wc -c <"${OUTPUT_PATH}" | xargs)"
    if [ "${FILE_SIZE}" -eq 0 ]; then
        log_important "The downloaded file has a size of 0 bytes, which means an error occurred. Cannot continue."
        exit 1
    else
        log "Downloaded file size: ${FILE_SIZE} bytes"
    fi
    # if we have a hashing utility installed, also download and validate the checksum
    SHA_COMMAND=""
    # shasum is installed by default on macOS and some distros
    if check_exists shasum; then
        SHA_COMMAND="shasum -a 256"
    # sha256sum is installed by default in some other distros
    elif check_exists sha256sum; then
        SHA_COMMAND="sha256sum"
    fi
    if [[ "${SHA_COMMAND}" != "" ]]; then
        log "Will use ${SHA_COMMAND} to validate the checksum of the downloaded file"
        SHA_URL="${URL}.sha256"
        SHA_PATH="${OUTPUT_PATH}.sha256"
        ${CURL_COMMAND} -o "${SHA_PATH}" "${SHA_URL}"
        if ${SHA_COMMAND} --status -c "${SHA_PATH}"; then
            log "The downloaded file's checksum validated correctly"
        else
            SHA_EXPECTED=$(cat "${SHA_PATH}")
            SHA_ACTUAL=$(${SHA_COMMAND} "${OUTPUT_PATH}")
            if check_exists awk; then
                SHA_EXPECTED=$(echo "${SHA_EXPECTED}" | awk '{print $1}')
                SHA_ACTUAL=$(echo "${SHA_ACTUAL}" | awk '{print $1}')
            fi
            log_important "Checksum of the downloaded file did not validate correctly"
            log_important "Expected: ${SHA_EXPECTED}"
            log_important "Got: ${SHA_ACTUAL}"
            log_important "Try rerunning this script from the start. If the issue persists, contact Teleport support."
            exit 1
        fi
    else
        log "shasum/sha256sum utilities not found, will skip checksum validation"
    fi
}
# gets the filename from a full path (https://target.site/path/to/file.tar.gz -> file.tar.gz)
get_download_filename() { echo "${1##*/}"; }
# gets the pid of any running teleport process (and converts newlines to spaces)
get_teleport_pid() {
    check_exists_fatal pgrep xargs
    pgrep teleport | xargs echo
}
# returns a command which will start teleport using the config
get_teleport_start_command() {
    echo "${TELEPORT_BINARY_DIR}/teleport start --config=${TELEPORT_CONFIG_PATH}"
}
# installs the teleport-provided launchd config
install_launchd_config() {
    log "Installing Teleport launchd config to ${LAUNCHD_CONFIG_PATH}"
    ${COPY_COMMAND} ./${TELEPORT_ARCHIVE_PATH}/examples/launchd/${LAUNCHD_PLIST_FILE} ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}
}
# installs the teleport-provided systemd unit
install_systemd_unit() {
    log "Installing Teleport systemd unit to ${SYSTEMD_UNIT_PATH}"
    ${COPY_COMMAND} ./${TELEPORT_ARCHIVE_PATH}/examples/systemd/teleport.service ${SYSTEMD_UNIT_PATH}
    log "Reloading unit files (systemctl daemon-reload)"
    systemctl daemon-reload
}
# formats the arguments as a yaml list
get_yaml_list() {
    name="${1}"
    list="${2}"
    indentation="${3}"
    echo "${indentation}${name}:"
    for item in ${list}; do
        echo "${indentation}- ${item}"
    done
}

# installs the provided teleport config (for app service)
install_teleport_app_config() {
    log "Writing Teleport app service config to ${TELEPORT_CONFIG_PATH}"
    CA_PINS_CONFIG=$(get_yaml_list "ca_pin" "${CA_PIN_HASHES}" "  ")
    cat << EOF > ${TELEPORT_CONFIG_PATH}
version: v3
teleport:
  nodename: ${NODENAME}
  auth_token: ${JOIN_TOKEN}
${CA_PINS_CONFIG}
  proxy_server: ${TARGET_HOSTNAME}:${TARGET_PORT}
  log:
    output: stderr
    severity: INFO
auth_service:
  enabled: no
ssh_service:
  enabled: no
proxy_service:
  enabled: no
app_service:
  enabled: yes
  apps:
  - name: "${APP_NAME}"
    uri: "${APP_URI}"
    public_addr: ${APP_PUBLIC_ADDR}
EOF
}
# installs the provided teleport config (for database service)
install_teleport_database_config() {
    log "Writing Teleport database service config to ${TELEPORT_CONFIG_PATH}"
    CA_PINS_CONFIG=$(get_yaml_list "ca_pin" "${CA_PIN_HASHES}" "  ")

    # This file is processed by `shellschek` as part of the lint step
    # It detects an issue because of un-set variables - $index and $line. This check is called SC2154.
    # However, that's not an issue, because those variables are replaced when we run go's text/template engine over it.
    # When executing the script, those are no long variables but actual values.
    # shellcheck disable=SC2154
    cat << EOF > ${TELEPORT_CONFIG_PATH}
version: v3
teleport:
  nodename: ${NODENAME}
  auth_token: ${JOIN_TOKEN}
${CA_PINS_CONFIG}
  proxy_server: ${TARGET_HOSTNAME}:${TARGET_PORT}
  log:
    output: stderr
    severity: INFO
auth_service:
  enabled: no
ssh_service:
  enabled: no
proxy_service:
  enabled: no
db_service:
  enabled: "yes"
  resources:
EOF

    # Quoting the EOF heredoc indicates to shell to treat this as a literal string and does not try to interpolate or execute anything.
    cat << "EOF" >> ${TELEPORT_CONFIG_PATH}
    - labels:{{range $index, $line := .db_service_resource_labels}}
        {{$line -}}
{{end}}
EOF
}

# installs the provided teleport config (for Discovery Service)
install_teleport_discovery_config() {
    log "Writing Teleport discovery service config to ${TELEPORT_CONFIG_PATH}"
    CA_PINS_CONFIG=$(get_yaml_list "ca_pin" "${CA_PIN_HASHES}" "  ")

    # This file is processed by `shellschek` as part of the lint step
    # It detects an issue because of un-set variables - $index and $line. This check is called SC2154.
    # However, that's not an issue, because those variables are replaced when we run go's text/template engine over it.
    # When executing the script, those are no long variables but actual values.
    # shellcheck disable=SC2154
    cat << EOF > ${TELEPORT_CONFIG_PATH}
version: v3
teleport:
  nodename: ${NODENAME}
  auth_token: ${JOIN_TOKEN}
${CA_PINS_CONFIG}
  proxy_server: ${TARGET_HOSTNAME}:${TARGET_PORT}
  log:
    output: stderr
    severity: INFO
auth_service:
  enabled: no
ssh_service:
  enabled: no
proxy_service:
  enabled: no
discovery_service:
  enabled: "yes"
  discovery_group: "{{.discoveryGroup}}"
EOF
}

# installs the provided teleport config (for node service)
install_teleport_node_config() {
    log "Writing Teleport node service config to ${TELEPORT_CONFIG_PATH}"
    ${TELEPORT_BINARY_DIR}/teleport node configure \
      --silent \
      --token ${JOIN_TOKEN} \
      ${JOIN_METHOD_FLAG} \
      --ca-pin ${CA_PINS} \
      --proxy ${TARGET_HOSTNAME}:${TARGET_PORT} \
      "${LABELS_FLAG[@]}" \
      --output ${TELEPORT_CONFIG_PATH}
}
# checks whether the given host is running macOS
is_macos_host() { if [[ ${OSTYPE} == "darwin"* ]]; then return 0; else return 1; fi }
# checks whether teleport is already running on the host
is_running_teleport() {
    check_exists_fatal pgrep
    TELEPORT_PID=$(get_teleport_pid)
    if [[ "${TELEPORT_PID}" != "" ]]; then return 0; else return 1; fi
}
# checks whether the given host is running systemd as its init system
is_using_systemd() { if [ -d /run/systemd/system ]; then return 0; else return 1; fi }
# prints a warning if the host isn't running systemd
no_systemd_warning() {
    log_important "This host is not running systemd, so Teleport cannot be started automatically when it exits."
    log_important "Please investigate an alternative way to keep Teleport running."
    log_important "You can find information in our documentation: ${TELEPORT_DOCS_URL}"
    log_important "For now, Teleport will be started in the foreground - you can press Ctrl+C to exit."
    log_only
    log_only "Run this command to start Teleport in future:"
    log_only "$(get_teleport_start_command)"
    log_only
    log_only "------------------------------------------------------------------------"
    log_only "| IMPORTANT: TELEPORT WILL STOP RUNNING AFTER YOU CLOSE THIS TERMINAL! |"
    log_only "|   YOU MUST CONFIGURE A SERVICE MANAGER TO MAKE IT RUN ON STARTUP!    |"
    log_only "------------------------------------------------------------------------"
    log_only
}
# print a message giving the name of the node and a link to the docs
# gives some debugging instructions if the service didn't start successfully
print_welcome_message() {
    log_only ""
    if is_running_teleport; then
        log_only "Teleport has been started."
        log_only ""
        if is_using_systemd; then
            log_only "View its status with 'sudo systemctl status teleport.service'"
            log_only "View Teleport logs using 'sudo journalctl -u teleport.service'"
            log_only "To stop Teleport, run 'sudo systemctl stop teleport.service'"
            log_only "To start Teleport again if you stop it, run 'sudo systemctl start teleport.service'"
        elif is_macos_host; then
            log_only "View Teleport logs in '${MACOS_STDERR_LOG}' and '${MACOS_STDOUT_LOG}'"
            log_only "To stop Teleport, run 'sudo launchctl unload ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}'"
            log_only "To start Teleport again if you stop it, run 'sudo launchctl load ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}'"
        fi
        log_only ""
        log_only "You can see this node connected in the Teleport web UI or 'tsh ls' with the name '${NODENAME}'"
        log_only "Find more details on how to use Teleport here: https://goteleport.com/docs/"
    else
        log_important "The Teleport service was installed, but it does not appear to have started successfully."
        if is_using_systemd; then
            log_important "Check the Teleport service's status with 'systemctl status teleport.service'"
            log_important "View Teleport logs with 'journalctl -u teleport.service'"
        elif is_macos_host; then
            log_important "Check Teleport logs in '${MACOS_STDERR_LOG}' and '${MACOS_STDOUT_LOG}'"
        fi
        log_important "Contact Teleport support for further assistance."
    fi
    log_only ""
}
# start teleport in foreground (when there's no systemd)
start_teleport_foreground() {
    log "Starting Teleport in the foreground"
    # shellcheck disable=SC2091
    $(get_teleport_start_command)
}
# start teleport via launchd (after installing config)
start_teleport_launchd() {
    log "Starting Teleport via launchctl. It will automatically be started whenever the system reboots."
    launchctl load ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}
    sleep ${ALIVE_CHECK_DELAY}
}
# start teleport via systemd (after installing unit)
start_teleport_systemd() {
    log "Starting Teleport via systemd. It will automatically be started whenever the system reboots."
    systemctl enable teleport.service
    systemctl start teleport.service
    sleep ${ALIVE_CHECK_DELAY}
}
# checks whether teleport binaries exist on the host
teleport_binaries_exist() {
    for BINARY_NAME in teleport tctl tsh; do
        if [ -f ${TELEPORT_BINARY_DIR}/${BINARY_NAME} ]; then return 0; else return 1; fi
    done
}
# checks whether a teleport config exists on the host
teleport_config_exists() { if [ -f ${TELEPORT_CONFIG_PATH} ]; then return 0; else return 1; fi; }
# checks whether a teleport data dir exists on the host
teleport_datadir_exists() { if [ -d ${TELEPORT_DATA_DIR} ]; then return 0; else return 1; fi; }
# checks whether a launchd plist file for teleport already exists on the host
launchd_plist_file_exists() { if [ -f ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE} ]; then return 0; else return 1; fi; }

# error out if any required values are not set
check_set TELEPORT_VERSION
check_set TARGET_HOSTNAME
check_set TARGET_PORT
check_set JOIN_TOKEN
check_set CA_PIN_HASHES
if [[ "${APP_INSTALL_MODE}" == "true" ]]; then
    check_set APP_NAME
    check_set APP_URI
    check_set APP_PUBLIC_ADDR
fi

###
# main script starts here
###
# check connectivity to teleport server/port
if [[ "${IGNORE_CONNECTIVITY_CHECK}" == "true" ]]; then
    log "TELEPORT_IGNORE_CONNECTIVITY_CHECK=true, not running connectivity check"
else
    log "Checking TCP connectivity to Teleport server (${TARGET_HOSTNAME}:${TARGET_PORT})"
    if ! check_connectivity "${TARGET_HOSTNAME}" "${TARGET_PORT}"; then
        # if we don't have a connectivity test method assigned, we know we couldn't run the test
        if [[ ${CONNECTIVITY_TEST_METHOD} == "" ]]; then
            log "Couldn't find nc, telnet or /dev/tcp to do a connection test"
            log "Going to blindly continue without testing connectivity"
        else
            log_important "Couldn't open a connection to the Teleport server (${TARGET_HOSTNAME}:${TARGET_PORT}) via ${CONNECTIVITY_TEST_METHOD}"
            log_important "This issue will need to be fixed before the script can continue."
            log_important "If you think this is an error, add 'export TELEPORT_IGNORE_CONNECTIVITY_CHECK=true && ' before the curl command which runs the script."
            exit 1
        fi
    else
        log "Connectivity to Teleport server (via ${CONNECTIVITY_TEST_METHOD}) looks good"
    fi
fi

# use OSTYPE variable to figure out host type/arch
if [[ "${OSTYPE}" == "linux"* ]]; then

    if [[ "$UPDATER_STYLE" == "binary" ]]; then
      # if we are using the new updater, we can bypass this detection dance
      # and always use the updater.
      TELEPORT_FORMAT="updater"
    else
        # linux host, now detect arch
        TELEPORT_BINARY_TYPE="linux"
        ARCH=$(uname -m)
        log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
        if [[ ${ARCH} == "armv7l" ]]; then
            TELEPORT_ARCH="arm"
        elif [[ ${ARCH} == "aarch64" ]]; then
            TELEPORT_ARCH="arm64"
        elif [[ ${ARCH} == "x86_64" ]]; then
            TELEPORT_ARCH="amd64"
        elif [[ ${ARCH} == "i686" ]]; then
            TELEPORT_ARCH="386"
        else
            log_important "Error: cannot detect architecture from uname -m: ${ARCH}"
            exit 1
        fi
        log "Detected arch: ${ARCH}, using Teleport arch ${TELEPORT_ARCH}"
    fi
    # if the download format is already set, we have no need to detect distro
    if [[ ${TELEPORT_FORMAT} == "" ]]; then
        # detect distro
        # if /etc/os-release doesn't exist, we need to use some other logic
        if [ ! -f /etc/os-release ]; then
            if [ -f /etc/centos-release ]; then
                if grep -q 'CentOS release 6' /etc/centos-release; then
                    log_important "Detected host type: CentOS 6 [$(cat /etc/centos-release)]"
                    log_important "Teleport will not work on CentOS 6 -based servers due to the glibc version being too low."
                    exit 1
                fi
            elif [ -f /etc/redhat-release ]; then
                if grep -q 'Red Hat Enterprise Linux Server release 5' /etc/redhat-release; then
                    log_important "Detected host type: RHEL5 [$(cat /etc/redhat-release)]"
                    log_important "Teleport will not work on RHEL5-based servers due to the glibc version being too low."
                    exit 1
                elif grep -q 'Red Hat Enterprise Linux Server release 6' /etc/redhat-release; then
                    log_important "Detected host type: RHEL6 [$(cat /etc/redhat-release)]"
                    log_important "Teleport will not work on RHEL6-based servers due to the glibc version being too low."
                    exit 1
                fi
            fi
        # use ID_LIKE value from /etc/os-release (if set)
        # this is 'debian' on ubuntu/raspbian, 'centos rhel fedora' on amazon linux etc
        else
            check_exists_fatal cut
            DISTRO_TYPE=$(grep ID_LIKE /etc/os-release | cut -d= -f2) || true
            if [[ ${DISTRO_TYPE} == "" ]]; then
                # use exact ID value from /etc/os-release if ID_LIKE is not set
                DISTRO_TYPE=$(grep -w ID /etc/os-release | cut -d= -f2)
            fi
            if [[ ${DISTRO_TYPE} =~ "debian" ]]; then
                TELEPORT_FORMAT="deb"
            elif [[ "$DISTRO_TYPE" =~ "amzn"* ]] || [[ ${DISTRO_TYPE} =~ "centos"* ]] || [[ ${DISTRO_TYPE} =~ "rhel" ]] || [[ ${DISTRO_TYPE} =~ "fedora"* ]] || \
                     [[ ${DISTRO_TYPE} == *"suse"* ]] || [[ ${DISTRO_TYPE} =~ "sles"* ]]; then
                TELEPORT_FORMAT="rpm"
            else
                log "Couldn't match a distro type using /etc/os-release, falling back to tarball installer"
                TELEPORT_FORMAT="tarball"
            fi
        fi
        log "Detected distro type: ${DISTRO_TYPE}"
        #suse, also identified as sles, uses a different path for its systemd then other distro types like ubuntu
        if [[ ${DISTRO_TYPE} =~ "suse"* ]] || [[ ${DISTRO_TYPE} =~ "sles"* ]]; then
            SYSTEMD_UNIT_PATH="/etc/systemd/system/teleport.service"
        fi
    fi
elif [[ "${OSTYPE}" == "darwin"* ]]; then
    # macOS host, now detect arch
    TELEPORT_BINARY_TYPE="darwin"
    ARCH=$(uname -m)
    log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
    if [[ ${ARCH} == "arm64" ]]; then
        TELEPORT_ARCH="arm64"
    elif [[ ${ARCH} == "x86_64" ]]; then
        TELEPORT_ARCH="amd64"
    else
        log_important "Error: unsupported architecture from uname -m: ${ARCH}"
        exit 1
    fi
    log "Detected macOS ${ARCH} architecture, using Teleport arch ${TELEPORT_ARCH}"
    TELEPORT_FORMAT="tarball"
    if launchd_plist_file_exists; then
        log_header "Warning: Found existing Teleport launchd config ${LAUNCHD_CONFIG_PATH}/${LAUNCHD_PLIST_FILE}."
        log_cleanup_message
        exit 1
    fi
else
    log_important "Error - unsupported platform: ${OSTYPE}"
    exit 1
fi
log "Using Teleport distribution: ${TELEPORT_FORMAT}"

# create temporary directory and exit cleanup logic
TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)
log "Created temp dir ${TEMP_DIR}"
pushd "${TEMP_DIR}" >/dev/null 2>&1

finish() {
    popd >/dev/null 2>&1
    rm -rf "${TEMP_DIR}"
}
trap finish EXIT

# optional format override (mostly for testing)
if [[ ${OVERRIDE_FORMAT} != "" ]]; then
    TELEPORT_FORMAT="${OVERRIDE_FORMAT}"
    log "Overriding TELEPORT_FORMAT to ${OVERRIDE_FORMAT}"
fi

# check whether teleport is running already
# if it is, we exit gracefully with an error
if is_running_teleport; then
    if [[ ${IGNORE_CHECKS} != "true" ]]; then
        TELEPORT_PID=$(get_teleport_pid)
        log_header "Warning: Teleport appears to already be running on this host (pid: ${TELEPORT_PID})"
        log_cleanup_message
        exit 1
    else
        log "Ignoring is_running_teleport as requested"
    fi
fi

# check for existing config file
if teleport_config_exists; then
    if [[ ${IGNORE_CHECKS} != "true" ]]; then
        log_header "Warning: There is already a Teleport config file present at ${TELEPORT_CONFIG_PATH}."
        log_cleanup_message
        exit 1
    else
        log "Ignoring teleport_config_exists as requested"
    fi
fi

# check for existing data directory
if teleport_datadir_exists; then
    if [[ ${IGNORE_CHECKS} != "true" ]]; then
        log_header "Warning: Found existing Teleport data directory (${TELEPORT_DATA_DIR})."
        log_cleanup_message
        exit 1
    else
        log "Ignoring teleport_datadir_exists as requested"
    fi
fi

# check for existing binaries
if teleport_binaries_exist; then
    if [[ ${IGNORE_CHECKS} != "true" ]]; then
        log_header "Warning: Found existing Teleport binaries under ${TELEPORT_BINARY_DIR}."
        log_cleanup_message
        exit 1
    else
        log "Ignoring teleport_binaries_exist as requested"
    fi
fi

install_from_file() {
    # select correct URL/installation method based on distro
    if [[ ${TELEPORT_FORMAT} == "tarball" ]]; then
        URL="https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-v${TELEPORT_VERSION}-${TELEPORT_BINARY_TYPE}-${TELEPORT_ARCH}-bin.tar.gz"

        # check that needed tools are installed
        check_exists_fatal curl tar
        # download tarball
        log "Downloading Teleport ${TELEPORT_FORMAT} release ${TELEPORT_VERSION}"
        DOWNLOAD_FILENAME=$(get_download_filename "${URL}")
        download "${URL}" "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        # extract tarball
        tar -xzf "${TEMP_DIR}/${DOWNLOAD_FILENAME}" -C "${TEMP_DIR}"
        # install binaries to /usr/local/bin
        for BINARY in ${TELEPORT_BINARY_LIST}; do
            ${COPY_COMMAND} "${TELEPORT_ARCHIVE_PATH}/${BINARY}" "${TELEPORT_BINARY_DIR}/"
        done
    elif [[ ${TELEPORT_FORMAT} == "deb" ]]; then
        # convert teleport arch to deb arch
        if [[ ${TELEPORT_ARCH} == "amd64" ]]; then
            DEB_ARCH="amd64"
        elif [[ ${TELEPORT_ARCH} == "386" ]]; then
            DEB_ARCH="i386"
        elif [[ ${TELEPORT_ARCH} == "arm" ]]; then
            DEB_ARCH="arm"
        elif [[ ${TELEPORT_ARCH} == "arm64" ]]; then
            DEB_ARCH="arm64"
        fi
        URL="https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}_${TELEPORT_VERSION}_${DEB_ARCH}.deb"
        check_deb_not_already_installed
        # check that needed tools are installed
        check_exists_fatal curl dpkg
        # download deb and register cleanup operation
        log "Downloading Teleport ${TELEPORT_FORMAT} release ${TELEPORT_VERSION}"
        DOWNLOAD_FILENAME=$(get_download_filename "${URL}")
        download "${URL}" "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        # install deb
        log "Using dpkg to install ${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        dpkg -i "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
    elif [[ ${TELEPORT_FORMAT} == "rpm" ]]; then
        # convert teleport arch to rpm arch
        if [[ ${TELEPORT_ARCH} == "amd64" ]]; then
            RPM_ARCH="x86_64"
        elif [[ ${TELEPORT_ARCH} == "386" ]]; then
            RPM_ARCH="i386"
        elif [[ ${TELEPORT_ARCH} == "arm" ]]; then
            RPM_ARCH="arm"
        elif [[ ${TELEPORT_ARCH} == "arm64" ]]; then
            RPM_ARCH="arm64"
        fi
        URL="https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-${TELEPORT_VERSION}-1.${RPM_ARCH}.rpm"
        check_rpm_not_already_installed
        # check for package managers
        if check_exists dnf; then
            log "Found 'dnf' package manager, using it"
            PACKAGE_MANAGER_COMMAND="dnf -y install"
        elif check_exists yum; then
            log "Found 'yum' package manager, using it"
            PACKAGE_MANAGER_COMMAND="yum -y localinstall"
        elif check_exists zypper; then
            log "Found 'zypper' package manager, using it"
            PACKAGE_MANAGER_COMMAND="zypper --non-interactive install"
        else
            PACKAGE_MANAGER_COMMAND=""
            log "Cannot find 'yum' or 'dnf' package manager commands, will try installing the rpm manually instead"
        fi
        # check that needed tools are installed
        check_exists_fatal curl
        log "Downloading Teleport ${TELEPORT_FORMAT} release ${TELEPORT_VERSION}"
        DOWNLOAD_FILENAME=$(get_download_filename "${URL}")
        download "${URL}" "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        # install with package manager if available
        if [[ ${PACKAGE_MANAGER_COMMAND} != "" ]]; then
            log "Installing Teleport release from ${TEMP_DIR}/${DOWNLOAD_FILENAME} using ${PACKAGE_MANAGER_COMMAND}"
            # install rpm with package manager
            ${PACKAGE_MANAGER_COMMAND} "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        # use rpm if we couldn't find a package manager
        else
            # install RPM (in upgrade mode)
            log "Using rpm to install ${TEMP_DIR}/${DOWNLOAD_FILENAME}"
            rpm -Uvh "${TEMP_DIR}/${DOWNLOAD_FILENAME}"
        fi
    else
        log_important "Can't figure out what Teleport format to use"
        exit 1
    fi
}

install_from_repo() {
    if [[ "${REPO_CHANNEL}" == "" ]]; then
        # By default, use the current version's channel.
        REPO_CHANNEL=stable/v"${TELEPORT_VERSION//.*/}"
    fi

    # Populate $ID, $VERSION_ID, $VERSION_CODENAME and other env vars identifying the OS.
    # shellcheck disable=SC1091
    . /etc/os-release

    PACKAGE_LIST=$(package_list)
    if [ "$ID" == "debian" ] || [ "$ID" == "ubuntu" ]; then
        # old versions of ubuntu require that keys get added by `apt-key add`, without
        # adding the key apt shows a key signing error when installing teleport.
        if [[
            ($ID == "ubuntu" && $VERSION_ID == "16.04") || \
            ($ID == "debian" && $VERSION_ID == "9" )
        ]]; then
            apt install apt-transport-https gnupg -y
            curl -fsSL https://apt.releases.teleport.dev/gpg | apt-key add -
            echo "deb https://apt.releases.teleport.dev/${ID} ${VERSION_CODENAME} ${REPO_CHANNEL}" > /etc/apt/sources.list.d/teleport.list
        else
            curl -fsSL https://apt.releases.teleport.dev/gpg \
                -o /usr/share/keyrings/teleport-archive-keyring.asc
            echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \
            https://apt.releases.teleport.dev/${ID} ${VERSION_CODENAME} ${REPO_CHANNEL}" > /etc/apt/sources.list.d/teleport.list
        fi
        apt-get update
        apt-get install -y ${PACKAGE_LIST}
    elif [ "$ID" = "amzn" ] || [ "$ID" = "rhel" ] || [ "$ID" = "centos" ] || [ "$ID" = "rocky" ] || [ "$ID" = "almalinux" ]; then
        if [ "$ID" = "rocky" ] || [ "$ID" = "almalinux" ]; then
            ID="rhel" # Rocky and AlmaLinux are bug-for-bug compatible with rhel (including the VERSION_ID format).
        fi
        if [ "$ID" = "rhel" ]; then
            VERSION_ID="${VERSION_ID//.*/}" # convert version numbers like '7.2' to only include the major version
        fi
        yum install -y yum-utils
        yum-config-manager --add-repo \
        "$(rpm --eval "https://yum.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/${REPO_CHANNEL}/teleport.repo")"

        # Remove metadata cache to prevent cache from other channel (eg, prior version)
        # See: https://github.com/gravitational/teleport/issues/22581
        yum --disablerepo="*" --enablerepo="teleport" clean metadata

        yum install -y ${PACKAGE_LIST}
    elif [ "$ID" = "sles" ] || [ "$ID" = "opensuse-tumbleweed" ] || [ "$ID" = "opensuse-leap" ]; then
        if [ "$ID" = "opensuse-tumbleweed" ]; then
          VERSION_ID="15" # tumbleweed uses dated VERSION_IDs like 20230702
        else
          VERSION_ID="${VERSION_ID//.*/}" # convert version numbers like '7.2' to only include the major version
        fi
        sudo rpm --import "https://zypper.releases.teleport.dev/gpg"
        sudo zypper --non-interactive addrepo "$(rpm --eval "https://zypper.releases.teleport.dev/sles/$VERSION_ID/Teleport/%{_arch}/${REPO_CHANNEL}/teleport.repo")"
        sudo zypper --gpg-auto-import-keys refresh
        sudo zypper --non-interactive install ${PACKAGE_LIST}
    else
        echo "Unsupported distro: $ID"
        exit 1
    fi
}

install_from_updater() {
    SCRIPT_URL="https://$TARGET_HOSTNAME:$TARGET_PORT/scripts/install.sh"
    CURL_COMMAND="curl -fsS"
    if [[ ${DISABLE_TLS_VERIFICATION} == "true" ]]; then
        CURL_COMMAND+=" -k"
        SCRIPT_URL+="?insecure=true"
    fi

    log "Requesting the install script: $SCRIPT_URL"
    $CURL_COMMAND "$SCRIPT_URL" -o "$TEMP_DIR/install.sh" || (log "Failed to retrieve the install script." && exit 1)

    chmod +x "$TEMP_DIR/install.sh"

    log "Executing the install script"
    # We execute the install script because it might be a bash or sh script depending on the install script served.
    # This might cause issues if tmp is mounted with noexec, but the oneoff.sh script will also download and exec
    # binaries from tmp
    "$TEMP_DIR/install.sh"
}

# package_list returns the list of packages to install.
# The list of packages can be fed into yum or apt because they already have the expected format when pinning versions.
package_list() {
    TELEPORT_PACKAGE_PIN_VERSION=${TELEPORT_PACKAGE_NAME}
    TELEPORT_UPDATER_PIN_VERSION="${TELEPORT_PACKAGE_NAME}-updater"

    if [[ "${TELEPORT_FORMAT}" == "deb" ]]; then
        TELEPORT_PACKAGE_PIN_VERSION+="=${TELEPORT_VERSION}"
        TELEPORT_UPDATER_PIN_VERSION+="=${TELEPORT_VERSION}"

    elif [[ "${TELEPORT_FORMAT}" == "rpm" ]]; then
        TELEPORT_YUM_VERSION="${TELEPORT_VERSION//-/_}"
        TELEPORT_PACKAGE_PIN_VERSION+="-${TELEPORT_YUM_VERSION}"
        TELEPORT_UPDATER_PIN_VERSION+="-${TELEPORT_YUM_VERSION}"
    fi

    PACKAGE_LIST=${TELEPORT_PACKAGE_PIN_VERSION}
    # (warning): This expression is constant. Did you forget the $ on a variable?
    # Disabling the warning above because expression is templated.
    # shellcheck disable=SC2050
    if is_using_systemd && [[ "$UPDATER_STYLE" == "package" ]]; then
        # Teleport Updater requires systemd.
        PACKAGE_LIST+=" ${TELEPORT_UPDATER_PIN_VERSION}"
    fi
    echo ${PACKAGE_LIST}
}

is_repo_available() {
    if [[ "${OSTYPE}" != "linux"* ]]; then
        return 1
    fi

    # Populate $ID, $VERSION_ID and other env vars identifying the OS.
    # shellcheck disable=SC1091
    . /etc/os-release

    # The following distros+version have a Teleport repository to install from.
    case "${ID}-${VERSION_ID}" in
        ubuntu-16.04* | ubuntu-18.04* | ubuntu-20.04* | ubuntu-22.04* | ubuntu-24.04* |\
        debian-9* | debian-10* | debian-11* | debian-12* | \
        rhel-7* | rhel-8* | rhel-9* | \
        centos-7* | centos-8* | centos-9* | \
        amzn-2 | amzn-2023 | \
        opensuse-tumbleweed* | sles-12* | sles-15* | opensuse-leap-15*)
            return 0;;
    esac

    return 1
}

if [[ "$TELEPORT_FORMAT" == "updater" ]]; then
    log "Installing from updater binary."
    install_from_updater
elif is_repo_available; then
    log "Installing repo for distro $ID."
    install_from_repo
else
    log "Installing from binary file."
    install_from_file
fi

# check that teleport binary can be found and runs
if ! check_teleport_binary; then
    log_important "The Teleport binary could not be found at ${TELEPORT_BINARY_DIR} as expected."
    log_important "This usually means that there was an error during installation."
    log_important "Check this log for obvious signs of error and contact Teleport support"
    log_important "for further assistance."
    exit 1
fi

# install teleport config
# check the mode and write the appropriate config type
if [[ "${APP_INSTALL_MODE}" == "true" ]]; then
    install_teleport_app_config
elif [[ "${DB_INSTALL_MODE}" == "true" ]]; then
    install_teleport_database_config
elif [[ "${DISCOVERY_INSTALL_MODE}" == "true" ]]; then
    install_teleport_discovery_config
else
    install_teleport_node_config
fi


# Used to track whether a Teleport agent was installed using this method.
export TELEPORT_INSTALL_METHOD_NODE_SCRIPT="true"

# install systemd unit if applicable (linux hosts)
if is_using_systemd; then
    log "Host is using systemd"
    # we only need to manually install the systemd config if teleport was installed via tarball
    # all other packages will deploy it automatically
    if [[ ${TELEPORT_FORMAT} == "tarball" ]]; then
        install_systemd_unit
    fi
    start_teleport_systemd
    print_welcome_message
# install launchd config on macOS hosts
elif is_macos_host; then
    log "Host is running macOS"
    install_launchd_config
    start_teleport_launchd
    print_welcome_message
# not a macOS host and no systemd available, print a warning
# and temporarily start Teleport in the foreground
else
    log "Host does not appear to be using systemd"
    no_systemd_warning
    start_teleport_foreground
fi
