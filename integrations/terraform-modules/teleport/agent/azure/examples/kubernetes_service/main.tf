locals {
  apply_azure_tags = {
    owner = "my-name-or-email"
  }
}

# Resource group for the Azure resources created by the module.
resource "azurerm_resource_group" "example" {
  name     = "resource-group-for-teleport-agents"
  location = "spaincentral"
  tags     = local.apply_azure_tags
}

module "agent" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "azure-prod"

  # Enable managed updates
  managed_updates_enabled = true
  managed_updates_group   = "default"

  apply_azure_tags = local.apply_azure_tags

  container_group_name = "teleport-kubernetes-service"
  location             = azurerm_resource_group.example.location
  resource_group_name  = azurerm_resource_group.example.name

  # When setting up access to private AKS clusters, set subnet_id to the resource ID of the subnet which will host the Azure Container Instance.
  # The subnet must be delegated to Microsoft.ContainerInstance/containerGroups and have access to
  # - the Teleport Proxy Service (var.teleport_proxy_addr)
  # - the Teleport image registry at https://public.ecr.aws/
  # - Azure Resource Manager at https://management.azure.com/ (HTTPS/TCP 443) to read cluster details and fetch credentials
  # - the target AKS API endpoints (DNS resolution and connectivity)
  # subnet_id = <subnet_id>

  kubernetes_service = {
    enabled = true
    resource_matcher_labels = {
      "region"             = ["*"]
      "teleport.dev/cloud" = ["Azure"]
      "subscription-id"    = [data.azurerm_client_config.this.subscription_id]
      "resource-group"     = ["my-resource-group-with-aks-clusters"]
    }
  }
}
