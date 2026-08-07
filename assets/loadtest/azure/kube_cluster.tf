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
    vm_size    = "Standard_D8s_v6" # 8 cpu 32gb ram
    node_count = 6
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

  lifecycle {
    ignore_changes = [
      default_node_pool.0.upgrade_settings,
    ]
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
