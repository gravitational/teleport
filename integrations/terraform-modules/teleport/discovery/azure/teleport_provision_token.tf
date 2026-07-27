################################################################################
# Teleport cluster provision token for Azure
################################################################################

locals {
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? join("-", compact([var.teleport_provision_token_name, local.teleport_resource_name_suffix]))
    : var.teleport_provision_token_name
  )

  has_wildcard_subscription_matcher = anytrue([
    for matcher in var.azure_matchers :
    contains(matcher.subscriptions, "*")
  ])

  wildcard_subscription_allow_rules = local.create_azure_managed_identity ? [
    for matcher in var.azure_matchers : {
      subscription    = null
      resource_groups = contains(matcher.resource_groups, "*") ? null : matcher.resource_groups
      tenant          = local.azure_tenant_id
    } if contains(matcher.subscriptions, "*")
  ] : []

  subscription_allow_rules = flatten([
    for matcher in var.azure_matchers : [
      for sub in matcher.subscriptions : {
        subscription    = sub
        resource_groups = contains(matcher.resource_groups, "*") ? null : matcher.resource_groups
        tenant          = null
      }
    ] if !contains(matcher.subscriptions, "*")
  ])

  token_allow_rules = (
    var.teleport_provision_token_allow_rules != null
    ? var.teleport_provision_token_allow_rules
    : concat(local.wildcard_subscription_allow_rules, local.subscription_allow_rules)
  )
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

  lifecycle {
    precondition {
      condition     = !(local.has_wildcard_subscription_matcher && !local.create_azure_managed_identity && var.teleport_provision_token_allow_rules == null)
      error_message = "Wildcard ('*') subscription discovery with an external managed identity (create_azure_managed_identity=false) requires teleport_provision_token_allow_rules to be set."
    }
  }
}
