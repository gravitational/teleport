################################################################################
# Azure role for Teleport Discovery Service
################################################################################

# Custom role for discovery permissions
resource "azurerm_role_definition" "teleport_discovery" {
  count = local.create ? 1 : 0

  assignable_scopes = ["/subscriptions/${local.azure_subscription_id}"]
  description       = "Azure role that allows a Teleport Discovery Service to discover VMs."
  name              = var.azure_role_definition_name
  scope             = "/subscriptions/${local.azure_subscription_id}"

  permissions {
    actions = [
      "Microsoft.Compute/virtualMachines/read",
      "Microsoft.Compute/virtualMachines/runCommand/action",
      "Microsoft.Compute/virtualMachines/runCommands/write",
      "Microsoft.Compute/virtualMachines/runCommands/read",
      "Microsoft.Compute/virtualMachines/runCommands/delete",
    ]
    not_actions = []
  }
}

# Assign the custom role to the managed identity principal
resource "azurerm_role_assignment" "teleport_discovery" {
  count = local.create ? 1 : 0

  principal_id       = one(azurerm_user_assigned_identity.teleport_discovery_service[*].principal_id)
  role_definition_id = one(azurerm_role_definition.teleport_discovery[*].role_definition_resource_id)
  scope              = "/subscriptions/${local.azure_subscription_id}"
}
