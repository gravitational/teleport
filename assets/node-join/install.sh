#!/bin/bash
set -euo pipefail
SCRIPT_NAME="teleport-node-installer"

# default values
LAUNCHD_CONFIG_PATH="/Library/LaunchDaemons"
LOG_FILENAME="${TMPDIR:-/tmp}/${SCRIPT_NAME}.log"
SYSTEMD_UNIT_PATH="/etc/systemd/system/teleport.service"
TARGET_PORT_DEFAULT=3080
TELEPORT_BINARY_DIR="/usr/local/bin"
TELEPORT_CONFIG_PATH="/etc/teleport.yaml"
TELEPORT_DATA_DIR="/var/lib/teleport"
TELEPORT_DOCS_URL="https://gravitational.com/teleport/docs"

# initialise variables
v=""
h=""
p=""
j=""
c=""
f=""
l=""
QUIET=false
OVERRIDE_FORMAT=""

# usage mesage
usage() { echo "Usage: $(basename $0) -q -l <log filename> [-v <teleport version>] [-h <target hostname>] <-p <target port>> [-j <node join token>] [-c <ca pin hash>]" 1>&2; exit 1; }
while getopts ":v:h:p:j:c:f:ql:" o; do
    case "${o}" in
        v)
            v=${OPTARG}
            ;;
        h)
            h=${OPTARG}
            ;;
        p)
            p=${OPTARG}
            ;;
        j)
            j=${OPTARG}
            ;;
        c)
            c=${OPTARG}
            ;;
        f)
            f=${OPTARG}
            if [[ ${f} != "tarball" && ${f} != "deb" && ${f} != "rpm" && ${f} != "rpm-centos6" ]]; then usage; fi
            ;;
        q)
            QUIET=true
            ;;
        l)
            l=${OPTARG}
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

# set/read values interactively if not provided
[ -n "${v}" ] && TELEPORT_VERSION=${v} || { echo -n "Enter Teleport version to install (without v): "; read TELEPORT_VERSION; }
[ -n "${h}" ] && TARGET_HOSTNAME=${h} || { echo -n "Enter target hostname to connect node to: "; read TARGET_HOSTNAME; }
[ -n "${p}" ] && TARGET_PORT=${p} || { echo -n "Enter target port to connect node to [default: 3080]: "; read TARGET_PORT; }
[ -n "${j}" ] && NODE_JOIN_TOKEN=${j} || { echo -n "Enter Teleport node join token as provided: "; read NODE_JOIN_TOKEN; }
[ -n "${c}" ] && CA_PIN_HASH=${c} || { echo -n "Enter CA pin hash: "; read CA_PIN_HASH; }
[ -n "${f}" ] && OVERRIDE_FORMAT=${f}
[ -n "${l}" ] && LOG_FILENAME=${l}

# set default target port if value not provided
if [[ "${TARGET_PORT}" == "" ]]; then
    TARGET_PORT=${TARGET_PORT_DEFAULT}
fi

# function to check if given variable is set
check_set() {
    if [[ $1 == "" ]]; then
        log "Required variable $1 is not set"
        exit 1
    fi
}

# error out any required values are not set
check_set TELEPORT_VERSION
check_set TARGET_HOSTNAME
check_set TARGET_PORT
check_set NODE_JOIN_TOKEN
check_set CA_PIN_HASH

# clear log file if provided
if [[ "${LOG_FILENAME}" != "" ]]; then
    if [ -f ${LOG_FILENAME} ]; then
        echo -n "" > ${LOG_FILENAME}
    fi
fi

# log functions
log_date() { echo -n $(date "+%Y-%m-%d %H:%M:%S %Z"); }
log() {
    LOG_LINE="$(log_date) [${SCRIPT_NAME}] $*"
    if [[ ${QUIET} != "true" ]]; then
        echo "${LOG_LINE}"
    fi
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> ${LOG_FILENAME}
    fi
}
# writes a line with no timestamp or starting data, always prints
log_only() {
    LOG_LINE="$*"
    echo "${LOG_LINE}"
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> ${LOG_FILENAME}
    fi
}
# writes a line by itself as a header
log_header() {
    LOG_LINE="$*"
    echo ""
    echo "${LOG_LINE}"
    echo ""
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> ${LOG_FILENAME}
    fi
}
# important log lines, print even when -q (quiet) is passed
log_important() {
    LOG_LINE="$(log_date) [${SCRIPT_NAME}] ---> $*"
    echo "${LOG_LINE}"
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> ${LOG_FILENAME}
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
    log_only "- remove any Teleport binaries (teleport, tctl, tsh) installed under ${TELEPORT_BINARY_DIR}"
    log_only "  - rm -f ${TELEPORT_BINARY_DIR}/teleport ${TELEPORT_BINARY_DIR}/tctl ${TELEPORT_BINARY_DIR}/tsh"
    log_only "Run this installer again when done."
    log_only
}

