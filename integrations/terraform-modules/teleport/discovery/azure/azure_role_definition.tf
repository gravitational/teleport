################################################################################
# Azure role for Teleport Discovery Service
################################################################################

locals {
  azure_role_assignment_scopes = coalescelist(
    var.azure_role_assignment_scopes,
    [for sub in local.azure_matcher_subscriptions : "/subscriptions/${sub}"],
  )

  uses_vm = contains(local.azure_matcher_types, "vm")

  vm_actions = [
    "Microsoft.Compute/virtualMachines/read",
    "Microsoft.Compute/virtualMachines/runCommand/action",
    "Microsoft.Compute/virtualMachines/runCommands/delete",
    "Microsoft.Compute/virtualMachines/runCommands/read",
    "Microsoft.Compute/virtualMachines/runCommands/write",
  ]

  role_actions = distinct(concat(
    ["Microsoft.Resources/subscriptions/read"],
    local.uses_vm ? local.vm_actions : [],
  ))
}

# Custom role for discovery permissions for each scope.
resource "azurerm_role_definition" "teleport_discovery" {
  for_each = local.create ? toset(local.azure_role_assignment_scopes) : toset([])

  assignable_scopes = [each.value]
  description       = "Azure role that allows a Teleport Discovery Service to discover VMs."
  # Split each scope by '/' and hyphenate the last two segments
  # e.g. "subscriptions-{uuid}", "managementGroups-{name}".
  name  = "${var.azure_role_definition_name}-${join("-", slice(split("/", each.value), length(split("/", each.value)) - 2, length(split("/", each.value))))}"
  scope = each.value

  permissions {
    actions     = local.role_actions
    not_actions = []
  }
}

# Assign the custom roles to the managed identity principal for each scope.
resource "azurerm_role_assignment" "teleport_discovery" {
  for_each = local.create ? toset(local.azure_role_assignment_scopes) : toset([])

  principal_id       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].principal_id)
  role_definition_id = azurerm_role_definition.teleport_discovery[each.key].role_definition_resource_id
  scope              = each.value
}
