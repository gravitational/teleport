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
  description = "AWS resource types to match when discovering resources with Teleport. Valid values are: `ec2`."
  type        = list(string)
  nullable    = false

  validation {
    condition = alltrue([
      for rt in var.match_aws_resource_types :
      contains([
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
      ], rt)
    ])
    error_message = format(
      "Allowed values for match_aws_resource_types are: %s.",
      join(", ", [
        "ec2",
      ])
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
    use_oidc_integration = optional(bool)
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
    error_message = "The discovery service AWS IAM credential source must be configured to assume the AWS IAM role for discovery either via OIDC integration or by assuming the role with an external ID. If the AWS IAM role for discovery will be attached directly to the discovery service instance outside of this module, then set `use_oidc_integration` to false and leave `trust_role` unset."
  }

  validation {
    condition = !(
      !var.discovery_service_iam_credential_source.use_oidc_integration
      && try(var.discovery_service_iam_credential_source.trust_role.role_arn == "", false)
    )
    error_message = "If the discovery service is to assume the discovery IAM role without OIDC (`use_oidc_integration` is set to false), then `trust_role.role_arn` must be set to a non-empty value."
  }
}

variable "aws_iam_role_name_for_child_accounts" {
  description = "Name for the AWS IAM role to assume in child accounts, when using organization-wide discovery."
  type        = string
  default     = "teleport-discovery-from-organization"
  nullable    = false
}

variable "enroll_organization_accounts" {
  description = "Discover resources in all the AWS accounts under the organization."
  type        = bool
  default     = false
  nullable    = false
}
