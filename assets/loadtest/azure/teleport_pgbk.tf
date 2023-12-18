resource "random_pet" "pgbk" {
  prefix = local.name_prefix
}

resource "azurerm_postgresql_flexible_server" "pgbk" {
  name                = random_pet.pgbk.id
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  version = 15

  # Standard_D2ds_v4: 2 vCPU, 8GiB of ram, 75 GiB temp storage
  # v4 because northeurope doesn't have v5 for Postgres
  sku_name   = "GP_Standard_D2ds_v4"
  storage_mb = 128 * 1024

  authentication {
    password_auth_enabled         = false
    active_directory_auth_enabled = true
    tenant_id                     = data.azurerm_client_config.current.tenant_id
  }

  high_availability {
    mode = "ZoneRedundant"
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

resource "azurerm_postgresql_flexible_server_configuration" "pgbk_wal_level" {
  server_id = azurerm_postgresql_flexible_server.pgbk.id

  name  = "wal_level"
  value = "logical"
}

resource "azurerm_postgresql_flexible_server_active_directory_administrator" "pgbk_adminuser" {
  server_name         = azurerm_postgresql_flexible_server.pgbk.name
  resource_group_name = azurerm_postgresql_flexible_server.pgbk.resource_group_name

  principal_name = "adminuser"

  object_id      = data.azurerm_client_config.current.object_id
  principal_type = "User"
  tenant_id      = data.azurerm_client_config.current.tenant_id
}

resource "azurerm_postgresql_flexible_server_active_directory_administrator" "pgbk_teleport" {
  server_name         = azurerm_postgresql_flexible_server.pgbk.name
  resource_group_name = azurerm_postgresql_flexible_server.pgbk.resource_group_name

  principal_name = "teleport"

  object_id      = azurerm_user_assigned_identity.teleport_identity.principal_id
  principal_type = "ServicePrincipal"
  tenant_id      = data.azurerm_client_config.current.tenant_id
}
