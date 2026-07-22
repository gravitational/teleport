variable "teleport_proxy_server" {
  type = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9.-]+:[0-9]+$", var.teleport_proxy_server))
    error_message = "The teleport_proxy_server must be in the format 'hostname:port'."
  }
  description = "The Teleport Proxy server address. Example: tenant.teleport.sh:443"
}

variable "aws_region" {
  type        = string
  description = "The AWS region where the Teleport agent will be deployed."
}

variable "ecs_cluster" {
  type        = string
  description = "The AWS ECS cluster name where Teleport agent will run."
}

variable "ecs_taskrole" {
  type        = string
  default     = "teleport_ecs_agent_taskrole"
  description = "The IAM role for the ECS task that allows it to interact with AWS APIs."
}

variable "ecs_taskrole_policy" {
  type        = any
  description = "The policy assigned to the IAM Role passed to the Teleport Agent."
}

variable "ecs_executionrole" {
  type        = string
  default     = "teleport_ecs_agent_executionrole"
  description = "The IAM role for the ECS agent to assume, only needed to write process logs into CloudWatch."
}

variable "teleport_agent_subnets" {
  type        = list(string)
  description = "The subnets where the Teleport Agent runs. Must allow access to: teleport cluster, teleport container registry and any private resource (e.g., EKS, RDS) to proxy."
}

variable "teleport_agent_security_groups" {
  type        = list(string)
  description = "The security groups for the Teleport Agent. Must have access to: teleport cluster, teleport container registry and any private resource (e.g., EKS, RDS) to proxy."
}

variable "default_tags" {
  type        = map(string)
  description = "Default tags to apply to all resources created by this Terraform configuration."
}

variable "teleport_task_family" {
  type        = string
  default     = "teleport-ecs-agent"
  description = "The ECS task definition family name."
}

variable "teleport_agent_config" {
  type        = any
  description = "The Teleport Agent configuration. Write the configuration using native Terraform syntax."
}
