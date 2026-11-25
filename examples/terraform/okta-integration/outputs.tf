output "api_app_client_id" {
  description = "Client ID of the Okta API Services app"
  value       = okta_app_oauth.teleport_okta_integration_api.client_id
}

output "okta_org_url" {
  description = "URL of the Okta organization"
  value       = local.okta_org_url
}

output "saml_metadata_url" {
  description = "Metadata URL of the Okta SAML app"
  # The provider's metadata_url attribute points at the admin API endpoint
  # (/api/v1/apps/<app id>/sso/saml/metadata), which requires an API token.
  # Teleport needs the public per-app metadata URL, which is keyed on the
  # app's entity key (exk...) rather than its app ID (0oa...).
  value = "${local.okta_org_url}/app/${okta_app_saml.teleport_okta_integration.entity_key}/sso/saml/metadata"
}
