################################################################################
# Azure OIDC federation 
################################################################################

locals {
  create_teleport_integration = local.create && var.create_teleport_integration

  teleport_integration_name = (
    var.teleport_integration_use_name_prefix
    ? "${var.teleport_integration_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_integration_name
  )
}

# Teleport Azure OIDC integration using the selected identity
resource "teleport_integration" "azure_oidc" {
  count = local.create_teleport_integration ? 1 : 0

  metadata = {
    description = "Azure OIDC integration for Azure discovery."
    labels      = local.apply_teleport_integration_labels
    name        = local.teleport_integration_name
  }
  spec = {
    azure_oidc = {
      client_id = one(azurerm_user_assigned_identity.teleport_discovery_service[*].client_id)
      tenant_id = local.azure_tenant_id
    }
  }
  sub_kind = "azure-oidc"
  version  = "v1"

  lifecycle {
    precondition {
      condition     = local.create_azure_managed_identity
      error_message = "create_teleport_integration requires create_azure_managed_identity to be true."
    }
  }

  depends_on = [
    # Don't create the integration until the federated credential and permissions are in place.
    # This should avoid a ~5 minute delay that can happen if the discovery service tries to run before it has permissions.
    azurerm_federated_identity_credential.teleport_discovery_service,
    azurerm_role_assignment.teleport_discovery
  ]
}
