locals {
  apply_azure_tags               = { origin = "example" }
  apply_teleport_resource_labels = { origin = "example" }

  # A Discovery Service watching this discovery group must already exist.
  # If you are using Teleport Cloud, you can use `cloud-discovery-group`.
  teleport_discovery_group = "discover-aks"
  matching_resource_groups = ["*"]
  matching_regions         = ["westus", "eastus"]
  matching_tags = {
    env = ["prod"]
  }
}

# Resource group that will be used to create resources related to the Discovery and Agent modules.
# You can use two resource groups if you want to separate the resources for the Discovery and Agent modules.
resource "azurerm_resource_group" "example" {
  name     = "teleport-aks-discovery-access"
  location = "eastus"
  tags     = local.apply_azure_tags
}

# Azure Discovery module creates the required resources for discovering AKS clusters.
# But it doesn't provide a way to access them (see the Azure Agent module below).
module "azure_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = var.teleport_proxy_addr
  teleport_discovery_group_name = local.teleport_discovery_group

  azure_resource_group_name       = azurerm_resource_group.example.name
  azure_managed_identity_location = azurerm_resource_group.example.location

  # Configure AKS matcher: resource groups, regions and tags to monitor.
  azure_matchers = [
    {
      types           = ["aks"]
      subscriptions   = [data.azurerm_client_config.current.subscription_id]
      resource_groups = local.matching_resource_groups
      regions         = local.matching_regions
      tags            = local.matching_tags
    }
  ]

  apply_azure_tags               = local.apply_azure_tags
  apply_teleport_resource_labels = local.apply_teleport_resource_labels
}

# Azure Agent module deploys a Kubernetes Service that will proxy the discovered AKS clusters, providing access to them.
module "azure_agent" {
  source = "../../../../agent/azure"

  teleport_proxy_public_addr    = var.teleport_proxy_addr
  teleport_discovery_group_name = local.teleport_discovery_group

  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  apply_azure_tags    = local.apply_azure_tags

  container_group_name = "teleport-aks-access"

  # When setting up access to private AKS clusters, set subnet_id to the resource ID of the subnet which will host the Azure Container Instance.
  # The subnet must be delegated to Microsoft.ContainerInstance/containerGroups and have access to
  # - the Teleport Proxy Service (var.teleport_proxy_addr)
  # - the Teleport image registry at https://public.ecr.aws/
  # - Azure Resource Manager at https://management.azure.com/ (HTTPS/TCP 443) to read cluster details and fetch credentials
  # - the target AKS API endpoints (DNS resolution and connectivity)
  # subnet_id = <subnet_id>

  # Keep the labels in sync with the configuration in the discovery module under the azure_matchers.
  kubernetes_service = {
    enabled = true
    resource_matcher_labels = {
      "region"             = local.matching_regions
      "teleport.dev/cloud" = ["Azure"]
      "subscription-id"    = [data.azurerm_client_config.current.subscription_id]
      "resource-group"     = local.matching_resource_groups
    }
  }
}
