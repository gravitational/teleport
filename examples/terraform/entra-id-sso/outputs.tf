output "application_id" {
  description = "Application (Client) ID"
  value       = azuread_application.teleport_saml.client_id
}

output "tenant_id" {
  description = "Azure AD Tenant ID"
  value       = data.azuread_client_config.current.tenant_id
}

output "federation_metadata_url" {
  description = "SAML Federation Metadata URL"
  value       = "https://login.microsoftonline.com/${data.azuread_client_config.current.tenant_id}/federationmetadata/2007-06/federationmetadata.xml?appid=${azuread_application.teleport_saml.client_id}"
}

output "teleport_connector_name" {
  description = "Name of the created SAML connector in Teleport"
  value       = teleport_saml_connector.entra_id.metadata.name
}
