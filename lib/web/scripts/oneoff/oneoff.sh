#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportFlavor='{{.TeleportFlavor}}' # teleport or teleport-ent
successMessage='{{.SuccessMessage}}'

# shellcheck disable=all
tempDir=$({{.BinMktemp}} -d)
OS=$({{.BinUname}} -s)
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

teleportArgs='{{.TeleportArgs}}'

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
    cd $tempDir

    tarballName=$(teleportTarballName)
    curl --show-error --fail --location --remote-name ${cdnBaseURL}/${tarballName}
    echo "Extracting teleport to $tempDir ..."
    tar -xzf ${tarballName}

    mkdir -p ./bin
    mv ./${teleportFlavor}/teleport ./bin/teleport
    echo "> ./bin/teleport ${teleportArgs}"
    ./bin/teleport ${teleportArgs} && echo $successMessage

    cd -
}

main
