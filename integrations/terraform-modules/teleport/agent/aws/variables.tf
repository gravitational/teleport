################################################################################
# Required variables
################################################################################

variable "vpc_id" {
  description = "VPC ID where Teleport will be deployed."
  type        = string
}

################################################################################
# Optional variables
################################################################################

variable "apply_aws_tags" {
  default     = {}
  description = "Additional AWS tags to apply to all created AWS resources."
  type        = map(string)
}

variable "create" {
  default     = true
  description = "Toggle creation of all resources."
  type        = bool
}

variable "create_security_group" {
  default     = true
  description = "Whether to create the Teleport instance outbound security group and the shared database-access security group."
  type        = bool
}

variable "database_service" {
  default     = {}
  description = <<EOF
Teleport Database Service configuration.
Set enabled to true to run the Teleport Database Service.
Set allow_database_modification to true to add RDS permissions that allow the service to modify database configuration.
Set default_iam_policy_database_types to the database types for which default IAM policy statements should be added to the ECS task role's inline policy.
Supported default IAM policy database types are `rds` and `rdsproxy`.
Statements in the default IAM policy can be overridden by a statement with a matching SID in var.ecs_task_role_inline_policy.
When resources is null, a default resource matcher is used that matches databases in the same account, region, and VPC.
Set database_security_group_ids to database security groups that should allow traffic from the Teleport Database Service.
Setting database_security_group_ids requires create_security_group to be true.
When ecs_task_role_self_assumption_allowed is true, each matcher's aws.assume_role_arn defaults to the ECS task role ARN, including custom matchers.
Set aws.assume_role_arn to another role ARN to override this default, or to an empty string to disable role assumption for that matcher.
EOF
  nullable    = false
  type = object({
    allow_database_modification       = optional(bool, false)
    database_security_group_ids       = optional(list(string), [])
    default_iam_policy_database_types = optional(list(string), [])
    enabled                           = optional(bool, false)
    resources = optional(list(object({
      labels = map(list(string))
      aws = optional(object({
        assume_role_arn = optional(string)
        external_id     = optional(string, "")
      }))
    })), null)
  })

  validation {
    condition = alltrue([
      for database_type in var.database_service.default_iam_policy_database_types
      : contains(["rds", "rdsproxy"], database_type)
    ])
    error_message = "Supported database types are: rds, rdsproxy."
  }

  validation {
    condition     = length(var.database_service.database_security_group_ids) == length(distinct(var.database_service.database_security_group_ids))
    error_message = "database_service.database_security_group_ids must not contain duplicate security group IDs."
  }
}

variable "ecs_cluster_name" {
  default     = "teleport"
  description = "Name of the ECS cluster."
  type        = string
}

variable "ecs_cluster_use_name_prefix" {
  default     = true
  description = "Determines whether `var.ecs_cluster_name` is used as a prefix of the ECS cluster name."
  type        = bool
}

variable "ecs_execution_role_inline_policy" {
  default     = null
  description = "Optional JSON policy document to merge into the inline policy attached to the ECS execution IAM role."
  nullable    = true
  type        = string
}

variable "ecs_service_name" {
  default     = "teleport"
  description = "Name of the ECS service."
  type        = string
}

variable "ecs_task_cloudwatch_log_group_kms_key_id" {
  default     = null
  description = <<EOF
KMS key ID or ARN used to encrypt the ECS task CloudWatch log group.
When null, CloudWatch Logs uses its default encryption key.
EOF
  nullable    = true
  type        = string
}

variable "ecs_task_cloudwatch_log_group_name" {
  default     = "ecs-teleport"
  description = "Name for the ECS task CloudWatch log group."
  type        = string
}

variable "ecs_task_cloudwatch_log_group_region" {
  default     = null
  description = <<EOF
AWS Region for the ECS task CloudWatch log group.
Defaults to var.region, then the AWS provider's configured Region.
EOF
  nullable    = true
  type        = string
}

variable "ecs_task_cloudwatch_log_group_retention_days" {
  default     = 30
  description = "Number of days to retain logs in the ECS task CloudWatch log group."
  type        = number
}

variable "ecs_task_cloudwatch_log_group_skip_destroy" {
  default     = false
  description = <<EOF
Whether to preserve the ECS task CloudWatch log group when destroying module resources.
Set to true if you do not wish the log group (and any logs it may contain) to be deleted at destroy time, and instead just remove the log group from the Terraform state.
EOF
  type        = bool
}

variable "ecs_task_cloudwatch_log_group_use_name_prefix" {
  default     = true
  description = "Determines whether `var.ecs_task_cloudwatch_log_group_name` is used as a prefix of the ECS task CloudWatch log group name."
  type        = bool
}

