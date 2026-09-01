# Teleport SAML connector creation

# Create the SAML connector in Teleport
resource "teleport_saml_connector" "entra_id" {
  version = "v2"
  metadata = {
    name = var.auth_connector_name
  }

  spec = {
    # ACS URL where SAML responses  sent
    acs = "${local.proxy_url}/v1/webapi/saml/acs/${var.auth_connector_name}"

    # Entity descriptor URL from Azure AD
    entity_descriptor_url = "https://login.microsoftonline.com/${data.azuread_client_config.current.tenant_id}/federationmetadata/2007-06/federationmetadata.xml?appid=${azuread_application.teleport_saml.client_id}"

    # Map Azure AD groups to Teleport roles
    attributes_to_roles = var.attributes_to_roles

    # Display name in Teleport UI
    display = var.display

    # Allow IdP-initiated login
    allow_idp_initiated = var.allow_idp_initiated
  }

  # Wait for Azure resources to be fully provisioned
  depends_on = [
    azuread_application.teleport_saml,
    azuread_service_principal.teleport_saml,
    azuread_service_principal_token_signing_certificate.teleport_saml
  ]
}
