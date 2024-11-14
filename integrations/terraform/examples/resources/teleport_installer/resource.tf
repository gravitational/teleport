# Teleport Installer resource

resource "teleport_installer" "example" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Example Teleport Installer"
    labels = {
      example = "yes"
    }
  }

  spec = {
    # This is the default installer script. Edit it to customize the commands
    # that the Teleport Discovery Service configures virtual machines to run to
    # install Teleport on startup.
    script = <<EOF
#!/usr/bin/env sh
set -eu

cdnBaseURL='https://cdn.teleport.dev'
teleportVersion='v{{.MajorVersion}}'
teleportFlavor='teleport-ent' # teleport or teleport-ent
successMessage='Teleport is installed and running.'
teleportArgs='install autodiscover-node --public-proxy-addr={{.PublicProxyAddr}} --teleport-package={{.TeleportPackage}} --repo-channel={{.RepoChannel}} --auto-upgrade={{.AutomaticUpgrades}} --azure-client-id={{.AzureClientID}}'

# shellcheck disable=all
# Use $HOME or / as base dir
tempDir=$(mktemp -d -p $${HOME:-}/)
OS=$(uname -s)
ARCH=$(uname -m)
# shellcheck enable=all

trap 'rm -rf -- "$tempDir"' EXIT

teleportTarballName() {
    if [ $${OS} = "Darwin" ]; then
        echo $${teleportFlavor}-$${teleportVersion}-darwin-universal-bin.tar.gz
        return 0
    fi;

    if [ $${OS} != "Linux" ]; then
        echo "Only MacOS and Linux are supported." >&2
        return 1
    fi;

    if [ $${ARCH} = "armv7l" ]; then echo "$${teleportFlavor}-$${teleportVersion}-linux-arm-bin.tar.gz"
    elif [ $${ARCH} = "aarch64" ]; then echo "$${teleportFlavor}-$${teleportVersion}-linux-arm64-bin.tar.gz"
    elif [ $${ARCH} = "x86_64" ]; then echo "$${teleportFlavor}-$${teleportVersion}-linux-amd64-bin.tar.gz"
    elif [ $${ARCH} = "i686" ]; then echo "$${teleportFlavor}-$${teleportVersion}-linux-386-bin.tar.gz"
    else
        echo "Invalid Linux architecture $${ARCH}." >&2
        return 1
    fi;
}

main() {
    tarballName=$(teleportTarballName)
    echo "Downloading from $${cdnBaseURL}/$${tarballName} and extracting teleport to $${tempDir} ..."
    curl --show-error --fail --location $${cdnBaseURL}/$${tarballName} | tar xzf - -C $${tempDir} $${teleportFlavor}/teleport

    mkdir -p $${tempDir}/bin
    mv $${tempDir}/$${teleportFlavor}/teleport $${tempDir}/bin/teleport
    echo "> $${tempDir}/bin/teleport $${teleportArgs} $@"
    sudo $${tempDir}/bin/teleport $${teleportArgs} $@ && echo $successMessage
}

main $@
EOF
  }
}
