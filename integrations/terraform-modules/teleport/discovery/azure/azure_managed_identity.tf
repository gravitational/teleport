################################################################################
# Managed identity + federation
################################################################################

locals {
  create_azure_managed_identity = local.create && var.create_azure_managed_identity
  azure_managed_identity_name = (
    var.azure_managed_identity_use_name_prefix
    ? join("-", compact([var.azure_managed_identity_name, local.teleport_resource_name_suffix]))
    : var.azure_managed_identity_name
  )
}

# User-assigned managed identity for discovery
resource "azurerm_user_assigned_identity" "teleport_discovery_service" {
  count = local.create_azure_managed_identity ? 1 : 0

  location            = var.azure_managed_identity_location
  name                = local.azure_managed_identity_name
  resource_group_name = var.azure_resource_group_name
  tags                = local.apply_azure_tags

  lifecycle {
    precondition {
      condition     = var.azure_resource_group_name != null
      error_message = "azure_resource_group_name is required when create_azure_managed_identity is true."
    }
    precondition {
      condition     = var.azure_managed_identity_location != null
      error_message = "azure_managed_identity_location is required when create_azure_managed_identity is true."
    }
  }
}

# Federated identity credential for the managed identity (trust Teleport proxy issuer)
resource "azurerm_federated_identity_credential" "teleport_discovery_service" {
  count = local.use_oidc_integration && local.create_azure_managed_identity ? 1 : 0

  audience = ["api://AzureADTokenExchange"]
  # Extract the host from proxy_addr (format: host:port) to construct the OIDC issuer URL
  issuer                    = replace(local.teleport_proxy_public_url, "/:[0-9]+.*/", "")
  name                      = var.azure_federated_identity_credential_name
  user_assigned_identity_id = one(azurerm_user_assigned_identity.teleport_discovery_service[*].id)
  subject                   = "teleport-azure"
}
