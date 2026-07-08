output "saml_metadata_url" {
  description = "Public SAML metadata URL for the Teleport connector (Teleport fetches this anonymously)"
  # NB: okta_app_saml.metadata_url is the /api/v1/apps/... endpoint, which is SSWS-gated
  # (403 anonymously). The public URL uses the SAML IdP config id (entity_key = exk...).
  value = "${var.okta_org_url}/app/${okta_app_saml.teleport.entity_key}/sso/saml/metadata"
}

output "oauth_client_id" {
  description = "OAuth service-app client ID for the Teleport plugin credential"
  value       = okta_app_oauth.api.client_id
}

output "saml_app_id" {
  value = okta_app_saml.teleport.id
}
