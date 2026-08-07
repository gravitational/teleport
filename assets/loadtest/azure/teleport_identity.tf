resource "azurerm_user_assigned_identity" "teleport_identity" {
  name                = "${azurerm_resource_group.rg.name}-teleport"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
}

resource "azurerm_federated_identity_credential" "teleport_identity" {
  name = "${azurerm_user_assigned_identity.teleport_identity.name}-${azurerm_kubernetes_cluster.kube_cluster.name}"

  parent_id = azurerm_user_assigned_identity.teleport_identity.id

  audience = ["api://AzureADTokenExchange"]

  issuer  = azurerm_kubernetes_cluster.kube_cluster.oidc_issuer_url
  subject = "system:serviceaccount:${local.teleport_namespace}:${local.teleport_release}"
}
