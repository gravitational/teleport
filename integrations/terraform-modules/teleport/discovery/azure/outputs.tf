output "azure_managed_identity_client_id" {
  description = "Client ID used by the Teleport Azure OIDC integration."
  value       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].client_id)
}

output "azure_managed_identity_id" {
  description = "Managed identity resource ID."
  value       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].id)
}

output "azure_managed_identity_principal_id" {
  description = "Principal ID used for role assignment."
  value       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].principal_id)
}

output "azure_role_assignment_id" {
  description = "ID of the role assignment granting discovery permissions."
  value       = one(azurerm_role_assignment.teleport_discovery[*].id)
}

output "azure_role_definition_id" {
  description = "ID of the discovery role definition."
  value       = one(azurerm_role_definition.teleport_discovery[*].role_definition_resource_id)
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
  value       = nonsensitive(try(teleport_provision_token.azure[0].metadata.name, null))
}
