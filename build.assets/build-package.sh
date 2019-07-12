#!/bin/bash
set -e

usage() { echo "Usage: $(basename $0) [-t <oss/ent>] [-v <version>] [-p <package type>] <-a [amd64/x86_64]|[386/i386]> <-r go1.9.7|fips> <-s tarball source dir>" 1>&2; exit 1; }
while getopts ":t:v:p:a:r:s:" o; do
    case "${o}" in
        t)
            t=${OPTARG}
            if [[ ${t} != "oss" && ${t} != "ent" ]]; then usage; fi
            ;;
        v)
            v=${OPTARG}
            ;;
        p)
            p=${OPTARG}
            if [[ ${p} != "rpm" && ${p} != "deb" && ${p} != "pkg" ]]; then usage; fi
            ;;
        a)
            a=${OPTARG}
            if [[ ${a} != "amd64" && ${a} != "x86_64" && ${a} != "386" && ${a} != "i386" ]]; then usage; fi
            ;;
        r)
            r=${OPTARG}
            if [[ ${r} != "go1.9.7" && ${r} != "fips" ]]; then usage; fi
            ;;
        s)
            s=${OPTARG}
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

if [ -z "${t}" ] || [ -z "${v}" ] || [ -z "${p}" ]; then
    usage
fi

TELEPORT_TYPE=${t}
TELEPORT_VERSION=${v}
PACKAGE_TYPE=${p}
ARCH=${a}
RUNTIME=${r}
TARBALL_DIRECTORY=/tmp/teleport-tarballs
DOWNLOAD_IF_NEEDED=true
if [[ "${s}" != "" ]]; then
    DOWNLOAD_IF_NEEDED=false
    TARBALL_DIRECTORY=${s}
fi

# linux package configuration
LINUX_BINARY_DIR=/usr/local/bin
LINUX_SYSTEMD_DIR=/lib/systemd/system
LINUX_CONFIG_DIR=/etc
LINUX_DATA_DIR=/var/lib/teleport

# extra package information for linux
MAINTAINER="ops@gravitational.com"
LICENSE="Apache License 2.0"
VENDOR="Gravitational"
DESCRIPTION="Gravitational Teleport is a gateway for managing access to clusters of Linux servers via SSH or the Kubernetes API"

DOWNLOAD_ROOT="https://get.gravitational.com"

# check that curl is installed
if [ ! $(command -v curl) ]; then
    echo "curl must be installed"
    exit 2
fi

# check that docker is installed when fpm is needed to build
if [[ "${PACKAGE_TYPE}" != "pkg" ]]; then
    if [ ! $(command -v docker) ]; then
        echo "docker must be installed to build non-OSX packages"
        exit 3
    fi
fi

# check that pkgbuild is installed if building for OS X and set variables appropriately
if [[ "${PACKAGE_TYPE}" == "pkg" ]]; then
    if ! uname | grep -q Darwin; then
        echo "You must be running on OS X to build .pkg files"
        exit 4
    fi
    if [[ "${ARCH}" != "" ]]; then
        echo "arch parameter is ignored when building for OS X"
        unset ARCH
    fi
    if [[ "${RUNTIME}" != "" ]]; then
        echo "runtime parameter is ignored when building for OS X"
        unset RUNTIME
    fi
    PLATFORM="darwin"
    ARCH="amd64"
    if [ ! $(command -v pkgbuild) ]; then
        echo "You need to install pkgbuild"
        echo "Run: xcode-select --install"
        exit 5
    fi