# other functions
# download wrapper for either wget or curl
download() {
    URL=$1
    OUTPUT_PATH=$2
    # use curl if we can
    if check_exists curl; then
        curl -Ls -o ${OUTPUT_PATH} ${URL}
    # fall back to wget
    elif check_exists wget; then
        wget -q -nv -O ${OUTPUT_PATH} ${URL}
    else
        log_important "Can't find curl or wget to use for downloads, one of these is required to function"
        exit 1
    fi
}
# check whether a named program exists
check_exists() { NAME=$1; if type ${NAME} >/dev/null 2>&1; then return 0; else return 1; fi; }
# checks for the existence of a list of named binaries and exits with error if any of them don't exist
check_exists_fatal() {
    for TOOL in "$@"; do
        if ! check_exists ${TOOL}; then
            log_important "Error: cannot find ${TOOL} - it needs to be installed"
            exit 1
        fi
    done
}
# checks whether the given host is running MacOS
is_macos_host() {
    if [[ ${OSTYPE} == "darwin"* ]]; then
        log "Host is running MacOS"
        return 0
    else
        log "Host is not running MacOS"
        return 1
    fi
}
# checks whether the given host is running systemd as its init system
is_using_systemd() {
    check_exists_fatal grep
    if cat /proc/1/cmdline | grep -q systemd; then
        log "Host is using systemd as pid 1"
        return 0
    else
        log "Host does not appear to be running systemd"
        return 1
    fi
}
# installs the teleport-provided launchd config
install_launchd_config() {
    log "Installing Teleport launchd config to ${LAUNCHD_CONFIG_PATH}"
    cp -f ./teleport/examples/launchd/teleport.plist ${LAUNCHD_CONFIG_PATH}/teleport.plist
}
# installs the teleport-provided systemd unit
install_systemd_unit() {
    log "Installing Teleport systemd unit to ${SYSTEMD_UNIT_PATH}"
    cp ./teleport/examples/systemd/teleport.service ${SYSTEMD_UNIT_PATH}
    log "Reloading unit files (systemctl daemon-reload)"
    systemctl daemon-reload
}
# start teleport via launchd (after installing config)
start_teleport_launchd() {
    log "Starting Teleport via launchctl"
    launchctl load ${LAUNCHD_CONFIG_PATH}/teleport.plist
}
# start teleport via systemd (after installing unit)
start_teleport_systemd() {
    log "Starting Teleport via systemd"
    systemctl start teleport
}
# start teleport in foreground (when there's no systemd)
start_teleport_foreground() {
    log "Starting Teleport in the foreground"
    teleport start --config=${TELEPORT_CONFIG_PATH}
}
# installs the provided teleport config
install_teleport_config() {
    log "Writing Teleport config to ${TELEPORT_CONFIG_PATH}"
    cat << EOF > ${TELEPORT_CONFIG_PATH}
teleport:
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
# checks whether teleport is already running on the host
is_running_teleport() {
    check_exists_fatal pgrep
    TELEPORT_PID=$(pgrep -d " " teleport)
    if [[ "${TELEPORT_PID}" != "" ]]; then
        log "Teleport appears to already be running (pid: ${TELEPORT_PID})"
        return 0
    else
        log "Teleport does not appear to already be running"
        return 1
    fi

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
# prints a warning if the host isn't running systemd
no_systemd_warning() {
    log_important "This host is not running systemd, so Teleport cannot be started automatically when it exits."
    log_important "Please investigate an alternative way to keep Teleport running."
    log_important "You can find information in our documentation: ${TELEPORT_DOCS_URL}"
    log_important "For now, Teleport will be started in the foregound - you can press Ctrl+C to exit."
    log_only
    log_only "------------------------------------------------------------------------"
    log_only "| IMPORTANT: TELEPORT WILL STOP RUNNING AFTER YOU CLOSE THIS TERMINAL! |"
    log_only "|   YOU MUST CONFIGURE A SERVICE MANAGER TO MAKE IT RUN ON STARTUP!    |"
    log_only "------------------------------------------------------------------------"
    log_only
}
# check connectivity to the given host/port and make a request to see if Teleport is listening
check_connectivity() {
    HOST=$1
    PORT=$2
    # check with nc
    if check_exists nc; then
        log "Checking connectivity to ${HOST}:${PORT} with nc"
        if nc -z -w3 ${HOST} ${PORT}; then
            log "Connectivity test succeeded with nc"
            return 0
        else
            log "Connectivity test failed with nc"
            return 1
        fi
    # if there's no nc, check with telnet
    elif check_exists telnet; then
        log "Checking connectivity to ${HOST}:${PORT} with telnet"
        if telnet -c ${HOST} ${PORT} </dev/null >/dev/null 2>&1 | grep -q Connected; then
            log "Connectivity test succeeded with telnet"
            return 0
        else
            log "Connectivity test failed with telnet"
            return 1
        fi
    # if there's no nc or telnet, try and use /dev/tcp
    elif [ -f /dev/tcp ]; then
        log "Checking connectivity to ${HOST}:${PORT} with /dev/tcp"
        if (head -1 < /dev/tcp/${HOST}/${PORT}) >/dev/null 2>&1; then
            log "Connectivity test succeeded with /dev/tcp"
            return 0
        else
            log "Connectivity test failed with /dev/tcp"
            return 1
        fi
    else
        return -1
    fi
}

# main script starts here
# check connectivity to teleport server/port
log "Checking TCP connectivity to Teleport server"
RETURN_CODE=$(check_connectivity ${TARGET_HOSTNAME} ${TARGET_PORT}) || true
#check_connectivity ${TARGET_HOSTNAME} ${TARGET_PORT}
if [[ ${RETURN_CODE} > 0 ]]; then
    log_important "Couldn't open a connection to the Teleport server - ${TARGET_HOST}:${TARGET_PORT}"
    log_important "This issue will need to be fixed before the script can continue."
    exit 1
elif [[ ${RETURN_CODE} < 0 ]]; then
    log "Couldn't find nc, telnet or /dev/tcp to do a connection test"
    log "Going to blindly continue without testing connectivity"
fi

# use OSTYPE variable to figure out host type/arch
if [[ "${OSTYPE}" == "linux-gnu"* ]]; then
    # linux host, now detect arch
    TELEPORT_BINARY_TYPE="linux"
    ARCH=$(uname -m)
    log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
    if [[ ${ARCH} == "armv7l" ]]; then
        TELEPORT_ARCH="arm"
    elif [[ ${ARCH} == "x86_64" ]]; then
        TELEPORT_ARCH="amd64"
    elif [[ ${ARCH} == "i386" ]]; then
        TELEPORT_ARCH="386"
    else
        log_important "Error: cannot detect architecture from uname -m: ${ARCH}"
        exit 1
    fi
    log "Detected arch: ${ARCH}, using Teleport arch ${TELEPORT_ARCH}"
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
        if [[ ${DISTRO_TYPE} == "debian" ]]; then
            TELEPORT_FORMAT="deb"
        elif [[ ${DISTRO_TYPE} == "centos"* ]] || [[ ${DISTRO_TYPE} == "rhel"* ]] || [[ ${DISTRO_TYPE} == "fedora"* ]]; then
            TELEPORT_FORMAT="rpm"
        fi
    fi
    log "Detected distro type: ${DISTRO_TYPE}, using Teleport ${TELEPORT_FORMAT} distribution"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
    # macos host, now detect arch
    TELEPORT_BINARY_TYPE="darwin"
    ARCH=$(uname -m)
    log "Detected host: ${OSTYPE}, using Teleport binary type ${TELEPORT_BINARY_TYPE}"
    if [[ ${ARCH} == "aarch64" ]]; then
        TELEPORT_ARCH="aarch64"
        log_important "Error: detected ${ARCH} but Teleport doesn't support this architecture yet, exiting"
    elif [[ ${ARCH} == "x86_64" ]]; then
        TELEPORT_ARCH="amd64"
    else
        log_important "Error: unsupported architecture from uname -m: ${ARCH}"
        exit 1
    fi
    log "Detected MacOS ${ARCH} architecture, using Teleport arch ${TELEPORT_ARCH}"
    TELEPORT_FORMAT="tarball"
    log "Using Teleport ${TELEPORT_FORMAT} distribution"
else
    log_important "Error - unsupported platform: ${OSTYPE}"
    exit 1
fi

# create temporary directory and exit cleanup logic
TEMP_DIR=$(mktemp -d -t teleport-XXXXXXXXXX)
log "Created temp dir ${TEMP_DIR}"
pushd ${TEMP_DIR} >/dev/null 2>&1

finish() {
    popd >/dev/null 2>&1
    log "Cleaning up temp dir ${TEMP_DIR} and exiting"
    rm -rf ${TEMP_DIR}
    exit 0
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
    log_header "Warning: Teleport appears to already be running on this host."
    log_cleanup_message
    exit 1
fi

# check for existing config file
if teleport_config_exists; then
    log_header "Warning: There is already a Teleport config file present at ${TELEPORT_CONFIG_PATH}."
    log_cleanup_message
    exit 1
fi

# check for existing data directory
if teleport_datadir_exists; then
    log_header "Warning: Found existing Teleport data under ${TELEPORT_DATA_DIR}."
    log_cleanup_message
    exit 1
fi

# check for existing binaries
if teleport_binaries_exist; then
    log_header "Warning: Found existing Teleport binaries under ${TELEPORT_BINARY_DIR}."
    log_cleanup_message
    exit 1
fi

# handle centos6 installations
if [[ ${TELEPORT_FORMAT} == "rpm-centos6" ]]; then
# ov    erride the format to 'tarball' (as that's how centos6 binaries are packaged)
    log "Overriding format for centos6 installation to use tarball"
    TELEPORT_FORMAT="tarball"
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
    log "Downloading Teleport release from ${URL} to ${TEMP_DIR}/teleport.tar.gz"
    download ${URL} ${TEMP_DIR}/teleport.tar.gz
    # extract tarball
    tar -xzf ${TEMP_DIR}/teleport.tar.gz -C ${TEMP_DIR}
    # install binaries to /usr/local/bin
    for BINARY in teleport tctl tsh; do
        cp teleport/${BINARY} ${TELEPORT_BINARY_DIR}/
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
    log "Downloading Teleport release from ${URL} to ${TEMP_DIR}/teleport.deb"
    download ${URL} ${TEMP_DIR}/teleport.deb
    # install deb
    dpkg -i ${TEMP_DIR}/teleport.deb
elif [[ ${TELEPORT_FORMAT} == "rpm" ]]; then
    # convert teleport arch to rpm arch
    if [[ ${TELEPORT_ARCH} == "amd64" ]]; then
        RPM_ARCH="x86_64"
    elif [[ ${TELEPORT_ARCH} == "386" ]]; then
        RPM_ARCH="i386"
    fi
    # check for package managers
    if check_exists dnf; then
        PACKAGE_MANAGER_COMMAND="dnf -y install"
    elif check_exists yum; then
        PACKAGE_MANAGER_COMMAND="yum -y localinstall"
    else
        PACKAGE_MANAGER_COMMAND=""
        log "Cannot find 'yum' or 'dnf' package manager commands"
        log "Will try downloading and installing the rpm manually instead"
    fi
    # install with package manager if available
    if [[ ${PACKAGE_MANAGER_COMMAND} != "" ]]; then
        log "Installing Teleport release from ${URL} using ${PACKAGE_MANAGER_COMMAND}"
        # install rpm (yum/dnf will download it themselves)
        ${PACKAGE_MANAGER_COMMAND} ${URL}
    # use curl and rpm if we can't find a package manager
    else
        # check that needed tools are installed
        check_exists_fatal rpm
        # download tarball
        log "Downloading Teleport release from ${URL} to ${TEMP_DIR}/teleport.rpm"
        download ${URL} ${TEMP_DIR}/teleport.rpm
        # install RPM (in upgrade mode)
        rpm -Uvh ${TEMP_DIR}/teleport.rpm
    fi
else
    log_important "Can't figure out what Teleport format to use"
    exit 1
fi

# install teleport config
install_teleport_config

# install systemd unit if applicable (linux hosts)
if is_using_systemd; then
    install_systemd_unit
    start_teleport_systemd
# install launchd config on MacOS hosts
elif is_macos_host; then
    install_launchd_config
    start_teleport_launchd
# not a MacOS host and no systemd available, print a warning
# and temporarily start Teleport in the foreground
else
    no_systemd_warning
    start_teleport_foreground
fi
