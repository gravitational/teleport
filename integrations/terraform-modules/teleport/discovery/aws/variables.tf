################################################################################
# Required variables
################################################################################

variable "teleport_proxy_public_addr" {
  description = "Teleport cluster proxy public address `host:port`."
  type        = string
  nullable    = false

  validation {
    condition     = !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme"
  }

  validation {
    condition     = var.teleport_proxy_public_addr == "" || strcontains(var.teleport_proxy_public_addr, ":")
    error_message = "The address must be in the form `host:port`."
  }
}

variable "teleport_discovery_group_name" {
  description = "Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use \"cloud-discovery-group\"."
  type        = string
  nullable    = false
}

variable "match_aws_resource_types" {
  description = "Deprecated legacy input. Use aws_matchers instead. AWS resource types to match when discovering resources with Teleport."
  type        = list(string)
  default     = []
  nullable    = false

  validation {
    condition = alltrue([
      for rt in var.match_aws_resource_types :
      contains([
        "ec2"
      ], rt)
    ])
    error_message = "Allowed values for match_aws_resource_types are: ec2. Use the new aws_matchers field instead for all supported types."
  }
}

variable "aws_matchers" {
  description = "AWS resource discovery matchers. Valid values for aws_matchers.types are: ec2, eks, rds."
  type = list(object({
    types                = list(string)
    regions              = optional(list(string), ["*"])
    tags                 = optional(map(list(string)), { "*" : ["*"] })
    setup_access_for_arn = optional(string, "")
    kube_app_discovery   = optional(bool)
  }))
  default  = []
  nullable = false

  validation {
    condition = alltrue([
      for matcher in var.aws_matchers :
      length(matcher.types) > 0 && length(matcher.regions) > 0
    ])
    error_message = "Each aws_matcher must have types and regions set."
  }

  validation {
    condition = alltrue(flatten([
      for matcher in var.aws_matchers : [
        for rt in matcher.types :
        contains([
          "ec2",
          "eks",
          "rds",
        ], rt)
      ]
    ]))
    error_message = "Allowed values for aws_matchers.types are: ec2, eks, rds."
  }

  validation {
    condition = !anytrue([
      for matcher in var.aws_matchers :
      matcher.setup_access_for_arn != "" && !contains(matcher.types, "eks")
    ])
    error_message = "setup_access_for_arn is only supported for EKS matchers."
  }
}

################################################################################
# Optional variables
################################################################################

variable "apply_aws_tags" {
  description = "Additional AWS tags to apply to all created AWS resources."
  type        = map(string)
  default     = {}
  nullable    = false
}

variable "apply_teleport_resource_labels" {
  description = "Additional Teleport resource labels to apply to all created Teleport resources."
  type        = map(string)
  default     = {}
  nullable    = false
}

variable "create" {
  description = "Toggle creation of all resources."
  type        = bool
  default     = true
  nullable    = false
}

variable "create_aws_iam_openid_connect_provider" {
  description = "Toggle AWS IAM OIDC provider creation. If false and using OIDC, then the AWS IAM OIDC provider must already exist."
  type        = bool
  default     = true
  nullable    = false
}

variable "aws_iam_policy_document" {
  description = "Override the AWS IAM policy document attached to the AWS IAM role for resource discovery."
  type        = string
  default     = ""
  nullable    = false
}

variable "match_aws_regions" {
  description = "Deprecated legacy input. Use aws_matchers instead. AWS regions to discover. The default matches all AWS regions."
  type        = list(string)
  default     = ["*"]
  nullable    = false
}

variable "match_aws_tags" {
  description = "Deprecated legacy input. Use aws_matchers instead. AWS resource tags to match when discovering resources with Teleport. The default matches all discovered AWS resources."
  type        = map(list(string))
  default     = { "*" : ["*"] }
  nullable    = false
}

variable "aws_iam_policy_name" {
  description = "Name for the AWS IAM policy for discovery."
  type        = string
  default     = "teleport-discovery"
  nullable    = false
}

variable "aws_iam_policy_use_name_prefix" {
  description = "Determines whether the name of the AWS IAM policy (`aws_iam_policy_name`) is used as a prefix."
  type        = bool
  default     = true
  nullable    = false
}

variable "aws_iam_role_name" {
  description = "Name for the AWS IAM role for discovery."
  type        = string
  default     = "teleport-discovery"
  nullable    = false
}

