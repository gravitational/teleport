terraform {
  required_version = ">= 1.6.0"
}

module "teleport_azure_discovery" {
  source = "../.."

  subscription_id                = var.subscription_id
  tenant_id                      = var.tenant_id
  region                         = var.region
  discovery_resource_group_names = var.discovery_resource_group_names
  proxy_addr                     = var.proxy_addr

  # optional
  discovery_tags               = var.discovery_tags
  identity_resource_group_name = var.identity_resource_group_name
  tags                         = var.tags
  discovery_group_name         = var.discovery_group_name
  installer_script_name        = var.installer_script_name
}
