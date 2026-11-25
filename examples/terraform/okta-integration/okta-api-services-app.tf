locals {
  # Read-only access is sufficient for user sync, app and group sync.
  read_api_scopes = [
    "okta.apps.read",
    "okta.groups.read",
    "okta.users.read",
  ]
  read_role_permissions = [
    "okta.users.read",
    "okta.groups.read",
    "okta.apps.read",
  ]

  # Bidirectional group sync needs write perms to change assignments in Okta.
  write_api_scopes = [
    "okta.apps.manage",
    "okta.groups.manage",
  ]
  write_role_permissions = [
    "okta.users.groupMembership.manage",
    "okta.users.appAssignment.manage",
    "okta.groups.members.manage",
    "okta.apps.assignment.manage",
  ]

  api_scopes = concat(
    local.read_api_scopes,
    var.enable_bidirectional_sync ? local.write_api_scopes : [],
  )
  admin_role_permissions = concat(
    local.read_role_permissions,
    var.enable_bidirectional_sync ? local.write_role_permissions : [],
  )
}

# Configure an API Services app that Teleport uses to call the Okta API for
# user sync and for app and group sync.
resource "okta_app_oauth" "teleport_okta_integration_api" {
  label          = var.api_app_name
  type           = "service"
  grant_types    = ["client_credentials"]
  response_types = ["token"]

  # Teleport authenticates with a JWT signed by its Okta key. Okta fetches the
  # public key from the Teleport JWKS endpoint, so there is no client secret.
  token_endpoint_auth_method = "private_key_jwt"
  jwks_uri                   = local.teleport_jwks_url
  omit_secret                = true

  # Teleport does not support DPoP for the Okta API Services app.
  dpop_bound_access_tokens = false
}

# Grant Okta API scopes to the app.
resource "okta_app_oauth_api_scope" "teleport_okta_integration_api" {
  app_id = okta_app_oauth.teleport_okta_integration_api.id
  issuer = local.okta_org_url
  scopes = local.api_scopes
}

# Configure a custom admin role with the permissions Teleport needs.
resource "okta_admin_role_custom" "teleport_okta_integration" {
  label       = var.admin_role_name
  description = "Permissions for the Teleport Okta integration"
  permissions = local.admin_role_permissions
}

# Configure the resources the custom admin role applies to. This example
# includes all users, groups, and apps. To limit what Teleport can access,
# list specific groups and apps instead, and include the Teleport SAML app.
resource "okta_resource_set" "teleport_okta_integration" {
  label       = var.resource_set_name
  description = "Resources managed by the Teleport Okta integration"
  resources = [
    "${local.okta_org_url}/api/v1/users",
    "${local.okta_org_url}/api/v1/groups",
    "${local.okta_org_url}/api/v1/apps",
  ]
}

# Assign the custom admin role to the app, scoped to the resource set.
resource "okta_app_oauth_role_assignment" "teleport_okta_integration_api" {
  client_id    = okta_app_oauth.teleport_okta_integration_api.client_id
  type         = "CUSTOM"
  role         = okta_admin_role_custom.teleport_okta_integration.id
  resource_set = okta_resource_set.teleport_okta_integration.id
}
