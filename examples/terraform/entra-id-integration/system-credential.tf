# Grant Microsoft Graph API permissions to the managed identity.
resource "azuread_app_role_assignment" "managed_id_permissions" {
  # Set permission only if use_system_credentials = true
  for_each = var.use_system_credentials ? toset(var.graph_permission_ids) : toset([])
  # ID of the API permission
  app_role_id = each.value
  # Object ID of the system assigned Managed identity
  principal_object_id = var.managed_id
  # Object ID of the MS graph API application. 
  resource_object_id = data.azuread_service_principal.graph_api.object_id
}

