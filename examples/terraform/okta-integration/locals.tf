# Here live locals shared by more than once .tf file.
locals {
  okta_org_url      = "https://${var.okta_org_name}.${var.okta_base_url}"
  teleport_acs_url  = "https://${var.proxy_service_address}/v1/webapi/saml/acs/${var.connector_name}"
  teleport_jwks_url = "https://${var.proxy_service_address}/.well-known/jwks-okta"
}