variable "aws_iam_role_use_name_prefix" {
  description = "Determines whether the name of the AWS IAM role (`aws_iam_role_name`) is used as a prefix."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_discovery_config_install_suffix" {
  description = "An optional installation suffix to use in the Teleport discovery_config. A suffix can be used to allow multiple Teleport installations on the same EC2 instance, which allows the instance to join multiple Teleport clusters. If specified, agent managed updates must be enabled on the cluster. See https://goteleport.com/docs/upgrading/agent-managed-updates/"
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_discovery_config_name" {
  description = "Name for the `teleport_discovery_config` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_discovery_config_use_name_prefix" {
  description = "Determines whether the name of the Teleport discovery config (`teleport_discovery_config_name`) is used as a prefix."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_integration_name" {
  description = "Name for the `teleport_integration` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_integration_use_name_prefix" {
  description = "Determines whether the name of the Teleport integration (`teleport_integration_name`) is used as a prefix."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_provision_token_name" {
  description = "Name for the `teleport_provision_token` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_provision_token_use_name_prefix" {
  description = "Determines whether the name of the Teleport provision token (`teleport_provision_token_name`) is used as a prefix."
  type        = bool
  default     = true
  nullable    = false
}

variable "discovery_service_iam_credential_source" {
  description = "Configure the AWS credential source for Teleport Discovery Service instances. The default uses AWS OIDC integration."
  type = object({
    use_oidc_integration = optional(bool, true)
    trust_role = optional(object({
      role_arn    = string
      external_id = optional(string, "")
    }))
  })
  default = {
    use_oidc_integration = true
    trust_role           = null
  }
  nullable = false

  validation {
    condition = !(
      var.discovery_service_iam_credential_source.use_oidc_integration
      && var.discovery_service_iam_credential_source.trust_role != null
    )
    error_message = "The discovery service AWS IAM credential source must be configured to assume the AWS IAM role for discovery either via OIDC integration or by assuming the role with an external ID, but not both."
  }

  validation {
    condition = !(
      !var.discovery_service_iam_credential_source.use_oidc_integration
      && try(var.discovery_service_iam_credential_source.trust_role.role_arn, "") == ""
    )
    error_message = "If the discovery service is to assume the discovery IAM role without OIDC (`use_oidc_integration` is set to false), then `trust_role.role_arn` must be set to a non-empty value."
  }
}

variable "aws_organization_iam_policies" {
  description = "AWS IAM policy customizations for organization-wide discovery to be created in AWS management account."
  type = object({
    account_enumeration = optional(object({
      name            = optional(string, "teleport-organization-account-enumeration")
      use_name_prefix = optional(bool, true)
      document        = optional(string, "")
    }), {})
    join_validation = optional(object({
      name            = optional(string, "teleport-organization-join-validation")
      use_name_prefix = optional(bool, true)
      document        = optional(string, "")
    }), {})
  })
  default  = {}
  nullable = false

  validation {
    condition = can(regex("^[\\w+=,.@-]{1,128}$", var.aws_organization_iam_policies.account_enumeration.name))
    # Regex can be found at:
    # https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html
    error_message = "Provide a valid AWS IAM Policy name for account_enumeration."
  }
  validation {
    condition = can(regex("^[\\w+=,.@-]{1,128}$", var.aws_organization_iam_policies.join_validation.name))
    # Regex can be found at:
    # https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html
    error_message = "Provide a valid AWS IAM Policy name for join_validation."
  }
}

variable "aws_organization_discovery" {
  description = <<EOT
Discover resources in accounts under the organization, filtered by Organizational Units (the Organization's Root ID or `*` can be used to include the entire organization).
A specific IAM role must be created in each child account, to be assumed by the Discovery Service. Check the module outputs for the trust relationship and permissions required for the role.
Limitations: only EC2 is supported.
EOT

  type = object({
    organizational_units = object({
      include = list(string)
      exclude = optional(list(string), [])
    })
  })
  default  = null
  nullable = true

  validation {
    condition     = var.aws_organization_discovery == null ? true : length(var.aws_organization_discovery.organizational_units.include) > 0
    error_message = "When enabling AWS Organization discovery, at least one organizational_units.include must be specified. You can use the organization's Root ID or the `*` to include the entire organization."
  }

  validation {
    condition = var.aws_organization_discovery == null ? true : (
      !contains(var.aws_organization_discovery.organizational_units.include, "*")
      || length(var.aws_organization_discovery.organizational_units.include) == 1
    )
    error_message = "`organizational_units.include` cannot mix `*` with specific Organizational Unit IDs; use `*` alone to include all OUs, or list OU IDs explicitly"
  }

  validation {
    condition = var.aws_organization_discovery == null ? true : alltrue([
      for ou in var.aws_organization_discovery.organizational_units.include :
      ou == "*" || can(regex("^(r-[a-z0-9]{4,32}|ou-[a-z0-9]{4,32}-[a-z0-9]{8,32})$", ou))
    ])
    # Regex can be found at:
    # https://docs.aws.amazon.com/organizations/latest/APIReference/API_Root.html
    # https://docs.aws.amazon.com/organizations/latest/APIReference/API_OrganizationalUnit.html
    error_message = "Included Organizational Units must each be `*`, the Organization Root ID (`r-xy`), or an Organizational Unit ID (`ou-xy-abcdefgh`)."
  }

  validation {
    condition = var.aws_organization_discovery == null ? true : alltrue([
      for ou in var.aws_organization_discovery.organizational_units.exclude :
      can(regex("^(r-[a-z0-9]{4,32}|ou-[a-z0-9]{4,32}-[a-z0-9]{8,32})$", ou))
    ])
    # Regex can be found at:
    # https://docs.aws.amazon.com/organizations/latest/APIReference/API_Root.html
    # https://docs.aws.amazon.com/organizations/latest/APIReference/API_OrganizationalUnit.html
    error_message = "Excluded Organizational Units must each be an Organization Root ID (`r-xy`) or an Organizational Unit ID (`ou-xy-abcdefgh`). Wildcards are not allowed in exclude."
  }
}

variable "aws_child_account_iam_role_name" {
  description = "Name for the AWS IAM role to assume in child accounts. This role must be created manually in each child account. Check the module outputs `aws_child_account_iam_role_template` for the trust relationship and permissions required."
  type        = string
  default     = "teleport-organization-discovery-child-account-role"
  nullable    = false
  validation {
    condition = can(regex("^[\\w+=,.@-]{1,64}$", var.aws_child_account_iam_role_name))
    # Regex can be found at:
    # https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html
    error_message = "Provide a valid AWS IAM Role name."
  }
}
