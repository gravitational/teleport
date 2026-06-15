output "azure_discovery_role_definition" {
  description = "The Azure role definition for the Teleport Discovery Service."
  value       = one(azurerm_role_definition.teleport_discovery[*])
}

output "azure_teleport_discovery_managed_identity" {
  description = "Managed identity created for the Teleport Discovery Service."
  value       = one(azurerm_user_assigned_identity.teleport_discovery_service[*])
}

output "teleport_discovery_config_name" {
  description = "Name of the Teleport dynamic `discovery_config`. Configuration details can be viewed with `tctl get discovery_config/<name>`. Teleport Discovery Service instances will use this `discovery_config` if they are in the same discovery group as the `discovery_config`."
  value       = try(teleport_discovery_config.azure[0].header.metadata.name, null)
}

output "teleport_integration_name" {
  description = "Name of the Teleport `integration` resource. The integration resource configures Teleport Discovery Service instances to assume an Azure managed identity for discovery using Azure OIDC federation. Integration details can be viewed with `tctl get integrations/<name>` or by visiting the Teleport web UI under 'Zero Trust Access' > 'Integrations'."
  value       = try(teleport_integration.azure_oidc[0].metadata.name, null)
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision `token` that allows Teleport nodes to join the Teleport cluster using Azure credentials. Token details can be viewed with `tctl get token/<name>`."
  value       = nonsensitive(local.teleport_provision_token_name)
}
