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
    # This "custom" script is actually the default installer script ($ tctl get installer/default-installer).
    # Edit it to customize the commands that the Teleport Discovery Service
    # configures virtual machines to run to install Teleport on startup.
    script = <<EOF
#!/usr/bin/env sh
set -eu


INSTALL_SCRIPT_URL="https://{{.PublicProxyAddr}}/scripts/install.sh"

echo "Offloading the installation part to the generic Teleport install script hosted at: $INSTALL_SCRIPT_URL"

TEMP_INSTALLER_SCRIPT="$(mktemp)"
curl -sSf "$INSTALL_SCRIPT_URL" -o "$TEMP_INSTALLER_SCRIPT"

chmod +x "$TEMP_INSTALLER_SCRIPT"

sudo -E "$TEMP_INSTALLER_SCRIPT" || (echo "The install script ($TEMP_INSTALLER_SCRIPT) returned a non-zero exit code" && exit 1)
rm "$TEMP_INSTALLER_SCRIPT"


echo "Configuring the Teleport agent"

set +x
TELEPORT_BINARY=/usr/local/bin/teleport
[ -z "$${TELEPORT_INSTALL_SUFFIX:-}" ] || TELEPORT_BINARY=/opt/teleport/$${TELEPORT_INSTALL_SUFFIX}/bin/teleport

sudo -E "$TELEPORT_BINARY" install autodiscover-node --public-proxy-addr={{.PublicProxyAddr}} --teleport-package={{.TeleportPackage}} --repo-channel={{.RepoChannel}} --auto-upgrade={{.AutomaticUpgrades}} --azure-client-id={{.AzureClientID}} $@
  }
}
EOF
  }
}
