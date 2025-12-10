output "client_id" {
  description = "Client ID used by the Teleport Azure OIDC integration."
  value       = local.client_id
}

output "principal_id" {
  description = "Principal ID used for role assignment."
  value       = local.principal_id
}

output "token_name" {
  description = "Teleport provision token name."
  value       = teleport_provision_token.azure_token.metadata.name
}

output "integration_name" {
  description = "Teleport integration resource name."
  value       = teleport_integration.azure_oidc.metadata.name
}

output "role_definition_id" {
  description = "ID of the custom role definition."
  value       = azurerm_role_definition.teleport_discovery.role_definition_resource_id
}

output "role_assignment_id" {
  description = "ID of the role assignment granting discovery permissions."
  value       = azurerm_role_assignment.teleport_discovery_assignment.id
}

output "managed_identity_id" {
  description = "Managed identity resource ID."
  value       = azurerm_user_assigned_identity.teleport.id
}
