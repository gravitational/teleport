locals {
  # Graph API permissions.
  # permissions = ["Application.ReadWrite.OwnedBy", "Group.Read.All", "User.Read.All"]
  permissions = [
    data.azuread_service_principal.graph_api.app_role_ids["Application.ReadWrite.OwnedBy"],
    data.azuread_service_principal.graph_api.app_role_ids["Group.Read.All"],
    data.azuread_service_principal.graph_api.app_role_ids["User.Read.All"],
  ]
}

# Configure Teleport as an OIDC IdP.
resource "azuread_application_federated_identity_credential" "app_oidc_idp" {
  count          = !var.use_system_credentials ? 1 : 0
  application_id = azuread_application.app.id
  display_name   = "${var.app_name}_oidc_idp"
  description    = "Teleport as an OIDC IdP"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://${var.proxy_service_address}"
  subject        = "teleport-azure"
}

# Assign API permissions. 
resource "azuread_application_api_access" "app_graph_permission" {
  count          = !var.use_system_credentials ? 1 : 0
  application_id = azuread_application.app.id
  api_client_id  = data.azuread_application_published_app_ids.well_known.result["MicrosoftGraph"]

  role_ids = local.permissions
}

# Grant admin consent.
resource "azuread_app_role_assignment" "app_grant_permission" {
  # Set permission only if use_system_credentials = true
  for_each = !var.use_system_credentials ? toset(local.permissions) : toset([])
  # ID of the API permission
  app_role_id = each.value
  # Object ID of the system assigned Managed identity
  principal_object_id = azuread_service_principal.app_sp.object_id
  # Object ID of the MS graph API application. 
  resource_object_id = data.azuread_service_principal.graph_api.object_id
}
