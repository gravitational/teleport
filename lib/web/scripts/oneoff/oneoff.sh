#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportFlavor='{{.TeleportFlavor}}' # teleport or teleport-ent
successMessage='{{.SuccessMessage}}'
teleportArgs='{{.TeleportArgs}}'
teleportBin='{{.TeleportBin}}'
teleportFIPSSuffix='{{ if .TeleportFIPS }}fips-{{ end}}'

# shellcheck disable=all
# Use $HOME or / as base dir
tempDir=$({{.BinMktemp}} -d -p ${HOME:-}/)
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    if [ ${OS} = "Darwin" ]; then
        echo "${teleportFlavor}-${teleportVersion}-darwin-universal-bin.tar.gz"
        return 0
    fi;

    if [ ${OS} != "Linux" ]; then
        echo "Only MacOS and Linux are supported." >&2
        return 1
    fi;

    if [ ${ARCH} = "armv7l" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm-${teleportFIPSSuffix}bin.tar.gz"
    elif [ ${ARCH} = "aarch64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm64-${teleportFIPSSuffix}bin.tar.gz"
    elif [ ${ARCH} = "x86_64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-amd64-${teleportFIPSSuffix}bin.tar.gz"
    elif [ ${ARCH} = "i686" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-386-${teleportFIPSSuffix}bin.tar.gz"
    else
        echo "Invalid Linux architecture ${ARCH}." >&2
        return 1
    fi;
}

main() {
    tarballName=$(teleportTarballName)
    echo "Downloading from ${cdnBaseURL}/${tarballName} and extracting teleport to ${tempDir} ..."
    curl --show-error --fail --location "${cdnBaseURL}/${tarballName}" | tar xzf - -C "${tempDir}" "${teleportFlavor}/${teleportBin}"

    mkdir -p "${tempDir}/bin"
    mv "${tempDir}/${teleportFlavor}/${teleportBin}" "${tempDir}/bin/${teleportBin}"
    echo "> ${tempDir}/bin/${teleportBin} ${teleportArgs} $@"
    {{.TeleportCommandPrefix}} "${tempDir}/bin/${teleportBin}" ${teleportArgs} $@ && echo "$successMessage"
}

main $@