variable "ecs_task_cpu" {
  default     = 2048
  description = "Number of cpu units used by the ECS task."
  type        = number
}

variable "ecs_task_definition_name" {
  default     = "teleport"
  description = "Name of the ECS task."
  type        = string
}

variable "ecs_task_definition_use_name_prefix" {
  default     = true
  description = "Determines whether `var.ecs_task_definition_name` is used as a prefix of the ECS task definition name."
  type        = bool
}

variable "ecs_task_desired_count" {
  default     = 2
  description = "Desired number of Teleport ECS tasks to run."
  type        = number
}

variable "ecs_task_force_new_deployment" {
  default     = false
  description = "Set to true to force the ECS service to redeploy tasks without configuration changes."
  type        = bool
}

variable "ecs_task_memory" {
  default     = 4096
  description = "Amount (in MiB) of memory used by the ECS task."
  type        = number
}

variable "ecs_task_role_inline_policy" {
  default     = null
  description = "Optional JSON policy document to merge into the inline policy attached to the ECS task IAM role."
  nullable    = true
  type        = string
}

variable "ecs_task_role_self_assumption_allowed" {
  default     = true
  description = "Whether the ECS task IAM role can assume itself."
  type        = bool
}

variable "environment_vars" {
  default     = {}
  description = "Environment variables to set on the Teleport ECS container."
  type        = map(string)
}

variable "join_params" {
  default     = null
  description = <<EOF
Override the Teleport join parameters.
When null and var.database_service.enabled is true, the module creates an IAM join token automatically.
Set this to use a pre-existing token or a different join method.
EOF
  type = object({
    token_name = string
    method     = string
  })
}

variable "log_level" {
  default     = "INFO"
  description = <<EOF
Teleport log level.
Only used when var.database_service.enabled is true.
EOF
  type        = string
}

variable "managed_updates_enabled" {
  default     = true
  description = "Whether to resolve the Teleport version from the configured Managed Updates endpoint when applying this module."
  type        = bool
}

variable "managed_updates_group" {
  default     = "default"
  description = "Update group to query through the v2 Managed Updates endpoint."
  type        = string
}

variable "private_subnet_ids" {
  default     = null
  description = <<EOF
Private subnet IDs where Teleport can be deployed.
Private subnets must have a default route through an AWS NAT gateway.
Set this or public_subnet_ids to bypass automatic subnet discovery, such as when the VPC is created in the same Terraform apply.
When both variables are null, the module discovers and classifies all subnets in vpc_id from their effective route tables.
EOF
  type        = list(string)
}

variable "public_subnet_ids" {
  default     = null
  description = <<EOF
Public subnet IDs where Teleport can be deployed.
Public subnets must have a default route through an AWS internet gateway.
Set this or private_subnet_ids to bypass automatic subnet discovery, such as when the VPC is created in the same Terraform apply.
When both variables are null, the module discovers and classifies all subnets in vpc_id from their effective route tables.
EOF
  type        = list(string)
}

variable "region" {
  default     = null
  description = <<EOF
AWS Region where regional resources are created.
When null, the AWS provider's configured Region is used.
EOF
  type        = string
}

variable "security_group_ids" {
  default     = []
  description = <<EOF
Additional security group IDs to attach to the Teleport ECS tasks.
These groups are not modified by the module.
EOF
  type        = list(string)
}

variable "teleport_container_image" {
  default     = "public.ecr.aws/gravitational/teleport-ent-distroless"
  description = "Container image used for Teleport ECS tasks."
  type        = string
}

variable "teleport_provision_token_name" {
  default     = "iam-agent"
  description = "Name for the Teleport provision token resource."
  type        = string
  nullable    = false
}

variable "teleport_provision_token_use_name_prefix" {
  default     = true
  description = "Determines whether the name of the Teleport provision token is used as a prefix."
  type        = bool
  nullable    = false
}

variable "teleport_proxy_public_addr" {
  default     = null
  description = <<EOF
Teleport cluster proxy public address `host:port`.
Required when var.database_service.enabled is true.
EOF
  type        = string

  validation {
    condition     = var.teleport_proxy_public_addr == null || !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme."
  }

  validation {
    condition     = var.teleport_proxy_public_addr == null || var.teleport_proxy_public_addr == "" || strcontains(var.teleport_proxy_public_addr, ":")
    error_message = "The address must be in the form `host:port`."
  }
}

variable "teleport_version_override" {
  default     = null
  description = <<EOF
teleport_version_override is intended for development and MUST NOT be used to control the Teleport version in a typical deployment.
This module is designed to run a specific Teleport version.
You will face compatibility issues trying to run a different Teleport version with it.
If you want to run Teleport version `X.Y.Z`, you should adjust the module version to `X.Y.Z` instead.
EOF
  type        = string
  nullable    = true
}
