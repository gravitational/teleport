################################################################################
# Teleport cluster discovery config
################################################################################

locals {
  teleport_discovery_config_name = (
    var.teleport_discovery_config_use_name_prefix
    ? join("-", compact([var.teleport_discovery_config_name, local.teleport_resource_name_suffix]))
    : var.teleport_discovery_config_name
  )

  vm_install_params = {
    join_method = "azure"
    join_token  = local.teleport_provision_token_name
    script_name = var.teleport_installer_script_name
  }

  azure_matchers = [
    for matcher in var.azure_matchers : merge(
      {
        types           = matcher.types
        subscriptions   = contains(matcher.subscriptions, "*") ? ["*"] : compact(matcher.subscriptions)
        resource_groups = compact(matcher.resource_groups)
        regions         = matcher.regions
        tags            = matcher.tags
      },
      local.use_oidc_integration ? { integration = local.teleport_integration_name } : {},
      contains(matcher.types, "vm") ? { install_params = local.vm_install_params } : {}
    )
  ]

  azure_matcher_types   = distinct(flatten([for matcher in local.azure_matchers : matcher.types]))
  azure_matcher_regions = distinct(flatten([for matcher in local.azure_matchers : matcher.regions]))
  azure_matcher_subscriptions = (
    local.has_wildcard_subscription_matcher
    ? ["*"]
    : distinct(flatten([for matcher in local.azure_matchers : matcher.subscriptions]))
  )
}

# Teleport discovery config
resource "teleport_discovery_config" "azure" {
  count = local.create ? 1 : 0

  header = {
    version = "v1"
    metadata = {
      description = "Configure Teleport to discover Azure resources."
      labels      = local.apply_teleport_resource_labels
      name        = local.teleport_discovery_config_name
    }
  }

  lifecycle {
    precondition {
      condition     = !local.create_azure_managed_identity || !local.has_wildcard_subscription_matcher || length(var.azure_role_assignment_scopes) > 0
      error_message = "Wildcard ('*') subscription discovery requires azure_role_assignment_scopes to be set, preferably with a management group scope, e.g. /providers/Microsoft.Management/managementGroups/<name>."
    }
  }

  spec = {
    discovery_group = var.teleport_discovery_group_name
    azure           = local.azure_matchers
  }

  depends_on = [
    # Don't create the discovery config until the integration is in place.
    # This should avoid a ~5 minute delay that can happen if the discovery service tries to run before it has permissions.
    teleport_integration.azure_oidc,
  ]
}
