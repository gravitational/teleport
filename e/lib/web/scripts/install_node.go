package scripts

import (
	"net/http"
	"text/template"

	"github.com/gravitational/teleport/lib/httplib"
)

// SetScriptHeaders sets response headers to plain text.
func SetScriptHeaders(h http.Header) {
	httplib.SetNoCacheHeaders(h)
	httplib.SetNoSniff(h)
	h.Set("Content-Type", "text/plain")
}

// ErrorBashScript is used to display friendly error message when
// there is an error prepping the actual script.
const ErrorBashScript = `
#!/bin/sh
echo -e "An error has occurred. \nThe token may be expired or invalid. \nPlease check log for further details."
exit 1
`

// InstallNodeBashScript is the script that will run on user's machine
// to install teleport and join a teleport cluster.
var InstallNodeBashScript = template.Must(template.New("nodejoin").Parse(`
#!/bin/bash
set -euo pipefail
SCRIPT_NAME="teleport-node-installer"

# default values
CONNECTIVITY_TEST_METHOD=""
COPY_COMMAND="cp"
DISTRO_TYPE=""
LAUNCHD_CONFIG_PATH="/Library/LaunchDaemons"
LOG_FILENAME="${TMPDIR:-/tmp}/${SCRIPT_NAME}.log"
MACOS_STDERR_LOG="/var/log/teleport-stderr.log"
MACOS_STDOUT_LOG="/var/log/teleport-stdout.log"
SYSTEMD_UNIT_PATH="/lib/systemd/system/teleport.service"
TARGET_PORT_DEFAULT=3080
TELEPORT_ARCHIVE_PATH="teleport"
TELEPORT_BINARY_DIR="/usr/local/bin"
TELEPORT_BINARY_LIST="teleport tctl tsh"
TELEPORT_CONFIG_PATH="/etc/teleport.yaml"
TELEPORT_DATA_DIR="/var/lib/teleport"
TELEPORT_DOCS_URL="https://gravitational.com/teleport/docs"
TELEPORT_FORMAT=""

# initialise variables (because set -u disallows unbound variables)
f=""
l=""
DISABLE_TLS_VERIFICATION=false
NODENAME=$(hostname)
IGNORE_CHECKS=false
OVERRIDE_FORMAT=""
QUIET=false

# the default value of each variable is a templatable Go value so that it can
# optionally be replaced by the server before the script is served up
TELEPORT_VERSION="{{.version}}"
TARGET_HOSTNAME="{{.hostname}}"
TARGET_PORT="{{.port}}"
NODE_JOIN_TOKEN="{{.token}}"
CA_PIN_HASH="{{.caPin}}"

# usage mesage
# shellcheck disable=SC2086
usage() { echo "Usage: $(basename $0) [-v teleport_version] [-h target_hostname] [-p target_port> [-j node_join_token] [-c ca_pin_hash] [-q] [-l log_filename]" 1>&2; exit 1; }
while getopts ":v:h:p:j:c:f:ql:ik" o; do
    case "${o}" in
        v)  TELEPORT_VERSION=${OPTARG};;
        h)  TARGET_HOSTNAME=${OPTARG};;
        p)  TARGET_PORT=${OPTARG};;
        j)  NODE_JOIN_TOKEN=${OPTARG};;
        c)  CA_PIN_HASH=${OPTARG};;
        f)  f=${OPTARG}; if [[ ${f} != "tarball" && ${f} != "deb" && ${f} != "rpm" && ${f} != "rpm-centos6" ]]; then usage; fi;;
        q)  QUIET=true;;
        l)  l=${OPTARG};;
        i)  IGNORE_CHECKS=true; COPY_COMMAND="cp -f";;
        k)  DISABLE_TLS_VERIFICATION=true;;
        *)  usage;;
    esac
done
shift $((OPTIND-1))

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

# set/read values interactively if not provided
# users will be prompted to enter their own value if all the following are true:
# - the current value is blank, or equal to the default Go template value
# - the value has not been provided by command line argument
! check_variable TELEPORT_VERSION version && { echo -n "Enter Teleport version to install (without v): "; read -r TELEPORT_VERSION; }
! check_variable TARGET_HOSTNAME hostname && { echo -n "Enter target hostname to connect node to: "; read -r TARGET_HOSTNAME; }
! check_variable TARGET_PORT port && { echo -n "Enter target port to connect node to [${TARGET_PORT_DEFAULT}]: "; read -r TARGET_PORT; }
! check_variable NODE_JOIN_TOKEN token && { echo -n "Enter Teleport node join token as provided: "; read -r NODE_JOIN_TOKEN; }
! check_variable CA_PIN_HASH caPin && { echo -n "Enter CA pin hash: "; read -r CA_PIN_HASH; }
[ -n "${f}" ] && OVERRIDE_FORMAT=${f}
[ -n "${l}" ] && LOG_FILENAME=${l}

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
    log_only "- stop any running Teleport processes"
    log_only "  - pkill -f teleport"
    log_only "- remove any data under ${TELEPORT_DATA_DIR}, along with the directory itself"
    log_only "  - rm -rf ${TELEPORT_DATA_DIR}"
    log_only "- remove any configuration at ${TELEPORT_CONFIG_PATH}"
    log_only "  - rm -f ${TELEPORT_CONFIG_PATH}"
    log_only "- remove any Teleport binaries (${TELEPORT_BINARY_LIST}) installed under ${TELEPORT_BINARY_DIR}"
    for BINARY in ${TELEPORT_BINARY_LIST}; do EXAMPLE_DELETE_COMMAND+="${TELEPORT_BINARY_DIR}/${BINARY} "; done
    log_only "  - rm -f ${EXAMPLE_DELETE_COMMAND}"
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
        if telnet -c "${HOST}" "${PORT}" </dev/null >/dev/null 2>&1 | grep -q Connected; then return 0; else return 1; fi
    # if there's no nc or telnet, try and use /dev/tcp
    elif [ -f /dev/tcp ]; then
        CONNECTIVITY_TEST_METHOD="/dev/tcp"
        if (head -1 < "/dev/tcp/${HOST}/${PORT}") >/dev/null 2>&1; then return 0; else return 1; fi
    else
        return 255
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
    # shasum is installed by default on MacOS and some distros
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
            log_important "Try rerunning this script from the start. If the issue persists, contact Gravitational Support."
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
    ${COPY_COMMAND} ./${TELEPORT_ARCHIVE_PATH}/examples/launchd/teleport.plist ${LAUNCHD_CONFIG_PATH}/teleport.plist
}
# installs the teleport-provided systemd unit
install_systemd_unit() {
    log "Installing Teleport systemd unit to ${SYSTEMD_UNIT_PATH}"
    ${COPY_COMMAND} ./${TELEPORT_ARCHIVE_PATH}/examples/systemd/teleport.service ${SYSTEMD_UNIT_PATH}
    log "Reloading unit files (systemctl daemon-reload)"
    systemctl daemon-reload
}
# installs the provided teleport config
install_teleport_config() {
    log "Writing Teleport config to ${TELEPORT_CONFIG_PATH}"
    cat << EOF > ${TELEPORT_CONFIG_PATH}
teleport:
  nodename: ${NODENAME}
  auth_token: ${NODE_JOIN_TOKEN}
  ca_pin: ${CA_PIN_HASH}
  auth_servers:
  - ${TARGET_HOSTNAME}:${TARGET_PORT}
  log:
    output: stderr
    severity: INFO
auth_service:
  enabled: no
ssh_service:
  enabled: yes
proxy_service:
  enabled: no
EOF
}
# checks whether the given host is running MacOS
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
    log_important "For now, Teleport will be started in the foregound - you can press Ctrl+C to exit."
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
            log_only "To stop Teleport, run 'sudo launchctl unload ${LAUNCHD_CONFIG_PATH}/teleport.plist'"
            log_only "To start Teleport again if you stop it, run 'sudo launchctl load ${LAUNCHD_CONFIG_PATH}/teleport.plist'"
        fi
        log_only ""
        log_only "You can see this node connected in the Teleport web UI or 'tsh ls' with the name '${NODENAME}'"
        log_only "Find more details on how to use Teleport here: https://gravitational.com/teleport/docs/user-manual/"
    else
        log_important "The Teleport service was installed, but it does not appear to have started successfully."
        if is_using_systemd; then
            log_important "Check the Teleport service's status with 'systemctl status teleport.service'"
            log_important "View Teleport logs with 'journalctl -u teleport.service'"
        elif is_macos_host; then
            log_important "Check Teleport logs in '${MACOS_STDERR_LOG}' and '${MACOS_STDOUT_LOG}'"
        fi
        log_important "Contact Gravitational support for further assistance."
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
    launchctl load ${LAUNCHD_CONFIG_PATH}/teleport.plist
}
# start teleport via systemd (after installing unit)
start_teleport_systemd() {
    log "Starting Teleport via systemd. It will automatically be started whenever the system reboots."
    systemctl enable teleport.service
    systemctl start teleport.service
}
# checks whether teleport binaries eist on the host
teleport_binaries_exist() {
    for BINARY_NAME in teleport tctl tsh; do
        if [ -f ${TELEPORT_BINARY_DIR}/${BINARY_NAME} ]; then return 0; else return 1; fi
    done
}
# checks whether a teleport config exists on the host
teleport_config_exists() { if [ -f ${TELEPORT_CONFIG_PATH} ]; then return 0; else return 1; fi; }
# checks whether a teleport data dir exists on the host
teleport_datadir_exists() { if [ -d ${TELEPORT_DATA_DIR} ]; then return 0; else return 1; fi; }

# error out if any required values are not set
check_set TELEPORT_VERSION
check_set TARGET_HOSTNAME
check_set TARGET_PORT
check_set NODE_JOIN_TOKEN
check_set CA_PIN_HASH

###
# main script starts here
###
# check connectivity to teleport server/port
log "Checking TCP connectivity to Teleport server (${TARGET_HOSTNAME}:${TARGET_PORT})"
if ! check_connectivity "${TARGET_HOSTNAME}" "${TARGET_PORT}"; then
    # if we don't have a connectivity test method assigned, we know we couldn't run the test
    if [[ ${CONNECTIVITY_TEST_METHOD} == "" ]]; then
        log "Couldn't find nc, telnet or /dev/tcp to do a connection test"
        log "Going to blindly continue without testing connectivity"
    else
        log_important "Couldn't open a connection to the Teleport server (${TARGET_HOSTNAME}:${TARGET_PORT})"
        log_important "This issue will need to be fixed before the script can continue."
        exit 1
    fi
else
    log "Connectivity to Teleport server (via ${CONNECTIVITY_TEST_METHOD}) looks good"
fi

# use OSTYPE variable to figure out host type/arch
if [[ "${OSTYPE}" == "linux-gnu"* ]]; then
    # linux host, now detect arch
    TELEPORT_BINARY_TYPE="linux"
    ARCH=$(uname -m)
    log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
    if [[ ${ARCH} == "armv7l" ]]; then
        TELEPORT_ARCH="arm"
        TELEPORT_FORMAT="tarball"
    elif [[ ${ARCH} == "aarch64" ]]; then
        TELEPORT_ARCH="aarch64"
        log_important "Error: detected ${ARCH} but Teleport doesn't build binaries for this architecture yet, exiting"
    elif [[ ${ARCH} == "x86_64" ]]; then
        TELEPORT_ARCH="amd64"
    elif [[ ${ARCH} == "i686" ]]; then
        TELEPORT_ARCH="386"
    else
        log_important "Error: cannot detect architecture from uname -m: ${ARCH}"
        exit 1
    fi
    log "Detected arch: ${ARCH}, using Teleport arch ${TELEPORT_ARCH}"
    # if the download format is already set, we have no need to detect distro
    if [[ ${TELEPORT_FORMAT} == "" ]]; then
        # detect distro
        # if /etc/os-release doesn't exist, we need to use some other logic
        if [ ! -f /etc/os-release ]; then
            if [ -f /etc/centos-release ]; then
                if grep -q 'CentOS release 6' /etc/centos-release; then
                    DISTRO_TYPE="centos6"
                    TELEPORT_FORMAT="rpm-centos6"
                fi
            elif [ -f /etc/redhat-release ]; then
                if grep -q 'Red Hat Enterprise Linux Server release 5' /etc/redhat-release; then
                    log_important "Detected host type: RHEL5 [$(cat /etc/redhat-release)]"
                    log_important "Teleport will not work on RHEL5-based servers due to the glibc version being too low."
                    exit 1
                elif grep -q 'Red Hat Enterprise Linux Server release 6' /etc/redhat-release; then
                    DISTRO_TYPE="redhat"
                    TELEPORT_FORMAT="rpm-centos6"
                    log "Detected host type: RHEL6 [$(cat /etc/redhat-release)]"
                fi
            fi
        # use ID_LIKE value from /etc/os-release (if set)
        # this is 'debian' on ubuntu/raspian, 'centos rhel fedora' on amazon linux etc
        else
            check_exists_fatal cut
            DISTRO_TYPE=$(grep ID_LIKE /etc/os-release | cut -d= -f2) || true
            if [[ ${DISTRO_TYPE} == "" ]]; then
                # use exact ID value from /etc/os-release if ID_LIKE is not set
                DISTRO_TYPE=$(grep -w ID /etc/os-release | cut -d= -f2)
            fi
            if [[ ${DISTRO_TYPE} =~ "debian" ]]; then
                TELEPORT_FORMAT="deb"
            elif [[ ${DISTRO_TYPE} =~ "centos"* ]] || [[ ${DISTRO_TYPE} =~ "rhel" ]] || [[ ${DISTRO_TYPE} =~ "fedora"* ]]; then
                TELEPORT_FORMAT="rpm"
            else
                log "Couldn't match a distro type using /etc/os-release, falling back to tarball installer"
                TELEPORT_FORMAT="tarball"
            fi
        fi
        log "Detected distro type: ${DISTRO_TYPE}"
    fi
elif [[ "${OSTYPE}" == "darwin"* ]]; then
    # macos host, now detect arch
    TELEPORT_BINARY_TYPE="darwin"
    ARCH=$(uname -m)
    log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
    if [[ ${ARCH} == "aarch64" ]]; then
        TELEPORT_ARCH="aarch64"
        log_important "Error: detected ${ARCH} but Teleport doesn't build binaries for this architecture yet, exiting"
    elif [[ ${ARCH} == "x86_64" ]]; then
        TELEPORT_ARCH="amd64"
    else
        log_important "Error: unsupported architecture from uname -m: ${ARCH}"
        exit 1
    fi
    log "Detected MacOS ${ARCH} architecture, using Teleport arch ${TELEPORT_ARCH}"
    TELEPORT_FORMAT="tarball"
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
    log "Cleaning up temp dir ${TEMP_DIR}"
    rm -rf "${TEMP_DIR}"
}
trap finish EXIT

# optional format override (mostly for testing)
if [[ ${OVERRIDE_FORMAT} != "" ]]; then
    TELEPORT_FORMAT="${OVERRIDE_FORMAT}"
    log "Overriding TELEPORT_FORMAT to ${OVERRIDE_FORMAT}"
fi

# check whether teleport is running already
# if it is, we exit gracefully with an eror
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
        log_header "Warning: Found existing Teleport data under ${TELEPORT_DATA_DIR}."
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

# handle centos6 installations
if [[ ${TELEPORT_FORMAT} == "rpm-centos6" ]]; then
    # override the format to 'tarball' (as that's how centos6 binaries are packaged)
    # also override DISTRO_TYPE to centos6 as that's used for the URL check below
    log "Overriding format for centos6 installation to use tarball"
    TELEPORT_FORMAT="tarball"
    DISTRO_TYPE="centos6"
fi

# select correct URL/installation method based on distro
if [[ ${TELEPORT_FORMAT} == "tarball" ]]; then
    # handle centos6 URL override
    if [[ ${DISTRO_TYPE} == "centos6" ]]; then
        URL="https://get.gravitational.com/teleport-v${TELEPORT_VERSION}-${TELEPORT_BINARY_TYPE}-${TELEPORT_ARCH}-centos6-bin.tar.gz"
    else
        URL="https://get.gravitational.com/teleport-v${TELEPORT_VERSION}-${TELEPORT_BINARY_TYPE}-${TELEPORT_ARCH}-bin.tar.gz"
    fi
    # check that needed tools are installed
    check_exists_fatal tar
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
    fi
    URL="https://get.gravitational.com/teleport_${TELEPORT_VERSION}_${DEB_ARCH}.deb"
    check_exists_fatal dpkg
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
    fi
    URL="https://get.gravitational.com/teleport-${TELEPORT_VERSION}-1.${RPM_ARCH}.rpm"
    # check for package managers
    if check_exists dnf; then
        log "Found 'dnf' package manager, using it"
        PACKAGE_MANAGER_COMMAND="dnf -y install"
    elif check_exists yum; then
        log "Found 'yum' package manager, using it"
        PACKAGE_MANAGER_COMMAND="yum -y localinstall"
    else
        PACKAGE_MANAGER_COMMAND=""
        log "Cannot find 'yum' or 'dnf' package manager commands, will try installing the rpm manually instead"
        # check that needed tools are installed
        check_exists_fatal rpm
    fi
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

# check that teleport binary can be found and runs
if ! check_teleport_binary; then
    log_important "The Teleport binary could not be found at ${TELEPORT_BINARY_DIR} as expected."
    log_important "This usually means that there was an error during installation."
    log_important "Check this log for obvious signs of error and contact Gravitational Support"
    log_important "for further assistance."
    exit 1
fi

# install teleport config
install_teleport_config

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
# install launchd config on MacOS hosts
elif is_macos_host; then
    log "Host is running MacOS"
    install_launchd_config
    start_teleport_launchd
    print_welcome_message
# not a MacOS host and no systemd available, print a warning
# and temporarily start Teleport in the foreground
else
    log "Host does not appear to be using systemd"
    no_systemd_warning
    start_teleport_foreground
fi
`))
