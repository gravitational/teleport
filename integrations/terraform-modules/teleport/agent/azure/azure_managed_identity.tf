locals {
  subscription_scope = var.create ? "/subscriptions/${data.azurerm_client_config.this[0].subscription_id}" : null

  managed_identity = one([
    for identity in azurerm_user_assigned_identity.teleport_agent : {
      id           = identity.id
      client_id    = identity.client_id
      principal_id = identity.principal_id
    }
  ])
}

resource "azurerm_user_assigned_identity" "teleport_agent" {
  count = var.create ? 1 : 0

  location            = var.location
  name                = local.container_group_name
  resource_group_name = var.resource_group_name
  tags                = var.apply_azure_tags
}
