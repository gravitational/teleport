resource "random_pet" "azsessions" {
  prefix    = local.short_name_prefix
  separator = ""
}

resource "azurerm_storage_account" "azsessions" {
  name                = random_pet.azsessions.id
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  account_tier             = "Standard"
  account_replication_type = "ZRS"
}

resource "azurerm_role_assignment" "azsessions" {
  scope                = azurerm_storage_account.azsessions.id
  role_definition_name = "Storage Blob Data Owner"
  principal_id         = azurerm_user_assigned_identity.teleport_identity.principal_id

  skip_service_principal_aad_check = true
}
