#!/bin/bash
set -e

usage() {
  echo "Usage: $(basename $0) [-t <oss/ent>] [-v <version>] [-p <package type>] [-b <bundle id>] <-a [amd64/x86_64]|[386/i386]|arm|arm64> <-r fips> <-s tarball source dir>" 1>&2
  exit 1
}

# Don't follow sourced script.
#shellcheck disable=SC1090
#shellcheck disable=SC1091
. "$(dirname "$0")/build-common.sh"

while getopts ":t:v:p:a:r:s:b:n" o; do
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
            if [[ ${a} != "amd64" && ${a} != "x86_64" && ${a} != "386" && ${a} != "i386" && ${a} != "arm" && ${a} != "arm64" && ${a} != "universal" ]]; then usage; fi
            ;;
        r)
            r=${OPTARG}
            if [[ ${r} != "fips" ]]; then usage; fi
            ;;
        s)
            s=${OPTARG}
            ;;
	b)
	    b=${OPTARG}
	    ;;
        n)
            # Dry-run mode.
            # Only affects parts of the script, use at your own peril!
            DRY_RUN_PREFIX='echo + '
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

TELEPORT_TYPE="$t"
TELEPORT_VERSION="$v"
PACKAGE_TYPE="$p"
ARCH="$a"
RUNTIME="$r"
TARBALL_DIRECTORY="$s"
GNUPG_DIR=${GNUPG_DIR:-/tmp/gnupg}

# linux package configuration
LINUX_BINARY_DIR=/opt/teleport/system/bin
LINUX_SYSTEMD_DIR=/opt/teleport/system/lib/systemd/system
LINUX_CONFIG_DIR=/etc
LINUX_DATA_DIR=/var/lib/teleport

# Image containing fpm for building linux packages. Built from gravitational/docker-fpm repo.
FPM_IMAGE_DEB="public.ecr.aws/gravitational/fpm:debian12-1.15.1-1"
FPM_IMAGE_RPM="public.ecr.aws/gravitational/fpm:centos8-1.15.1-1"

# extra package information for linux
MAINTAINER="info@goteleport.com"
LICENSE="Teleport Community Edition License"
VENDOR="Gravitational"
DESCRIPTION="Teleport provides on-demand, least-privileged access to your infrastructure, on a foundation of cryptographic identity and zero trust, with built-in identity and policy governance"
DOCS_URL="https://goteleport.com/docs"

# check that curl is installed
if [[ ! $(type curl) ]]; then
    echo "curl must be installed"
    exit 2
fi

# check that tar is installed
if [[ ! $(type tar) ]]; then
    echo "tar must be installed"
    exit 11
fi

