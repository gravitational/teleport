#!/usr/bin/env sh
set -eu

cdnBaseURL='{{.CDNBaseURL}}'
teleportVersion='{{.TeleportVersion}}'
teleportArtifact='{{.TeleportArtifact}}' # teleport, teleport-ent or teleport-update
teleportDirectory='{{.TeleportDirectory}}'
successMessage='{{.SuccessMessage}}'
entrypointArgs='{{.EntrypointArgs}}'
entrypoint='{{.Entrypoint}}'
packageSuffix='{{ if .TeleportFIPS }}fips-{{ end }}bin.tar.gz'
fips='{{ if .TeleportFIPS }}true{{ end }}'
supportedOSes='{{join .SupportedOSes " "}}'

# shellcheck disable=all
# Use $HOME or / as base dir
tempDir=$({{.BinMktemp}} -d -p ${HOME:-}/)
OS=$({{.BinUname}} -s | {{.BinTR}} '[:upper:]' '[:lower:]')
ARCH=$({{.BinUname}} -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    case " $supportedOSes " in
        *" $OS "*)
            # Supported os, nothing to do.
            ;;
        *)
            echo "ERROR: This script works only for: $supportedOSes. Please go to the downloads page to find the proper installation method for your operating system:" >&2
            echo "https://goteleport.com/download/" >&2
            return 1
            ;;
    esac

    if [ "${OS}" = "darwin" ]; then
        if [ "$fips" = "true" ]; then
            echo "FIPS version of Teleport is not compatible with MacOS. Please run this script in a Linux machine."
            return 1
        fi
        echo "${teleportArtifact}-${teleportVersion}-darwin-universal-${packageSuffix}"
        return 0
    fi
    if [ "${OS}" = "linux" ]; then
        if [ ${ARCH} = "armv7l" ]; then echo "${teleportArtifact}-${teleportVersion}-linux-arm-${packageSuffix}"
        elif [ ${ARCH} = "aarch64" ]; then echo "${teleportArtifact}-${teleportVersion}-linux-arm64-${packageSuffix}"
        elif [ ${ARCH} = "x86_64" ]; then echo "${teleportArtifact}-${teleportVersion}-linux-amd64-${packageSuffix}"
        elif [ ${ARCH} = "i686" ]; then echo "${teleportArtifact}-${teleportVersion}-linux-386-${packageSuffix}"
        else
            echo "Invalid Linux architecture ${ARCH}." >&2
            return 1
        fi;
        return 0
    fi
}

main() {
    tarballName=$(teleportTarballName)
    echo "Downloading from ${cdnBaseURL}/${tarballName} and extracting teleport to ${tempDir} ..."
    curl --show-error --fail --location "${cdnBaseURL}/${tarballName}" | tar xzf - -C "${tempDir}" "${teleportDirectory}/${entrypoint}"

    mkdir -p "${tempDir}/bin"
    mv "${tempDir}/${teleportDirectory}/${entrypoint}" "${tempDir}/bin/${entrypoint}"
    echo "> ${tempDir}/bin/${entrypoint} ${entrypointArgs} $@"
    {{.TeleportCommandPrefix}} "${tempDir}/bin/${entrypoint}" ${entrypointArgs} $@ && echo "$successMessage"
}

main $@
