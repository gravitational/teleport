data "azurerm_resource_group" "teleport_discovery" {
  name = "my-rg"
}

module "azure_discovery" {
  source = "terraform.releases.teleport.dev/teleport/discovery/azure"

  teleport_proxy_public_addr    = "azure-tenant.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  azure_resource_group_name       = data.azurerm_resource_group.teleport_discovery.name
  azure_managed_identity_location = data.azurerm_resource_group.teleport_discovery.location

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = ["00000000-0000-0000-0000-000000000001"]
    }
  ]
}

output "integration_name" { value = module.azure_discovery.teleport_integration_name }
output "discovery_config_name" { value = module.azure_discovery.teleport_discovery_config_name }
output "provision_token_name" { value = module.azure_discovery.teleport_provision_token_name }
