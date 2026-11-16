locals {
  apply_azure_tags               = { origin = "example" }
  apply_teleport_resource_labels = { origin = "example" }

  # A Discovery Service watching this discovery group must already exist.
  # If you are using Teleport Cloud, you can use `cloud-discovery-group`.
  teleport_discovery_group = "discover-aks"

  # Matching rules for discovering AKS clusters.
  # This should include both public and private AKS clusters.
  matching_resource_groups = ["*"]
  matching_regions         = ["westus", "eastus"]
  matching_tags = {
    env = ["prod"]
  }

  # Azure Management Group ID is the UUID of the management group.
  azure_management_group_id = "00000000-0000-0000-0000-000000000000"

  # Deployment Subscription ID is the subscription where the Azure resources will be deployed.
  deployment_subscription_id = "11111111-1111-1111-1111-111111111111"
  public_matching_labels = {
    "PrivateCluster"     = ["false"]
    "teleport.dev/cloud" = ["Azure"]
  }

  # Private Subscription ID is the subscription where the private AKS clusters are deployed.
  # You can remove this if you do not have private AKS clusters, or convert it into a list so that multiple private subscriptions can be specified.
  private_subscription_id = "22222222-2222-2222-2222-222222222222"
  private_subnet_id       = "/subscriptions/${local.private_subscription_id}/resourceGroups/networking/providers/Microsoft.Network/virtualNetworks/private-network/subnets/teleport"
  private_matching_labels = {
    "PrivateCluster"     = ["true"]
    "teleport.dev/cloud" = ["Azure"]
    "subscription-id"    = [local.private_subscription_id]
  }
}

# Shared discovery and the public Kubernetes Service are hosted here.
resource "azurerm_resource_group" "example" {
  name     = "teleport-aks-access"
  location = "eastus"
  tags     = local.apply_azure_tags
}

# One shared discovery configuration for all subscriptions in the group.
module "azure_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = var.teleport_proxy_addr
  teleport_discovery_group_name = local.teleport_discovery_group

  azure_resource_group_name       = azurerm_resource_group.example.name
  azure_managed_identity_location = azurerm_resource_group.example.location

  # Discovery permissions inherit to all current and future descendant subscriptions.
  azure_management_group_id = local.azure_management_group_id

  azure_matchers = [
    {
      types           = ["aks"]
      subscriptions   = ["*"]
      resource_groups = local.matching_resource_groups
      regions         = local.matching_regions
      tags            = local.matching_tags
    }
  ]

  apply_azure_tags               = local.apply_azure_tags
  apply_teleport_resource_labels = local.apply_teleport_resource_labels
}

# Public AKS clusters in the selected subscription and regions.
# No subnet is set on these replicas.
module "azure_agent" {
  source = "../../../../agent/azure"

  teleport_proxy_public_addr    = var.teleport_proxy_addr
  teleport_discovery_group_name = local.teleport_discovery_group

  azure_management_group_id = local.azure_management_group_id

  resource_group_name          = azurerm_resource_group.example.name
  location                     = azurerm_resource_group.example.location
  apply_azure_tags             = local.apply_azure_tags
  container_group_name         = "teleport-agent-${azurerm_resource_group.example.location}"
  log_analytics_workspace_name = "teleport-${azurerm_resource_group.example.location}"

  kubernetes_service = {
    enabled                 = true
    resource_matcher_labels = local.public_matching_labels
  }
}

# This location must match the existing private subnet's VNet region.
resource "azurerm_resource_group" "private" {
  provider = azurerm.private

  name     = "teleport-aks-access-private"
  location = "eastus"
  tags     = local.apply_azure_tags
}

# Private AKS clusters reachable from this subnet.
# Repeat this provider/module pattern for additional private networks, selecting the clusters each can reach.
module "azure_agent_private" {
  source = "../../../../agent/azure"

  providers = {
    azurerm = azurerm.private
  }

  create = local.private_subnet_id != null

  teleport_proxy_public_addr    = var.teleport_proxy_addr
  teleport_discovery_group_name = local.teleport_discovery_group

  subnet_id = local.private_subnet_id

  resource_group_name          = azurerm_resource_group.private.name
  location                     = azurerm_resource_group.private.location
  apply_azure_tags             = local.apply_azure_tags
  container_group_name         = "teleport-agent-${azurerm_resource_group.private.location}"
  log_analytics_workspace_name = "teleport-${azurerm_resource_group.private.location}"

  kubernetes_service = {
    enabled                 = true
    resource_matcher_labels = local.private_matching_labels
  }
}
