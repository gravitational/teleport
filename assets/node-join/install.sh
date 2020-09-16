#!/bin/bash
set -euo pipefail
SCRIPT_NAME="teleport-node-installer"

# default values
INSTALL_DIRECTORY="/usr/local/bin"
LOG_FILENAME="${TMPDIR:-/tmp}/${SCRIPT_NAME}.log"
SYSTEMD_UNIT_PATH="/etc/systemd/system/teleport.service"
TARGET_PORT_DEFAULT=3025
TELEPORT_CONFIG_PATH="/etc/teleport.yaml"

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
[ -n "${p}" ] && TARGET_PORT=${p} || { echo -n "Enter target port to connect node to [default: 3025]: "; read TARGET_PORT; }
[ -n "${j}" ] && NODE_JOIN_TOKEN=${j} || { echo -n "Enter Teleport node join token as provided: "; read NODE_JOIN_TOKEN; }
[ -n "${c}" ] && CA_PIN_HASH=${c} || { echo -n "Enter CA pin hash: "; read CA_PIN_HASH; }
[ -n "${f}" ] && OVERRIDE_FORMAT=${f}
[ -n "${l}" ] && LOG_FILENAME=${l}

# set default target port if value not provided
if [[ "${TARGET_PORT}" == "" ]]; then
    TARGET_PORT=${TARGET_PORT_DEFAULT}
fi

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
# important log lines, print even when -q (quiet) is passed
log_important() {
    LOG_LINE="$(log_date) [${SCRIPT_NAME}] ---> $*"
    echo "${LOG_LINE}"
    if [[ "${LOG_FILENAME}" != "" ]]; then
        echo "${LOG_LINE}" >> ${LOG_FILENAME}
    fi
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
# installs the teleport-provided systemd unit
install_systemd_unit() {
    log "Installing Teleport systemd unit to ${SYSTEMD_UNIT_PATH}"
    cp ./teleport/examples/systemd/teleport.service ${SYSTEMD_UNIT_PATH}
    log "Reloading unit files (systemctl daemon-reload)"
    systemctl daemon-reload
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
# prints a warning if the host isn't running systemd
no_systemd_warning() {
    log_important "This host is not running systemd, so Teleport cannot be started automatically when it exits."
    log_important "Please investigate an alternative way to keep Teleport running."
    log_important "For now, Teleport will be started in the foregound - you can press Ctrl+C to exit."
    log_important "------------------------------------------------------------------------"
    log_important "| IMPORTANT: TELEPORT WILL STOP RUNNING AFTER YOU CLOSE THIS TERMINAL! |"
    log_important "------------------------------------------------------------------------"
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
    # TODO(gus): remove
    # echo "Doing nothing"
}
trap finish EXIT

# optional format override (mostly for testing)
if [[ ${OVERRIDE_FORMAT} != "" ]]; then
    TELEPORT_FORMAT="${OVERRIDE_FORMAT}"
    log "Overriding TELEPORT_FORMAT to ${OVERRIDE_FORMAT}"
fi

# select correct URL/installation method based on distro
if [[ ${TELEPORT_FORMAT} == "tarball" ]]; then
    URL="https://get.gravitational.com/teleport-v${TELEPORT_VERSION}-${TELEPORT_BINARY_TYPE}-${TELEPORT_ARCH}-bin.tar.gz"
    # check that needed tools are installed
    check_exists_fatal tar
    # download tarball
    log "Downloading Teleport release from ${URL} to ${TEMP_DIR}/teleport.tar.gz"
    download ${URL} ${TEMP_DIR}/teleport.tar.gz
    # extract tarball
    tar -xzf ${TEMP_DIR}/teleport.tar.gz -C ${TEMP_DIR}
    # install binaries to /usr/local/bin
    for BINARY in teleport tctl tsh; do
        cp -f teleport/${BINARY} ${INSTALL_DIRECTORY}/
    done
    # install config file
    install_teleport_config
    # install systemd unit if applicable
    if is_using_systemd; then
        install_systemd_unit
        start_teleport_systemd
    else
        no_systemd_warning
        start_teleport_foreground
    fi
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
    # install teleport config
    install_teleport_config
    # start systemd unit (if applicable)
    if is_using_systemd; then
        start_teleport_systemd
    else
        no_systemd_warning
        start_teleport_foreground
    fi
elif [[ ${TELEPORT_FORMAT} == "rpm"* ]]; then
    # convert teleport arch to rpm arch
    if [[ ${TELEPORT_ARCH} == "amd64" ]]; then
        RPM_ARCH="x86_64"
    elif [[ ${TELEPORT_ARCH} == "386" ]]; then
        RPM_ARCH="i386"
    fi
    URL="https://get.gravitational.com/teleport-${TELEPORT_VERSION}-1.${RPM_ARCH}.rpm"
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
    # install teleport config
    install_teleport_config
    # start systemd unit (if applicable)
    if is_using_systemd; then
        start_teleport_systemd
    else
        no_systemd_warning
        start_teleport_foreground
    fi
else
    log_important "Can't figure out what Teleport format to use"
    exit 1
fi
