locals {
  apply_azure_tags               = { origin = "example" }
  apply_teleport_resource_labels = { origin = "example" }
}

data "azurerm_client_config" "current" {}

# Resource group for the Azure resources created by the module.
resource "azurerm_resource_group" "example" {
  name     = "teleport-discovery"
  location = "eastus"
  tags     = local.apply_azure_tags
}

module "azure_discovery" {
  source = "../.."

  teleport_discovery_group_name = "cloud-discovery-group"
  teleport_proxy_public_addr    = "example.teleport.sh:443"

  # Name of an existing Azure Resource Group where Azure resources will be created.
  azure_resource_group_name = azurerm_resource_group.example.name
  # Region where Azure managed identity will be created (westus)
  azure_managed_identity_location = azurerm_resource_group.example.location

  # Discover Azure VMs across all resource groups in the current subscription
  # and matching the specified tags.
  azure_matchers = [
    {
      types           = ["vm"]
      subscriptions   = [data.azurerm_client_config.current.subscription_id]
      resource_groups = ["*"]
      regions         = ["westus", "eastus"]
      tags = {
        env = ["example"]
      }
    }
  ]

  # Apply the additional Azure tag "origin=example" to all Azure resources created by this module
  apply_azure_tags = local.apply_azure_tags
  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = local.apply_teleport_resource_labels
  # Using a custom installer script on discovered VMs instead of the default installer script.
  teleport_installer_script_name = teleport_installer.example.metadata.name
}

resource "teleport_installer" "example" {
  version = "v1"
  metadata = {
    name        = "custom-azure-installer-example"
    description = "Example Teleport Installer"
    labels      = local.apply_teleport_resource_labels
  }

  spec = {
    # This "custom" script is actually the default installer script (you can view the default installer to verify: `$ tctl get installer/default-installer`).
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
EOF
  }
}
