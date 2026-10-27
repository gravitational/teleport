#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportFlavor='{{.TeleportFlavor}}' # teleport or teleport-ent
successMessage='{{.SuccessMessage}}'
entrypointArgs='{{.EntrypointArgs}}'
entrypoint='{{.Entrypoint}}'
packageSuffix='{{ if .TeleportFIPS }}fips-{{ end }}bin.tar.gz'
fips='{{ if .TeleportFIPS }}true{{ end }}'
cacheDir='{{ if .CacheDir }}{{.CacheDir}}/{{.TeleportVersion}}{{ end }}'
logToStderr='{{ if .LogToStderr }}true{{ end }}'

# The cache dirs mkdir makes are not writable by group or others; checkCache covers the binaries.
if [ -n "${cacheDir}" ]; then umask 022; mkdir -p "${cacheDir}"; fi
# shellcheck disable=all
# Use $HOME or / as base dir, or the cache dir, so the download moves into it with a rename
tempDir=$({{.BinMktemp}} -d -p ${cacheDir:-${HOME:-}/})
binDir="${cacheDir:-${tempDir}/bin}"
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    if [ "${OS}" = "Darwin" ]; then
        if [ "$fips" = "true" ]; then
            echo "FIPS version of Teleport is not compatible with MacOS. Please run this script in a Linux machine."
            return 1
        fi
        echo "${teleportFlavor}-${teleportVersion}-darwin-universal-${packageSuffix}"
        return 0
    fi;

    if [ "${OS}" != "Linux" ]; then
        echo "Only MacOS and Linux are supported." >&2
        return 1
    fi;

    if [ ${ARCH} = "armv7l" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm-${packageSuffix}"
    elif [ ${ARCH} = "aarch64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm64-${packageSuffix}"
    elif [ ${ARCH} = "x86_64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-amd64-${packageSuffix}"
    elif [ ${ARCH} = "i686" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-386-${packageSuffix}"
    else
        echo "Invalid Linux architecture ${ARCH}." >&2
        return 1
    fi;
}

log() {
    if [ -n "$logToStderr" ]; then echo "$@" >&2; else echo "$@"; fi
}

download() {
    tarballName=$(teleportTarballName)
    log "Downloading from ${cdnBaseURL}/${tarballName} and extracting teleport to ${tempDir} ..."
    # A cached binary outlives the run, so it must not keep the tarball's owner.
    # A cached binary keeps running as root, so its download never follows a redirect off https.
    curl ${logToStderr:+--silent} ${cacheDir:+--proto-redir =https} --show-error --fail --location "${cdnBaseURL}/${tarballName}" | tar xzf - ${cacheDir:+--no-same-owner} -C "${tempDir}" "${teleportFlavor}/${entrypoint}"

    mkdir -p "${binDir}"
    mv "${tempDir}/${teleportFlavor}/${entrypoint}" "${binDir}/${entrypoint}"
}

# checkCache exits unless the path and every directory above it belong to root
# or this user and, unless sticky, are not writable by group or others, and
# none is a symlink: a cached binary runs without being downloaded again. find
# prints only a path that passes, so a find that fails refuses it too.
checkCache() {
    p="$1"
    while :; do
        if [ -L "${p}" ] || [ -z "$(find "${p}" -maxdepth 0 \( -user 0 -o -user "$(id -u)" \) \( -perm -1000 -o ! -perm -020 ! -perm -002 \) -print)" ]; then
            echo "Refusing to run from the cache: ${p} is a symlink, has an owner other than root or the current user, or is writable by group or others." >&2
            exit 1
        fi
        if [ "${p}" = / ]; then break; fi
        p=$(dirname "${p}")
    done
}

main() {
    if [ ! -x "${binDir}/${entrypoint}" ]; then
        download
    fi
    log "> ${binDir}/${entrypoint} ${entrypointArgs} $@"
    if [ -n "${cacheDir}" ]; then
        checkCache "${binDir}/${entrypoint}"
        # The cached binary replaces the script, so ending the session ends it.
        rm -rf -- "${tempDir}"
        exec {{.TeleportCommandPrefix}} "${binDir}/${entrypoint}" ${entrypointArgs} $@
    fi
    {{.TeleportCommandPrefix}} "${binDir}/${entrypoint}" ${entrypointArgs} $@ && log "$successMessage"
}

main $@
