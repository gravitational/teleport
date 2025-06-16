variable "aws_region" {
  type        = string
  description = "The AWS region where the ECS cluster and services will be deployed; this must match the region of your EKS cluster."
}

variable "teleport_image" {
  type        = string
  default     = "public.ecr.aws/gravitational-staging/teleport-ent-distroless:18.0.0-alpha.4"
  description = "Teleport image location and tag. Keep it up to date."
}

variable "ecs_cluster" {
  type        = string
  description = "The AWS ECS cluster name where the discovery and kubernetes services will run."
}

variable "ecs_taskrole" {
  type        = string
  default     = "teleport_ecs_discovery_kubernetes_taskrole"
  description = "The IAM role for the ECS task that allows it to interact with AWS EKS APIs."
}

variable "ecs_executionrole" {
  type        = string
  default     = "teleport_ecs_discovery_kubernetes_executionrole"
  description = "The IAM role for the ECS agent to assume, only needed to write process logs into CloudWatch."
}

variable "teleport_agent_subnets" {
  type        = list(string)
  description = "The subnets where the Teleport Agent runs. Must allow access to: teleport cluster, teleport container registry and EKS Clusters to proxy."
}

variable "teleport_agent_security_groups" {
  type        = list(string)
  description = "The security groups for the Teleport Agent. Must have access to: teleport cluster, teleport container registry and EKS Clusters to proxy."
}

variable "default_tags" {
  type        = map(string)
  description = "Default tags to apply to all resources created by this Terraform configuration."
}

variable "discover_eks_tags" {
  type        = map(list(string))
  description = "Tags used to filter EKS Clusters. Only matching clusters will be accessible."
}

variable "teleport_proxy_server" {
  type = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9.-]+:[0-9]+$", var.teleport_proxy_server))
    error_message = "The teleport_proxy_server must be in the format 'hostname:port'."
  }
  description = "The Teleport Proxy server address. Example: tenant.teleport.sh:443"
}


variable "teleport_iam_token_name" {
  type        = string
  default     = "iam-join-token"
  description = "The Teleport IAM Join Token for the agent to use. It must be an IAM Join Token that allows Discovery and Kubernetes Roles."
}
