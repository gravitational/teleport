locals {
  create                = var.create
  azure_subscription_id = data.azurerm_client_config.this.subscription_id
  azure_tenant_id = data.azurerm_client_config.this.tenant_id
  apply_azure_tags = merge(var.apply_azure_tags, {
    "TeleportCluster"     = local.teleport_cluster_name
    "TeleportIntegration" = local.teleport_integration_name
    "TeleportIACTool"     = "terraform"
  })
  apply_teleport_resource_labels = merge(var.apply_teleport_resource_labels, {
    "teleport.dev/iac-tool" = "terraform",
  })

  teleport_cluster_name         = try(local.teleport_ping.cluster_name, "")
  teleport_ping                 = try(jsondecode(data.http.teleport_ping[0].response_body), null)
  teleport_proxy_public_url     = "https://${var.teleport_proxy_public_addr}"
  teleport_resource_name_suffix = "azure-${local.azure_subscription_id}"
}

data "azurerm_client_config" "this" {}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  request_headers = { Accept = "application/json" }
  url             = "${local.teleport_proxy_public_url}/webapi/find"
}
