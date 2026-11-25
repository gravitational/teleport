# Configure a SAML 2.0 app that Teleport uses for SSO.
resource "okta_app_saml" "teleport_okta_integration" {
  label = var.app_name

  # Single sign-on URL and Audience URI (SP Entity ID).
  sso_url     = local.teleport_acs_url
  recipient   = local.teleport_acs_url
  destination = local.teleport_acs_url
  audience    = local.teleport_acs_url

  # Name ID format EmailAddress, with the Okta username as the app username.
  subject_name_id_format   = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
  subject_name_id_template = "$${user.userName}"
  user_name_template       = "$${source.login}"

  response_signed         = true
  assertion_signed        = true
  signature_algorithm     = "RSA_SHA256"
  digest_algorithm        = "SHA256"
  authn_context_class_ref = "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"

  # Prompt users to authenticate again when the Teleport SAML connector
  # requests it with `force_authn`, for example for SSO MFA checks.
  honor_force_authn = true

  # Teleport maps the user's Okta groups to the `groups` trait.
  attribute_statements {
    type         = "GROUP"
    name         = "groups"
    filter_type  = "REGEX"
    filter_value = ".*"
  }

  # Teleport uses the Okta username as the Teleport username.
  attribute_statements {
    type   = "EXPRESSION"
    name   = "username"
    values = ["user.profile.login"]
  }
}

# Look up the Okta groups to assign to the Teleport SAML app.
data "okta_group" "assigned" {
  for_each = toset(var.group_names)
  name     = each.value
}

# Assign the Okta groups whose members can log in to Teleport.
resource "okta_app_group_assignment" "teleport_okta_integration" {
  for_each = data.okta_group.assigned

  app_id   = okta_app_saml.teleport_okta_integration.id
  group_id = each.value.id
}
