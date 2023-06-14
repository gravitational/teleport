#!/usr/bin/env bash
set -euo pipefail

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
tempDir=$({{.BinMktemp}} -d)
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
teleportArgs='{{.TeleportArgs}}'

function teleportTarballName(){    
    if [[ ${OS} == "Darwin" ]]; then
        echo teleport-${teleportVersion}-darwin-universal-bin.tar.gz
        return 0
    fi;

    if [[ ${OS} != "Linux" ]]; then
        echo "Only MacOS and Linux are supported." >&2
        return 1
    fi;

    if [[ ${ARCH} == "armv7l" ]]; then echo "teleport-${teleportVersion}-linux-arm-bin.tar.gz"
    elif [[ ${ARCH} == "aarch64" ]]; then echo "teleport-${teleportVersion}-linux-arm64-bin.tar.gz"
    elif [[ ${ARCH} == "x86_64" ]]; then echo "teleport-${teleportVersion}-linux-amd64-bin.tar.gz"
    elif [[ ${ARCH} == "i686" ]]; then echo "teleport-${teleportVersion}-linux-386-bin.tar.gz"
    else
        echo "Invalid Linux architecture ${ARCH}." >&2
        return 1
    fi;
}

function main() {
    pushd $tempDir > /dev/null

    tarballName=$(teleportTarballName)
    curl --show-error --fail --location --remote-name ${cdnBaseURL}/${tarballName}
    tar -xzf ${tarballName}

    echo "> ./teleport/teleport ${teleportArgs}"
    ./teleport/teleport ${teleportArgs}
    
    popd > /dev/null
}

main
