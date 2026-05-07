locals {
  create                = var.create
  azure_subscription_id = one(data.azurerm_client_config.this[*].subscription_id)
  azure_tenant_id       = one(data.azurerm_client_config.this[*].tenant_id)
  apply_azure_tags = merge(var.apply_azure_tags, {
    "TeleportCluster"     = local.teleport_cluster_name
    "TeleportIntegration" = local.teleport_integration_name
    "TeleportIACTool"     = "terraform"
  })
  apply_teleport_resource_labels = merge(var.apply_teleport_resource_labels, {
    "teleport.dev/iac-tool" = "terraform",
  })
  apply_teleport_integration_labels = merge(local.apply_teleport_resource_labels, {
    "teleport.dev/azure-managed-identity-region"         = var.azure_managed_identity_location
    "teleport.dev/azure-managed-identity-resource-group" = var.azure_resource_group_name
  })

  teleport_cluster_name         = local.teleport_ping.cluster_name
  teleport_ping                 = jsondecode(data.http.teleport_ping.response_body)
  teleport_proxy_public_url     = "https://${var.teleport_proxy_public_addr}"
  teleport_resource_name_suffix = random_id.suffix.hex
}

resource "random_id" "suffix" {
  byte_length = 4
}

data "azurerm_client_config" "this" {
  count = local.create_azure_managed_identity ? 1 : 0
}

data "http" "teleport_ping" {
  request_headers = { Accept = "application/json" }
  url             = "${local.teleport_proxy_public_url}/webapi/find"
}
