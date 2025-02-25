variable "agent_count" {
  type        = number
  description = "Number of agents to deploy"
}

variable "agent_labels" {
  type        = map(string)
  description = "labels to apply to each Agent in addition to \"role:agent-pool\""
}

variable "proxy_service_address" {
  type        = string
  description = "Host and HTTPS port of the Teleport Proxy Service"
}

variable "teleport_edition" {
  type        = string
  default     = "oss"
  description = "Edition of your Teleport cluster. Can be: oss, enterprise, or cloud."
  validation {
    condition     = contains(["oss", "enterprise", "cloud"], var.teleport_edition)
    error_message = "teleport_edition must be one of: oss, enterprise, cloud."
  }
}

variable "teleport_version" {
  type        = string
  description = "Version of Teleport to install on each agent"
}

