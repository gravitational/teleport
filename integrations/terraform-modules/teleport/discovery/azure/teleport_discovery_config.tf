################################################################################
# Teleport cluster discovery config
################################################################################

# Teleport discovery config targeting Azure VMs
resource "teleport_discovery_config" "azure" {
  count = local.create ? 1 : 0

  header = {
    metadata = {
      description = "Configure Teleport to discover Azure resources."
      labels      = local.apply_teleport_resource_labels
      name = (
        var.teleport_discovery_config_use_name_prefix
        ? "${var.teleport_discovery_config_name}-${local.teleport_resource_name_suffix}"
        : var.teleport_discovery_config_name
      )
    }
    version = "v1"
  }

  spec = {
    azure = [{
      install_params = {
        join_method = "azure"
        join_token  = try(teleport_provision_token.azure[0].metadata.name, "")
        script_name = var.teleport_installer_script_name
      }
      integration     = try(teleport_integration.azure_oidc[0].metadata.name, "")
      regions         = var.match_azure_regions
      resource_groups = compact(var.match_azure_resource_groups)
      subscriptions   = [local.azure_subscription_id]
      tags            = var.match_azure_tags
      types           = ["vm"]
    }]
    discovery_group = var.teleport_discovery_group_name
  }

  depends_on = [
    # Don't create the discovery config until the integration is in place.
    # This should avoid a ~5 minute delay that can happen if the discovery service tries to run before it has permissions.
    teleport_integration.azure_oidc,
  ]
}