else
    PLATFORM="linux"
    # if arch isn't set for other package types, throw an error
    if [[ "${ARCH}" == "" ]]; then
        usage
    fi
    # set docker image appropriately
    if [[ "${PACKAGE_TYPE}" == "deb" ]]; then
        DOCKER_IMAGE="cdrx/fpm-debian:8"
    elif [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        DOCKER_IMAGE="cdrx/fpm-centos:7"
    fi
fi

# handle differences between 'gravitational' arch and system arch
FILENAME_ARCH=${ARCH}
if [[ "${ARCH}" == "i386" ]]; then
    FILENAME_ARCH="386"
elif [[ "${ARCH}" == "386" ]]; then
    ARCH="i386"
elif [[ "${ARCH}" == "x86_64" ]]; then
    FILENAME_ARCH="amd64"
    if [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        ARCH="x86_64"
    fi
elif [[ "${ARCH}" == "amd64" ]]; then
    if [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        ARCH="x86_64"
    fi
fi

# set optional runtime section for filename
if [[ "${RUNTIME}" == "go1.9.7" ]]; then
    OPTIONAL_RUNTIME_SECTION="-go1.9.7"
elif [[ "${RUNTIME}" == "fips" ]]; then
    OPTIONAL_RUNTIME_SECTION="-fips"
fi

# set variables appropriately depending on type of package being built
if [[ "${TELEPORT_TYPE}" == "ent" ]]; then
    TARBALL_FILENAME="teleport-ent-v${TELEPORT_VERSION}-${PLATFORM}-${FILENAME_ARCH}${OPTIONAL_RUNTIME_SECTION}-bin.tar.gz"
    URL="${DOWNLOAD_ROOT}/${TARBALL_FILENAME}"
    PKG_FILENAME="teleport-ent-v${TELEPORT_VERSION}.${PACKAGE_TYPE}"
    TAR_PATH="teleport-ent"
    if [[ "${RUNTIME}" == "go1.9.7" ]]; then
        TYPE_DESCRIPTION="[Enterprise edition, built with Go 1.9.7]"
    elif [[ "${RUNTIME}" == "fips" ]]; then
        TYPE_DESCRIPTION="[Enterprise edition, built with FIPS support]"
    else
        TYPE_DESCRIPTION="[Enterprise edition]"
    fi
else
    TARBALL_FILENAME="teleport-v${TELEPORT_VERSION}-${PLATFORM}-${FILENAME_ARCH}${OPTIONAL_RUNTIME_SECTION}-bin.tar.gz"
    URL="${DOWNLOAD_ROOT}/${TARBALL_FILENAME}"
    PKG_FILENAME="teleport-${TELEPORT_VERSION}.${PACKAGE_TYPE}"
    TAR_PATH="teleport"
    if [[ "${RUNTIME}" == "go1.9.7" ]]; then
        TYPE_DESCRIPTION="[Open source edition, built with Go 1.9.7]"
    elif [[ "${RUNTIME}" == "fips" ]]; then
        TYPE_DESCRIPTION="[Open source edition, built with FIPS support]"
    else
        TYPE_DESCRIPTION="[Open source edition]"
    fi
fi

# set file list
if [[ "${PACKAGE_TYPE}" == "pkg" ]]; then
    FILE_LIST="${TAR_PATH}/tsh ${TAR_PATH}/tctl ${TAR_PATH}/teleport"
else
    FILE_LIST="${TAR_PATH}/tsh ${TAR_PATH}/tctl ${TAR_PATH}/teleport ${TAR_PATH}/examples/systemd/teleport.service"
    LINUX_BINARY_FILE_LIST="${TAR_PATH}/tsh ${TAR_PATH}/tctl ${TAR_PATH}/teleport"
    LINUX_SYSTEMD_FILE_LIST="${TAR_PATH}/examples/systemd/teleport.service"
    LINUX_CONFIG_FILE_LIST=""
    #OUTPUT_NAME=${TAR_PATH}
    if [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        OUTPUT_FILENAME="${TAR_PATH}-${TELEPORT_VERSION}-1${OPTIONAL_RUNTIME_SECTION}.${ARCH}.rpm"
    elif [[ "${PACKAGE_TYPE}" == "deb" ]]; then
        OUTPUT_FILENAME="${TAR_PATH}_${TELEPORT_VERSION}${OPTIONAL_RUNTIME_SECTION}_${ARCH}.deb"
    fi
fi

# create a temporary directory and download specified Teleport version
pushd $(mktemp -d)
TMPDIR=$(pwd)
mkdir -p ${TMPDIR}/buildroot

# implement a rudimentary download cache for repeat builds on the same host
mkdir -p ${TARBALL_DIRECTORY}
if [ ! -f ${TARBALL_DIRECTORY}/${TARBALL_FILENAME} ]; then
    if [[ "${DOWNLOAD_IF_NEEDED}" == "true" ]]; then
        echo "Downloading ${URL} to ${TARBALL_DIRECTORY}"
        curl -sL ${URL} -o ${TARBALL_DIRECTORY}/${TARBALL_FILENAME}
    else
        echo "Can't find ${TARBALL_DIRECTORY}/${TARBALL_FILENAME}"
        echo "Downloading from ${DOWNLOAD_ROOT} is disabled when a path is provided with -s"
        exit 6
    fi
else
    echo "Found ${TARBALL_DIRECTORY}/${TARBALL_FILENAME} - using it"
fi

# extract necessary files from tarball
tar -C $(pwd) -xvzf ${TARBALL_DIRECTORY}/${TARBALL_FILENAME} ${FILE_LIST}

# move files into correct locations before building the package
if [[ "${PACKAGE_TYPE}" != "pkg" ]]; then
    if [[ "${LINUX_BINARY_FILE_LIST}" != "" ]]; then
        mkdir -p ${TMPDIR}/buildroot${LINUX_BINARY_DIR}
        mv -v ${LINUX_BINARY_FILE_LIST} ${TMPDIR}/buildroot${LINUX_BINARY_DIR}
    fi
    if [[ "${LINUX_SYSTEMD_FILE_LIST}" != "" ]]; then
        mkdir -p ${TMPDIR}/buildroot${LINUX_SYSTEMD_DIR}
        mv -v ${LINUX_SYSTEMD_FILE_LIST} ${TMPDIR}/buildroot${LINUX_SYSTEMD_DIR}
    fi
    if [[ "${LINUX_CONFIG_FILE}" != "" ]]; then
        mkdir -p ${TMPDIR}/buildroot${LINUX_CONFIG_DIR}
        mv -v ${LINUX_CONFIG_FILE} ${TMPDIR}/buildroot${LINUX_CONFIG_DIR}
        CONFIG_FILE_STANZA="--config-files /src/buildroot${LINUX_CONFIG_DIR}/${LINUX_CONFIG_FILE} "
    fi
    # /var/lib/teleport
    mkdir -p ${TMPDIR}/buildroot${LINUX_DATA_DIR}
fi
popd

if [[ "${PACKAGE_TYPE}" == "pkg" ]]; then
    # erase any existing versions of the package in the output directory first
    rm -f ${PKG_FILENAME}

    # build the package for OS X
    pkgbuild \
        --root ${TMPDIR}/${TAR_PATH} \
        --identifier com.gravitational.teleport \
        --version ${TELEPORT_VERSION} \
        --install-location /usr/local/bin \
        ${PKG_FILENAME}

    # checksum created packages
    for FILE in *.${PACKAGE_TYPE}; do
        shasum -a 256 ${FILE} > ${FILE}.sha256
    done
else
    # erase any existing packages of the same type/version/arch in the output directory first
    rm -vf ${OUTPUT_FILENAME}

    # build for other platforms
    docker run -v ${TMPDIR}:/src -it --rm ${DOCKER_IMAGE} \
        fpm \
        --input-type dir \
        --output-type ${PACKAGE_TYPE} \
        --name ${TAR_PATH} \
        --version "${TELEPORT_VERSION}" \
        --maintainer "${MAINTAINER}" \
        --url "${URL}" \
        --license "${LICENSE}" \
        --vendor "${VENDOR}" \
        --description "${DESCRIPTION} ${TYPE_DESCRIPTION}" \
        --architecture ${ARCH} \
        --package ${OUTPUT_FILENAME} \
        --chdir /src/buildroot \
        --directories ${LINUX_DATA_DIR} \
        --provides teleport \
        --prefix / \
        --verbose \
        ${CONFIG_FILE_STANZA} .

    # copy created package back to current directory
    cp ${TMPDIR}/*.${PACKAGE_TYPE} .

    # checksum created packages
    for FILE in *.${PACKAGE_TYPE}; do
        sha256sum ${FILE} > ${FILE}.sha256
    done
fi

# clean up temporary directory
rm -rf ${TMPDIR}


