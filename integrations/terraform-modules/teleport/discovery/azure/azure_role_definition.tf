################################################################################
# Azure role for Teleport Discovery Service
################################################################################

locals {
  azure_role_assignable_scopes = coalescelist(
    var.azure_role_assignable_scopes,
    [local.scope.subscription],
  )
  azure_role_assignment_scopes = coalescelist(
    var.azure_role_assignment_scopes,
    [local.scope.subscription],
  )
}

# Custom role for discovery permissions
resource "azurerm_role_definition" "teleport_discovery" {
  count = local.create ? 1 : 0

  assignable_scopes = local.azure_role_assignable_scopes
  description       = "Azure role that allows a Teleport Discovery Service to discover VMs."
  name              = var.azure_role_definition_name
  scope             = local.scope.subscription

  permissions {
    actions = [
      "Microsoft.Compute/virtualMachines/read",
      "Microsoft.Compute/virtualMachines/runCommand/action",
      "Microsoft.Compute/virtualMachines/runCommands/delete",
      "Microsoft.Compute/virtualMachines/runCommands/read",
      "Microsoft.Compute/virtualMachines/runCommands/write",
      "Microsoft.Resources/subscriptions/read",
    ]
    not_actions = []
  }
}

# Assign the custom role to the managed identity principal
resource "azurerm_role_assignment" "teleport_discovery" {
  for_each = local.create ? toset(local.azure_role_assignment_scopes) : []

  principal_id       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].principal_id)
  role_definition_id = one(azurerm_role_definition.teleport_discovery[*].role_definition_resource_id)
  scope              = each.value
}
