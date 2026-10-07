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
  managed_identity_role_scope  = local.create_managed_identity_role ? coalesce(var.managed_identity_role.scope, local.subscription_scope) : null
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
  count = local.create_managed_identity_role ? 1 : 0

  principal_id       = local.managed_identity.principal_id
  principal_type     = "ServicePrincipal"
  role_definition_id = one(azurerm_role_definition.teleport_agent[*].role_definition_resource_id)
  scope              = local.managed_identity_role_scope
}
