#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportFlavor='{{.TeleportFlavor}}' # teleport or teleport-ent
successMessage='{{.SuccessMessage}}'
teleportArgs='{{.TeleportArgs}}'

# shellcheck disable=all
# Use $HOME or / as base dir
tempDir=$({{.BinMktemp}} -d -p ${HOME:-}/)
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    if [ ${OS} = "Darwin" ]; then
        echo ${teleportFlavor}-${teleportVersion}-darwin-universal-bin.tar.gz
        return 0
    fi;

    if [ ${OS} != "Linux" ]; then
        echo "Only MacOS and Linux are supported." >&2
        return 1
    fi;

    if [ ${ARCH} = "armv7l" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm-bin.tar.gz"
    elif [ ${ARCH} = "aarch64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-arm64-bin.tar.gz"
    elif [ ${ARCH} = "x86_64" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-amd64-bin.tar.gz"
    elif [ ${ARCH} = "i686" ]; then echo "${teleportFlavor}-${teleportVersion}-linux-386-bin.tar.gz"
    else
        echo "Invalid Linux architecture ${ARCH}." >&2
        return 1
    fi;
}

main() {
    tarballName=$(teleportTarballName)
    echo "Downloading from ${cdnBaseURL}/${tarballName} and extracting teleport to ${tempDir} ..."
    curl --show-error --fail --location ${cdnBaseURL}/${tarballName} | tar xzf - -C ${tempDir} ${teleportFlavor}/teleport

    mkdir -p ${tempDir}/bin
    mv ${tempDir}/${teleportFlavor}/teleport ${tempDir}/bin/teleport
    echo "> ${tempDir}/bin/teleport ${teleportArgs} $@"
    {{.TeleportCommandPrefix}} ${tempDir}/bin/teleport ${teleportArgs} $@ && echo $successMessage
}

main $@
