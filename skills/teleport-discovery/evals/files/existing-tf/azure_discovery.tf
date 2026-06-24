module "azure_discovery" {
  source  = "terraform.releases.teleport.dev/teleport/discovery/azure"
  version = "~> 18.8"

  teleport_proxy_public_addr    = "test.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  azure_resource_group_name       = "my-rg"
  azure_managed_identity_location = "eastus"

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = ["00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"]
    }
  ]
}

output "azure_discovery" {
  value = module.azure_discovery
}
