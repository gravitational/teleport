################################################################################
# Required variables
################################################################################

variable "teleport_proxy_public_addr" {
  description = "Teleport cluster proxy public address."
  type        = string
  nullable    = false

  validation {
    condition     = var.teleport_proxy_public_addr != ""
    error_message = "Must not be empty."
  }

  validation {
    condition     = !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme"
  }
}

variable "teleport_discovery_group_name" {
  description = "Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use \"cloud-discovery-group\"."
  type        = string
  nullable    = false

  validation {
    condition     = var.teleport_discovery_group_name != ""
    error_message = "Must not be empty."
  }
}

locals {
  allowed_match_aws_resource_types = [
    # TODO(gavin): add module support for all resource types
    # "docdb",
    "ec2",
    # "eks",
    # "elasticache-serverless",
    # "elasticache",
    # "memorydb",
    # "opensearch",
    # "rds",
    # "rdsproxy",
    # "redshift-serverless",
    # "redshift"
  ]
}

variable "match_aws_resource_types" {
  description = "AWS resource types to match when discovering resources with Teleport."
  type        = list(string)
  nullable    = false

  validation {
    condition = alltrue([
      for rt in var.match_aws_resource_types :
      contains(local.allowed_match_aws_resource_types, rt)
    ])
    error_message = format(
      "Allowed values for match_aws_resource_types are: %s.",
      join(", ", local.allowed_match_aws_resource_types)
    )
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
  description = "Additional Teleport resource labels to apply to all created Teleport."
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

  validation {
    condition     = !(var.create && var.create_aws_iam_openid_connect_provider && !var.create_aws_iam_role)
    error_message = "If the AWS IAM OIDC provider will be created, then the AWS IAM role for discovery must also be created so that the OIDC provider is included in the role's trust policy."
  }
}

variable "create_aws_iam_policy_attachment" {
  description = "Toggle AWS IAM policy attachment to the Discovery Service AWS IAM role. If false, then the AWS IAM policy must already be attached."
  type        = bool
  default     = true
  nullable    = false
}

variable "create_aws_iam_policy" {
  description = "Toggle AWS IAM policy creation. If false, then the IAM policy for discovery must already exist."
  type        = bool
  default     = true
  nullable    = false

  validation {
    condition     = !(var.create && var.create_aws_iam_policy && !var.create_aws_iam_policy_attachment)
    error_message = "If the AWS IAM policy for discovery will be created, then it must also be attached to the AWS IAM role for discovery."
  }
}

variable "create_aws_iam_role" {
  description = "Toggle creation of the AWS IAM role for Teleport Discovery Service. If false, then the IAM role must already exist."
  type        = bool
  default     = true
  nullable    = false

  validation {
    condition     = !(var.create && var.create_aws_iam_role && !var.create_aws_iam_policy_attachment)
    error_message = "If the AWS IAM role for discovery will be created, then the AWS IAM policy for discovery must also be attached to it."
  }
}

variable "match_aws_regions" {
  description = "AWS regions to discover. The default matches all AWS regions."
  type        = list(string)
  default     = ["*"]
  nullable    = false
}

variable "match_aws_tags" {
  description = "AWS resource tags to match when discovering resources with Teleport. The default matches all discovered AWS resources."
  type        = map(list(string))
  default     = { "*" : ["*"] }
  nullable    = false
}

variable "name_prefix" {
  description = "Prefix to include in resource names. This prefix is also added to any resource name overrides."
  type        = string
  default     = ""
  nullable    = false
}

variable "aws_iam_policy_name" {
  description = "Optional name override for the AWS IAM policy for discovery."
  type        = string
  default     = ""
  nullable    = false
}

variable "aws_iam_role_name" {
  description = "Optional name override for the AWS IAM role for discovery."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_discovery_config_name" {
  description = "Optional name override for the `teleport_discovery_config` resource."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_integration_name" {
  description = "Optional name override for the `teleport_integration` resource."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_provision_token_name" {
  description = "Optional name override for the `teleport_provision_token` resource."
  type        = string
  default     = ""
  nullable    = false
}

variable "discovery_service_iam_credential_source" {
  description = "Configure the intended credential source for Teleport Discovery Service instances. The default uses AWS OIDC integration."
  type = object({
    use_oidc_integration = optional(bool, true) # the default
    trust_role = optional(object({
      role_arn    = string
      external_id = string
    }))
  })
  default  = {}
  nullable = false

  validation {
    condition = !(
      var.create
      && var.discovery_service_iam_credential_source.use_oidc_integration
      && var.discovery_service_iam_credential_source.trust_role != null
    )
    error_message = "The discovery service AWS IAM credential source must be configured to assume the AWS IAM role for discovery either via OIDC integration or by assuming the role with an external ID. If the AWS IAM role for discovery will be attached directly to the discovery service instance outside of this module, then set `use_oidc_integration` to false and leave `trust_role` unset."
  }

  validation {
    condition = !(
      var.create
      && !var.discovery_service_iam_credential_source.use_oidc_integration
      && var.discovery_service_iam_credential_source.trust_role != null
      && (
        var.discovery_service_iam_credential_source.trust_role.role_arn == ""
        || var.discovery_service_iam_credential_source.trust_role.external_id == ""
      )
    )
    error_message = "`trust_role` must include non-empty values for both the trusted role's ARN and an external ID to use in the trust policy of the AWS IAM role for discovery."
  }
}
