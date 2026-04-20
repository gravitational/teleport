################################################################################
# Teleport cluster provision token for Azure
################################################################################

locals {
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? "${var.teleport_provision_token_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_provision_token_name
  )

  token_allow_rules = flatten([
    for matcher in var.azure_matchers : [
      for sub in matcher.subscriptions : merge(
        { subscription = sub },
        contains(matcher.resource_groups, "*") ? {} : { resource_groups = matcher.resource_groups }
      )
    ]
  ])
}

# Teleport provision token for Azure join
resource "teleport_provision_token" "azure" {
  count = local.create ? 1 : 0

  metadata = {
    description = "Allow Teleport nodes to join the cluster using Azure credentials."
    labels      = local.apply_teleport_resource_labels
    name        = local.teleport_provision_token_name
  }
  spec = {
    azure = {
      allow = local.token_allow_rules
    }
    join_method = "azure"
    roles       = ["Node"]
  }
  version = "v2"
}