# check that docker is installed when fpm is needed to build
if [[ "${PACKAGE_TYPE}" != "pkg" ]]; then
    if [[ ! $(type docker) ]]; then
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
    if [[ "${RUNTIME}" != "" ]]; then
        echo "runtime parameter is ignored when building for OS X"
        unset RUNTIME
    fi
    PLATFORM="darwin"
    if [[ ! $(type pkgbuild) ]]; then
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

    if [[ -n "${b:-}" ]]; then
        echo "bundle ID parameter can only be used for OS X packages"
        exit 6
    fi

    # set docker image appropriately
    if [[ "${PACKAGE_TYPE}" == "deb" ]]; then
        FPM_IMAGE="${FPM_IMAGE_DEB}"
    elif [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        FPM_IMAGE="${FPM_IMAGE_RPM}"
    fi
fi

PACKAGE_ARCH=""
# handle differences between 'gravitational' arch and system arch
if [[ "${ARCH}" == "386" || "${ARCH}" == "i386" ]]; then
    TEXT_ARCH="32-bit x86"
    TARBALL_ARCH="386"
    DEB_PACKAGE_ARCH="i386"
    DEB_OUTPUT_ARCH="i386"
    RPM_PACKAGE_ARCH="i386"
    RPM_OUTPUT_ARCH="i386"
elif [[ "${ARCH}" == "amd64" || "${ARCH}" == "x86_64" ]]; then
    TEXT_ARCH="64-bit x86"
    TARBALL_ARCH="amd64"
    PACKAGE_ARCH="amd64"
    DEB_PACKAGE_ARCH="amd64"
    DEB_OUTPUT_ARCH="amd64"
    RPM_PACKAGE_ARCH="x86_64"
    RPM_OUTPUT_ARCH="x86_64"
elif [[ "${ARCH}" == "arm" ]]; then
    TEXT_ARCH="32-bit ARM"
    TARBALL_ARCH="arm"
    # 32-bit arm can be hardfloat and softfloat, and we build for linux-gnueabihf
    DEB_PACKAGE_ARCH="armhf"
    DEB_OUTPUT_ARCH="arm" # backwards compatibility
    RPM_PACKAGE_ARCH="armv7hl"
    RPM_OUTPUT_ARCH="arm" # backwards compatibility
elif [[ "${ARCH}" == "arm64" ]]; then
    TEXT_ARCH="64-bit ARM"
    TARBALL_ARCH="arm64"
    PACKAGE_ARCH="arm64"
    DEB_PACKAGE_ARCH="arm64"
    DEB_OUTPUT_ARCH="arm64"
    RPM_PACKAGE_ARCH="aarch64"
    RPM_OUTPUT_ARCH="arm64" # backwards compatibility
elif [[ "${ARCH}" == "universal" ]]; then
    TARBALL_ARCH="universal"
    PACKAGE_ARCH="universal"
fi

# amd64 RPMs should use CentOS 7 compatible artifacts
if [[ "${PACKAGE_TYPE}" == "rpm" && "${RPM_PACKAGE_ARCH}" == "x86_64" ]]; then
    OPTIONAL_TARBALL_SECTION+="-centos7"
fi

# set optional runtime section for filename
if [[ "${RUNTIME}" == "fips" ]]; then
    OPTIONAL_RUNTIME_SECTION+="-fips"
fi

# After install is --after-install except for RPM, we use --rpm-posttrans.
# This is because RPM runs after install scrips before the old package removal,
# so old Teleport are still here and we cannot run our teleport-update symlink logic.
AFTER_INSTALL_TARGET="--after-install"

# set variables appropriately depending on type of package being built
if [[ "${TELEPORT_TYPE}" == "ent" ]]; then
    TARBALL_FILENAME="teleport-ent-v${TELEPORT_VERSION}-${PLATFORM}-${TARBALL_ARCH}${OPTIONAL_TARBALL_SECTION}${OPTIONAL_RUNTIME_SECTION}-bin.tar.gz"
    TAR_PATH="teleport-ent"
    RPM_NAME="teleport-ent"
    if [[ "${RUNTIME}" == "fips" ]]; then
        TYPE_DESCRIPTION="[${TEXT_ARCH} Enterprise edition, built with FIPS support]"
        RPM_NAME="teleport-ent-fips"
    else
        TYPE_DESCRIPTION="[${TEXT_ARCH} Enterprise edition]"
    fi
    LICENSE_STANZA=()
else
    TARBALL_FILENAME="teleport-v${TELEPORT_VERSION}-${PLATFORM}-${TARBALL_ARCH}${OPTIONAL_TARBALL_SECTION}${OPTIONAL_RUNTIME_SECTION}-bin.tar.gz"
    TAR_PATH="teleport"
    RPM_NAME="teleport"
    if [[ "${RUNTIME}" == "fips" ]]; then
        TYPE_DESCRIPTION="[${TEXT_ARCH} Open source edition, built with FIPS support]"
        RPM_NAME="teleport-fips"
    else
        TYPE_DESCRIPTION="[${TEXT_ARCH} Open source edition]"
    fi
    TYPE_DESCRIPTION="${TYPE_DESCRIPTION} Distributed under the ${LICENSE}"
    LICENSE_STANZA=(--license "${LICENSE}")
fi

# set file list
if [[ "${PACKAGE_TYPE}" == "pkg" ]]; then
    if [[ -z "${PACKAGE_ARCH}" ]]; then
        echo "Unsupported architecture: ${ARCH}"
        exit 1
    fi
    # No architecture tag on package filename for universal (multi-arch) binaries.
    ARCH_TAG=""
    if [[ "${PACKAGE_ARCH}" != "universal" ]]; then
        ARCH_TAG="-${PACKAGE_ARCH}"
    fi
    SIGN_PKG="true"
    FILE_LIST="${TAR_PATH}/teleport ${TAR_PATH}/tbot ${TAR_PATH}/fdpass-teleport"
    BUNDLE_ID="${b:-com.gravitational.teleport}"
    if [[ "${TELEPORT_TYPE}" == "ent" ]]; then
        PKG_FILENAME="teleport-ent-bin-${TELEPORT_VERSION}${ARCH_TAG}.${PACKAGE_TYPE}"
    else
        PKG_FILENAME="teleport-bin-${TELEPORT_VERSION}${ARCH_TAG}.${PACKAGE_TYPE}"
    fi
else
    FILE_LIST="${TAR_PATH}/tsh ${TAR_PATH}/tctl ${TAR_PATH}/teleport ${TAR_PATH}/tbot ${TAR_PATH}/fdpass-teleport ${TAR_PATH}/teleport-update ${TAR_PATH}/examples/systemd/teleport.service ${TAR_PATH}/examples/systemd/post-install ${TAR_PATH}/examples/systemd/before-remove"
    LINUX_BINARY_FILE_LIST="${TAR_PATH}/tsh ${TAR_PATH}/tctl ${TAR_PATH}/tbot ${TAR_PATH}/fdpass-teleport ${TAR_PATH}/teleport ${TAR_PATH}/teleport-update"
    LINUX_SYSTEMD_FILE_LIST="${TAR_PATH}/examples/systemd/teleport.service"
    EXTRA_DOCKER_OPTIONS=""
    RPM_SIGN_STANZA=""
    if [[ "${PACKAGE_TYPE}" == "rpm" ]]; then
        PACKAGE_ARCH="${RPM_PACKAGE_ARCH}"
        OUTPUT_FILENAME="${TAR_PATH}-${TELEPORT_VERSION}-1${OPTIONAL_RUNTIME_SECTION}.${RPM_OUTPUT_ARCH}.rpm"
        FILE_PERMISSIONS_STANZA="--rpm-user root --rpm-group root --rpm-use-file-permissions "
        # the rpm/rpmmacros file suppresses the creation of .build-id files (see https://github.com/gravitational/teleport/issues/7040)
        EXTRA_DOCKER_OPTIONS="-v $(pwd)/rpm/rpmmacros:/root/.rpmmacros"

        AFTER_INSTALL_TARGET="--rpm-posttrans"
        # if we set this environment variable, don't sign RPMs (can be useful for building test RPMs
        # without having the signing keys)
        if [ "${UNSIGNED_RPM}" == "true" ]; then
            echo "RPMs will not be signed as requested"
        else
            # the GNUPG_DIR location here is assumed to contain a complete ~/.gnupg directory structure
            # with pubring.kbx and trustdb.gpg files, plus a private-keys-v1.d directory with signing keys
            # it needs to contain the "Gravitational, Inc" private key and signing key.
            # we also use the rpm-sign/rpmmacros file instead which contains extra directives used for signing.
            EXTRA_DOCKER_OPTIONS="-v $(pwd)/rpm-sign/rpmmacros:/root/.rpmmacros -v $(pwd)/rpm-sign/popt-override:/etc/popt.d/rpmsign-override -v ${GNUPG_DIR}:/root/.gnupg"
            RPM_SIGN_STANZA="--rpm-sign --rpm-digest sha256"
        fi
    elif [[ "${PACKAGE_TYPE}" == "deb" ]]; then
        PACKAGE_ARCH="${DEB_PACKAGE_ARCH}"
        OUTPUT_FILENAME="${TAR_PATH}_${TELEPORT_VERSION}${OPTIONAL_RUNTIME_SECTION}_${DEB_OUTPUT_ARCH}.deb"
        FILE_PERMISSIONS_STANZA="--deb-user root --deb-group root "
    fi
fi

# create a temporary directory and download specified Teleport version
pushd "$(mktemp -d)"
PACKAGE_TEMPDIR=$(pwd)
# automatically clean up on exit
trap 'rm -rf ${PACKAGE_TEMPDIR}' EXIT
mkdir -p ${PACKAGE_TEMPDIR}/buildroot

# Find or download tarball to the local file cache.
tarname="$TARBALL_FILENAME"
[[ -n "$TARBALL_DIRECTORY" ]] && tarname="$TARBALL_DIRECTORY/$TARBALL_FILENAME"
tarout='' # find_or_fetch_tarball writes to this
find_or_fetch_tarball "$tarname" tarout
TARBALL_DIRECTORY="$(dirname "$tarout")"
TARBALL_FILENAME="$(basename "$tarout")" # for consistency, shouldn't change
echo "Found ${TARBALL_DIRECTORY}/${TARBALL_FILENAME} - using it"

# extract necessary files from tarball
tar -C "$(pwd)" -xvzf ${TARBALL_DIRECTORY}/${TARBALL_FILENAME} ${FILE_LIST}

# move files into correct locations before building the package
if [[ "${PACKAGE_TYPE}" != "pkg" ]]; then
    if [[ "${LINUX_BINARY_FILE_LIST}" != "" ]]; then
        mkdir -p ${PACKAGE_TEMPDIR}/buildroot${LINUX_BINARY_DIR}
        mv -v ${LINUX_BINARY_FILE_LIST} ${PACKAGE_TEMPDIR}/buildroot${LINUX_BINARY_DIR}
    fi
    if [[ "${LINUX_SYSTEMD_FILE_LIST}" != "" ]]; then
        mkdir -p ${PACKAGE_TEMPDIR}/buildroot${LINUX_SYSTEMD_DIR}
        mv -v ${LINUX_SYSTEMD_FILE_LIST} ${PACKAGE_TEMPDIR}/buildroot${LINUX_SYSTEMD_DIR}
    fi
    if [[ "${LINUX_CONFIG_FILE}" != "" ]]; then
        mkdir -p ${PACKAGE_TEMPDIR}/buildroot${LINUX_CONFIG_DIR}
        mv -v ${LINUX_CONFIG_FILE} ${PACKAGE_TEMPDIR}/buildroot${LINUX_CONFIG_DIR}
        CONFIG_FILE_STANZA="--config-files /src/buildroot${LINUX_CONFIG_DIR}/${LINUX_CONFIG_FILE} "
    fi

    # include post-install and before-remove script
    mv -v ${TAR_PATH}/examples/systemd/post-install ${PACKAGE_TEMPDIR}
    mv -v ${TAR_PATH}/examples/systemd/before-remove ${PACKAGE_TEMPDIR}

    # create versions folder
    mkdir -p ${PACKAGE_TEMPDIR}/buildroot${LINUX_DATA_DIR}/versions

    # /var/lib/teleport
    # shellcheck disable=SC2174
    mkdir -p -m0700 ${PACKAGE_TEMPDIR}/buildroot${LINUX_DATA_DIR}
fi
popd

if [[ "${PACKAGE_TYPE}" == "pkg" ]]; then
    # erase any existing versions of the package in the output directory first
    rm -f ${PKG_FILENAME}

    if [[ "${SIGN_PKG}" == "true" ]]; then
        # run codesign to sign binaries
        for FILE in ${FILE_LIST}; do
            $DRY_RUN_PREFIX codesign -s "${DEVELOPER_ID_APPLICATION}" \
                -f \
                -v \
                --timestamp \
                --options runtime \
                ${PACKAGE_TEMPDIR}/${FILE}
        done
    fi

    # build the package for OS X
    pkgbuild \
        --root ${PACKAGE_TEMPDIR}/${TAR_PATH} \
        --identifier ${BUNDLE_ID} \
        --version ${TELEPORT_VERSION} \
        --install-location /usr/local/bin \
        ${PKG_FILENAME}

    if [[ "${SIGN_PKG}" == "true" ]]; then
        # mark package as unsigned first
        mv ${PKG_FILENAME} ${PKG_FILENAME}.unsigned

        # run productsign to sign package
        $DRY_RUN_PREFIX productsign \
            --sign "${DEVELOPER_ID_INSTALLER}" \
            --timestamp \
            ${PKG_FILENAME}.unsigned \
            ${PKG_FILENAME}
        [[ -n "$DRY_RUN_PREFIX" ]] && cp "$PKG_FILENAME.unsigned" "$PKG_FILENAME"

        # remove unsigned package after successful signing
        rm -f ${PKG_FILENAME}.unsigned

        notarize "$PKG_FILENAME" "$TEAMID" "$BUNDLE_ID"
    fi

    # checksum created packages
    for PACKAGE in *."${PACKAGE_TYPE}"; do
        shasum -a 256 ${PACKAGE} > ${PACKAGE}.sha256
    done
else
    # erase any existing packages of the same type/version/arch in the output directory first
    rm -vf ${OUTPUT_FILENAME}

    # build for other platforms
    docker run -v ${PACKAGE_TEMPDIR}:/src --rm ${EXTRA_DOCKER_OPTIONS} ${FPM_IMAGE} \
        fpm \
        --input-type dir \
        --output-type ${PACKAGE_TYPE} \
        --name ${RPM_NAME} \
        --version "${TELEPORT_VERSION}" \
        --maintainer "${MAINTAINER}" \
        --url "${DOCS_URL}" \
        --vendor "${VENDOR}" \
        --description "${DESCRIPTION} ${TYPE_DESCRIPTION}" \
        --architecture ${PACKAGE_ARCH} \
        --package ${OUTPUT_FILENAME} \
        --chdir /src/buildroot \
        --directories ${LINUX_DATA_DIR} \
        --provides teleport \
        --prefix / \
        --verbose \
        "$AFTER_INSTALL_TARGET" /src/post-install \
        --before-remove /src/before-remove \
        ${CONFIG_FILE_STANZA} \
        ${FILE_PERMISSIONS_STANZA} \
        "${LICENSE_STANZA[@]}" \
        ${RPM_SIGN_STANZA} .

    # copy created package back to current directory
    cp ${PACKAGE_TEMPDIR}/*."${PACKAGE_TYPE}" .

    # checksum created packages
    for FILE in *."${PACKAGE_TYPE}"; do
        sha256sum ${FILE} > ${FILE}.sha256
    done
fi
