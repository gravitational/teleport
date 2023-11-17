resource "random_pet" "kube_cluster" {
  prefix = local.name_prefix
}

resource "azurerm_kubernetes_cluster" "kube_cluster" {
  name                = random_pet.kube_cluster.id
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  dns_prefix = random_pet.kube_cluster.id

  default_node_pool {
    name       = "defaultpool"
    vm_size    = "Standard_D16s_v3" # 16 cpu 64gb ram
    node_count = 3
  }

  identity {
    type = "SystemAssigned"
  }

  oidc_issuer_enabled       = true
  workload_identity_enabled = true

  network_profile {
    network_plugin = "kubenet"
    network_policy = "calico"
  }
}

/*
# to use an ACR registry, add details here and point the teleport image at nameofacr.azurecr.io/foo

data "azurerm_container_registry" "acr" {
  name                = "name of the registry"
  resource_group_name = "resource group of the registry"
}

resource "azurerm_role_assignment" "acr" {
  scope                = data.azurerm_container_registry.acr.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_kubernetes_cluster.kube_cluster.kubelet_identity.0.object_id

  skip_service_principal_aad_check = true
}
*/
