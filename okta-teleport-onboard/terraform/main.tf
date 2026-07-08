locals {
  acs      = "https://${var.teleport_domain}/v1/webapi/saml/acs/okta"
  jwks_uri = "https://${var.teleport_domain}/v1/.well-known/jwks-okta"
  suffix   = var.label_suffix != "" ? var.label_suffix : var.teleport_domain
}

data "okta_group" "sso" {
  name = var.sso_group
}

# --- SSO: SAML app + group assignment ---
resource "okta_app_saml" "teleport" {
  label                    = "Teleport connector (${local.suffix})"
  sso_url                  = local.acs
  audience                 = local.acs
  destination              = local.acs
  recipient                = local.acs
  subject_name_id_format   = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
  subject_name_id_template = "$${user.userName}"
  authn_context_class_ref  = "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"
  response_signed          = true
  assertion_signed         = true
  signature_algorithm      = "RSA_SHA256"
  digest_algorithm         = "SHA256"
  honor_force_authn        = true

  attribute_statements {
    name      = "username"
    namespace = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"
    type      = "EXPRESSION"
    values    = ["user.login"]
  }
  attribute_statements {
    name         = "groups"
    namespace    = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"
    type         = "GROUP"
    filter_type  = "REGEX"
    filter_value = ".*"
    values       = []
  }
}

resource "okta_app_group_assignment" "sso" {
  app_id   = okta_app_saml.teleport.id
  group_id = data.okta_group.sso.id
}

# --- API access: OAuth service app + scopes ---
resource "okta_app_oauth" "api" {
  label                      = "Teleport API access (${local.suffix})"
  type                       = "service"
  token_endpoint_auth_method = "private_key_jwt"
  jwks_uri                   = local.jwks_uri
  issuer_mode                = "DYNAMIC"
  omit_secret                = true
  grant_types                = ["client_credentials"]
  response_types             = ["token"]
}

resource "okta_app_oauth_api_scope" "api" {
  app_id = okta_app_oauth.api.id
  issuer = var.okta_org_url
  scopes = [
    "okta.apps.read", "okta.apps.manage",
    "okta.groups.read", "okta.groups.manage",
    "okta.users.read", "okta.users.manage",
  ]
}

# --- Scoped admin role + resource set + binding to the service app ---
resource "okta_admin_role_custom" "teleport" {
  label       = "Teleport Sync (${local.suffix})"
  description = "Role for the Teleport Okta integration"
  permissions = [
    "okta.apps.read", "okta.apps.assignment.manage",
    "okta.groups.read", "okta.groups.members.manage",
    "okta.users.read", "okta.users.appAssignment.manage",
  ]
}

resource "okta_resource_set" "teleport" {
  label       = "Teleport Sync Resources (${local.suffix})"
  description = "Users, apps and groups managed by the Teleport integration"
  resources = [
    "${var.okta_org_url}/api/v1/users",
    "${var.okta_org_url}/api/v1/apps",
    "${var.okta_org_url}/api/v1/groups",
  ]
}

resource "okta_app_oauth_role_assignment" "teleport" {
  client_id    = okta_app_oauth.api.client_id
  type         = "CUSTOM"
  role         = okta_admin_role_custom.teleport.id
  resource_set = okta_resource_set.teleport.id
}
