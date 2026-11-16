locals {
  kubernetes_service_permissions = {
    actions = [
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
    ]
    data_actions = [
      "Microsoft.ContainerService/managedClusters/groups/impersonate/action",
      "Microsoft.ContainerService/managedClusters/users/impersonate/action",
      "Microsoft.ContainerService/managedClusters/serviceaccounts/impersonate/action",
      "Microsoft.ContainerService/managedClusters/pods/read",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectaccessreviews/write",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectrulesreviews/write",
    ]
  }

  # Kubernetes is currently the only supported service.
  # When adding another service, ensure the permissions are correctly defined.
  default_role_permissions = local.kubernetes_service_permissions

  create_managed_identity_role = var.create && var.managed_identity_role != null

  azure_management_group_scope = (
    var.azure_management_group_id != null
    ? "/providers/Microsoft.Management/managementGroups/${var.azure_management_group_id}"
    : null
  )

  managed_identity_role_scope_override = try(var.managed_identity_role.scope, null)

  # Determine the scope for the managed identity role, prioritizing the override, then the explicit management group, and finally the current client subscription.
  managed_identity_role_scope = local.create_managed_identity_role ? coalesce(local.managed_identity_role_scope_override, local.azure_management_group_scope, local.subscription_scope) : null

  # When using DataActions permissions in roles (which the Kubernetes Service requires), Azure requires an explicit list of subscription scopes.
  # Re-apply this module to pick up new subscriptions.
  managed_identity_role_assignment_scopes = local.create_managed_identity_role ? (
    length(data.azurerm_management_group.agent) == 0
    ? { default = local.managed_identity_role_scope }
    : { for sub in data.azurerm_management_group.agent[0].all_subscription_ids : "/subscriptions/${sub}" => "/subscriptions/${sub}" }
  ) : {}
}

resource "azurerm_role_definition" "teleport_agent" {
  count = local.create_managed_identity_role ? 1 : 0

  assignable_scopes = [local.managed_identity_role_scope]
  description       = "Azure role that will be assigned to the managed identity used by the Teleport agent."
  name              = local.container_group_name

  scope = local.managed_identity_role_scope
  permissions {
    actions      = coalesce(var.managed_identity_role.actions, local.default_role_permissions.actions)
    data_actions = coalesce(var.managed_identity_role.data_actions, local.default_role_permissions.data_actions)
  }
}

resource "azurerm_role_assignment" "teleport_agent" {
  for_each = local.managed_identity_role_assignment_scopes

  principal_id   = local.managed_identity.principal_id
  principal_type = "ServicePrincipal"
  role_definition_id = (
    each.key == "default"
    ? one(azurerm_role_definition.teleport_agent[*].role_definition_resource_id)
    : "${each.value}/providers/Microsoft.Authorization/roleDefinitions/${one(azurerm_role_definition.teleport_agent[*].role_definition_id)}"
  )
  scope = each.value
}
