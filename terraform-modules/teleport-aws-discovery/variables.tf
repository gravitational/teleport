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

variable "create" {
  description = "Toggle resource creation."
  type        = bool
  default     = true
  nullable    = false
}

variable "match_aws_regions" {
  description = "AWS regions to discover. The default matches all AWS regions."
  type        = list(string)
  default     = ["*"]
  nullable    = false
}

variable "match_aws_tags" {
  description = "AWS resource tags to match when registering discovered resources with Teleport. The default matches all discovered AWS resources."
  type        = map(list(string))
  default     = { "*" : ["*"] }
  nullable    = false
}

variable "name_prefix" {
  description = "Prefix to include in resource names."
  type        = string
  default     = ""
  nullable    = false
}

variable "tags" {
  description = "Tags to apply to AWS resources."
  type        = map(string)
  default     = {}
  nullable    = false
}

variable "teleport_discovery_config_name" {
  description = "Teleport discovery config name to use instead of a generated name."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_discovery_service_iam_policy_name" {
  description = "Teleport discovery AWS IAM policy name to use instead of a generated name."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_discovery_service_iam_role_name" {
  description = "Teleport discovery AWS IAM role name to use instead of a generated name."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_integration_name" {
  description = "Teleport integration name to use instead of a generated name."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_provision_token_name" {
  description = "Teleport provisioning token name to use instead of a generated name."
  type        = string
  default     = ""
  nullable    = false
}

variable "teleport_resource_labels" {
  description = "Additional labels to apply to Teleport cluster resources."
  type        = map(string)
  default     = {}
  nullable    = false
}
