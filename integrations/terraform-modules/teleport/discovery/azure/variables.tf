################################################################################
# Required variables
################################################################################

variable "azure_managed_identity_location" {
  type        = string
  description = "Azure region (location) where the managed identity will be created (e.g., \"eastus\"). Required when `create_azure_managed_identity` is `true`."
  default     = null
  nullable    = true
}

variable "azure_resource_group_name" {
  type        = string
  description = "Name of an existing Azure Resource Group where Azure resources will be created. Required when `create_azure_managed_identity` is `true`."
  default     = null
  nullable    = true
}

variable "teleport_discovery_group_name" {
  description = "Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use \"cloud-discovery-group\"."
  type        = string
  nullable    = false
}

variable "teleport_proxy_public_addr" {
  description = "Teleport cluster proxy public address in the form `host:port` (no URL scheme)."
  type        = string
  nullable    = false

  validation {
    condition     = !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme."
  }

  validation {
    condition     = strcontains(var.teleport_proxy_public_addr, ":")
    error_message = "The address must be in the form <host:port>."
  }
}

variable "azure_matchers" {
  description = "Azure resource discovery matchers. Valid values for azure_matchers.types are: vm."
  type = list(object({
    types           = list(string)
    subscriptions   = list(string)
    resource_groups = optional(list(string), ["*"])
    regions         = optional(list(string), ["*"])
    tags            = optional(map(list(string)), { "*" : ["*"] })
  }))
  nullable = false

  validation {
    condition     = length(var.azure_matchers) > 0
    error_message = "Must have at least one azure_matcher."
  }

  validation {
    condition = alltrue([
      for matcher in var.azure_matchers :
      length(matcher.types) > 0
    ])
    error_message = "Must have at least one type."
  }

  validation {
    condition = alltrue([
      for matcher in var.azure_matchers :
      length(matcher.subscriptions) > 0
    ])
    error_message = "Must have at least one subscription."
  }

  validation {
    condition = alltrue([
      for matcher in var.azure_matchers :
      !contains(matcher.subscriptions, "*") || length(matcher.subscriptions) == 1
    ])
    error_message = "Wildcard ('*') must be the only entry in a matcher's subscriptions list."
  }

  validation {
    condition = alltrue(flatten([
      for matcher in var.azure_matchers : [
        for rt in matcher.types :
        contains([
          # TODO: add module support for all resource types
          "vm",
          # "aks",
          # "mysql",
          # "postgres",
          # "redis",
          # "sqlserver",
        ], rt)
      ]
    ]))
    error_message = format(
      "Allowed values for azure_matchers.types are: %s.",
      join(", ", [
        "vm",
      ])
    )
  }
}

################################################################################
# Optional variables
################################################################################

variable "apply_azure_tags" {
  type        = map(string)
  description = "Additional Azure tags to apply to all created Azure resources."
  default     = {}
  nullable    = false
}

variable "apply_teleport_resource_labels" {
  description = "Additional Teleport resource labels to apply to all created Teleport resources."
  type        = map(string)
  default     = {}
  nullable    = false
}

variable "azure_federated_identity_credential_name" {
  description = "Name of the Azure federated identity credential created for workload identity federation."
  type        = string
  default     = "teleport-federation"
  nullable    = false
}

variable "azure_managed_identity_name" {
  description = "Name of the Azure user-assigned managed identity created for Teleport Discovery."
  type        = string
  default     = "discovery-identity"
  nullable    = false
}

variable "azure_managed_identity_use_name_prefix" {
  description = "Whether `azure_managed_identity_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "azure_role_assignment_scopes" {
  default     = []
  description = "The scopes at which the Azure discovery role will be assigned. For wildcard ('*') Azure subscription discovery, a management group scope can be used (e.g. `/providers/Microsoft.Management/managementGroups/<name>`). By default, scopes are derived from the subscriptions configured in `azure_matchers`."
  nullable    = false
  type        = list(string)
}

variable "azure_role_definition_name" {
  description = "Name for the Azure custom role definition created for Teleport Discovery."
  type        = string
  default     = "teleport-discovery"
  nullable    = false
}

variable "azure_role_definition_use_name_prefix" {
  description = "Whether `azure_role_definition_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "create" {
  description = "Toggle creation of all resources."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_discovery_config_name" {
  description = "Name for the `teleport_discovery_config` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_discovery_config_use_name_prefix" {
  description = "Whether `teleport_discovery_config_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_installer_script_name" {
  type        = string
  description = "Name of an existing Teleport installer script to use."
  default     = "default-installer"
  nullable    = false
}

variable "teleport_integration_name" {
  description = "Name for the `teleport_integration` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "use_oidc_integration" {
  description = "Whether an Azure OIDC integration and federated identity credential are created and referenced by the Teleport discovery config (true) or not (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "create_azure_managed_identity" {
  description = "Whether Azure managed identity and role resources are created (true) or not (false). When false, no Azure resources are created. Must be set to `true` when `use_oidc_integration` is `true`."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_integration_use_name_prefix" {
  description = "Whether `teleport_integration_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_provision_token_allow_rules" {
  description = "Custom allow rules for the Teleport provision token. Required when using a wildcard (`*`) subscription matcher."
  type = list(object({
    subscription    = optional(string)
    resource_groups = optional(list(string))
    tenant          = optional(string)
  }))
  default  = null
  nullable = true
}

variable "teleport_provision_token_name" {
  description = "Name for the `teleport_provision_token` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_provision_token_use_name_prefix" {
  description = "Whether `teleport_provision_token_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}
