variable "agent_count" {
  type        = number
  description = "Number of agents to deploy"
}

variable "proxy_service_address" {
  type        = string
  description = "Host and HTTPS port of the Teleport Proxy Service"
}

variable "aws_region" {
  type        = string
  description = "Region in which to deploy AWS resources"
}

variable "teleport_version" {
  type        = string
  description = "Version of Teleport to install on each agent"
}

variable "subnet_id" {
  type        = string
  description = "ID of the AWS subnet for deploying Teleport agents"
}
