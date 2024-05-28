variable "agent_count" {
  type        = number
  description = "Number of agents to deploy"
}

variable "agent_roles" {
  type        = list(string)
  description = "The roles that the agent is allowed to have."
  default     = ["Node"]
  validation {
    condition = length(setsubtract(var.agent_roles, [
      "App",
      "Db",
      "Discovery",
      "Kube",
      "Node",
    ])) == 0
    error_message = "agent_roles must be one or more of ${join(", ", [
      "App",
      "Db",
      "Discovery",
      "Kube",
      "Node",
    ])}"
  }
}

variable "proxy_service_address" {
  type        = string
  description = "Host and HTTPS port of the Teleport Proxy Service"
}

variable "teleport_edition" {
  type        = string
  default     = "oss"
  description = "Edition of your Teleport cluster. Can be: oss, enterprise, team, or cloud."
  validation {
    condition     = contains(["oss", "enterprise", "team", "cloud"], var.teleport_edition)
    error_message = "teleport_edition must be one of: oss, enterprise, team, cloud."
  }
}

variable "teleport_version" {
  type        = string
  description = "Version of Teleport to install on each agent"
}

