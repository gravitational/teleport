################################################################################
# Teleport cluster provision token for Azure
################################################################################

locals {
  create_teleport_provision_token = local.create && var.create_teleport_provision_token

  teleport_provision_token_name = (
    var.create_teleport_provision_token && var.teleport_provision_token_use_name_prefix
    ? "${var.teleport_provision_token_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_provision_token_name
  )

  has_wildcard_subscription_matcher = anytrue([
    for matcher in var.azure_matchers :
    contains(matcher.subscriptions, "*")
  ])

  token_allow_rules_from_matchers = flatten([
    for matcher in var.azure_matchers : [
      for sub in matcher.subscriptions : merge(
        { subscription = sub },
        contains(matcher.resource_groups, "*") ? {} : { resource_groups = matcher.resource_groups }
      )
    ]
  ])

  token_allow_rules = (
    var.teleport_provision_token_allow_rules != null
    ? var.teleport_provision_token_allow_rules
    : local.token_allow_rules_from_matchers
  )
}

# Teleport provision token for Azure join
resource "teleport_provision_token" "azure" {
  count = local.create_teleport_provision_token ? 1 : 0

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

  lifecycle {
    precondition {
      condition     = !(local.has_wildcard_subscription_matcher && var.teleport_provision_token_allow_rules == null)
      error_message = "Wildcard ('*') subscription discovery requires teleport_provision_token_allow_rules to be set."
    }
  }
}
