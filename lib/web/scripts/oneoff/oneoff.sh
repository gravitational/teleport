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

# shellcheck disable=all
# Use $HOME or / as base dir
tempDir=$({{.BinMktemp}} -d -p ${HOME:-}/)
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    if [ "${OS}" = "Darwin" ]; then
        if [ "$fips" = "true"]; then
            echo "FIPS install is not supported on MacOS"
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

main() {
    tarballName=$(teleportTarballName)
    echo "Downloading from ${cdnBaseURL}/${tarballName} and extracting teleport to ${tempDir} ..."
    curl --show-error --fail --location "${cdnBaseURL}/${tarballName}" | tar xzf - -C "${tempDir}" "${teleportFlavor}/${entrypoint}"

    mkdir -p "${tempDir}/bin"
    mv "${tempDir}/${teleportFlavor}/${entrypoint}" "${tempDir}/bin/${entrypoint}"
    echo "> ${tempDir}/bin/${entrypoint} ${entrypointArgs} $@"
    {{.TeleportCommandPrefix}} "${tempDir}/bin/${entrypoint}" ${entrypointArgs} $@ && echo "$successMessage"
}

main $@
