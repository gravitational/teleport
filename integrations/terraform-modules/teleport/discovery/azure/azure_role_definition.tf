################################################################################
# Azure role for Teleport Discovery Service
################################################################################

locals {
  azure_management_group_scope = (
    var.azure_management_group_id != null
    ? (startswith(var.azure_management_group_id, "/providers/Microsoft.Management/managementGroups/")
      ? var.azure_management_group_id
      : "/providers/Microsoft.Management/managementGroups/${var.azure_management_group_id}")
    : null
  )

  azure_role_assignment_scopes = coalescelist(
    var.azure_role_assignment_scopes,
    local.azure_management_group_scope != null
    ? [local.azure_management_group_scope]
    : [],
    [for sub in local.azure_matcher_subscriptions : "/subscriptions/${sub}"],
  )

  uses_vm = contains(local.azure_matcher_types, "vm")

  vm_actions = [
    "Microsoft.Compute/virtualMachines/read",
    "Microsoft.Compute/virtualMachines/runCommands/read",
    "Microsoft.Compute/virtualMachines/runCommands/write",
    "Microsoft.Compute/virtualMachineScaleSets/read",
    "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
    "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/read",
    "Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/write",
  ]

  role_actions = distinct(concat(
    ["Microsoft.Resources/subscriptions/read"],
    local.uses_vm ? local.vm_actions : [],
  ))

  azure_role_definition_name = (
    var.azure_role_definition_use_name_prefix
    ? join("-", compact([var.azure_role_definition_name, local.teleport_resource_name_suffix]))
    : var.azure_role_definition_name
  )
}

# Custom role for Teleport Discovery Service permissions.
resource "azurerm_role_definition" "teleport_discovery" {
  count = local.create_azure_managed_identity ? 1 : 0

  assignable_scopes = local.azure_role_assignment_scopes
  description       = "Azure role that allows a Teleport Discovery Service to discover VMs."
  name              = local.azure_role_definition_name
  scope             = coalesce(local.azure_management_group_scope, local.azure_role_assignment_scopes[0])

  permissions {
    actions     = local.role_actions
    not_actions = []
  }

  lifecycle {
    precondition {
      condition = (
        length([
          for s in var.azure_role_assignment_scopes :
          s if startswith(s, "/providers/Microsoft.Management/managementGroups/")
        ]) <= 1
      ) || var.azure_management_group_id != null
      error_message = "`azure_management_group_id` is required when `azure_role_assignment_scopes` contains more than one management group scope."
    }
  }
}

# Assign the custom role to the managed identity for each scope.
resource "azurerm_role_assignment" "teleport_discovery" {
  for_each = local.create_azure_managed_identity ? toset(local.azure_role_assignment_scopes) : toset([])

  principal_id       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].principal_id)
  role_definition_id = one(azurerm_role_definition.teleport_discovery[*].role_definition_resource_id)
  scope              = each.value
}
