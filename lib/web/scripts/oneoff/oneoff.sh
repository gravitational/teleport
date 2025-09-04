#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportFlavor='{{.TeleportFlavor}}' # teleport, teleport-ent or teleport-update
teleportPackage='{{.TeleportPackage}}'
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
    if [ "${OS}" != "Linux" ]; then
        echo "ERROR: This script works only for Linux. Please go to the downloads page to find the proper installation method for your operating system:" >&2
        echo "https://goteleport.com/download/" >&2
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
    curl --show-error --fail --location "${cdnBaseURL}/${tarballName}" | tar xzf - -C "${tempDir}" "${teleportPackage}/${entrypoint}"

    mkdir -p "${tempDir}/bin"
    mv "${tempDir}/${teleportPackage}/${entrypoint}" "${tempDir}/bin/${entrypoint}"
    echo "> ${tempDir}/bin/${entrypoint} ${entrypointArgs} $@"
    {{.TeleportCommandPrefix}} "${tempDir}/bin/${entrypoint}" ${entrypointArgs} $@ && echo "$successMessage"
}

main $@
