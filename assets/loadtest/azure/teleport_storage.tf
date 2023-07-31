resource "random_pet" "pgbk" {
  prefix = local.name_prefix
}

resource "azurerm_postgresql_flexible_server" "pgbk" {
  name                = random_pet.pgbk.id
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  version    = 15
  sku_name   = "GP_Standard_D2ds_v5" # 2 cpu 8gb ram
  storage_mb = 131072

  authentication {
    password_auth_enabled         = false
    active_directory_auth_enabled = true
    tenant_id                     = data.azurerm_client_config.current.tenant_id
  }

  high_availability {
    mode = "SameZone"
  }

  lifecycle {
    ignore_changes = [
      zone, high_availability.0.standby_availability_zone
    ]
  }
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "pgbk" {
  name             = "public"
  server_id        = azurerm_postgresql_flexible_server.pgbk.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "255.255.255.255"
}

resource "azurerm_postgresql_flexible_server_configuration" "pgbk" {
  server_id = azurerm_postgresql_flexible_server.pgbk.id

  name  = "wal_level"
  value = "logical"

  # this restarts the server, it's better to do it last
  depends_on = [
    azurerm_postgresql_flexible_server_firewall_rule.pgbk,
    azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_adminuser,
    azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_teleport,
  ]
}

resource "azurerm_postgresql_flexible_server_active_directory_administrator" "pgbk_adminuser" {
  server_name         = azurerm_postgresql_flexible_server.pgbk.name
  resource_group_name = azurerm_postgresql_flexible_server.pgbk.resource_group_name

  principal_name = "adminuser"

  object_id      = data.azurerm_client_config.current.object_id
  principal_type = "User"
  tenant_id      = data.azurerm_client_config.current.tenant_id
}



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


# identity used by teleport auth
resource "azurerm_user_assigned_identity" "teleport_identity" {
  name                = "${azurerm_resource_group.rg.name}-teleport"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
}

# teleport_identity can be used by the teleport auth service account
resource "azurerm_federated_identity_credential" "teleport_identity" {
  name = "${azurerm_user_assigned_identity.teleport_identity.name}-${azurerm_kubernetes_cluster.kube_cluster.name}"

  parent_id           = azurerm_user_assigned_identity.teleport_identity.id
  resource_group_name = azurerm_user_assigned_identity.teleport_identity.resource_group_name

  audience = ["api://AzureADTokenExchange"]

  issuer  = azurerm_kubernetes_cluster.kube_cluster.oidc_issuer_url
  subject = "system:serviceaccount:${local.teleport_namespace}:${local.teleport_release}"
}

# teleport_identity has a "teleport" postgres user
resource "azurerm_postgresql_flexible_server_active_directory_administrator" "pgbk_teleport" {
  server_name         = azurerm_postgresql_flexible_server.pgbk.name
  resource_group_name = azurerm_postgresql_flexible_server.pgbk.resource_group_name

  principal_name = "teleport"

  object_id      = azurerm_user_assigned_identity.teleport_identity.principal_id
  principal_type = "ServicePrincipal"
  tenant_id      = data.azurerm_client_config.current.tenant_id
}

# teleport_identity can use storage account
resource "azurerm_role_assignment" "teleport_identity" {
  scope                = azurerm_storage_account.azsessions.id
  role_definition_name = "Storage Blob Data Owner"
  principal_id         = azurerm_user_assigned_identity.teleport_identity.principal_id

  skip_service_principal_aad_check = true
}
