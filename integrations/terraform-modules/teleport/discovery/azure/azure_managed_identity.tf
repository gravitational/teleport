################################################################################
# Managed identity + federation
################################################################################

# User-assigned managed identity for discovery
resource "azurerm_user_assigned_identity" "teleport_discovery_service" {
  count = local.create ? 1 : 0

  location            = var.azure_managed_identity_location
  name                = var.azure_managed_identity_name
  resource_group_name = var.azure_resource_group_name
  tags                = local.apply_azure_tags
}

# Federated identity credential for the managed identity (trust Teleport proxy issuer)
resource "azurerm_federated_identity_credential" "teleport_discovery_service" {
  count = local.create ? 1 : 0

  audience = ["api://AzureADTokenExchange"]
  # Extract the host from proxy_addr (format: host:port) to construct the OIDC issuer URL
  issuer              = replace(local.teleport_proxy_public_url, "/:[0-9]+.*/", "")
  name                = var.azure_federated_identity_credential_name
  parent_id           = one(azurerm_user_assigned_identity.teleport_discovery_service[*].id)
  resource_group_name = var.azure_resource_group_name
  subject             = "teleport-azure"
}
