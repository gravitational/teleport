variable "insecure_direct_access" {
  type        = bool
  default     = false
  description = "Whether to enable direct access to agent instances. Only enable this in low-security demo environments."
}

variable "region" {
  type        = string
  description = "Location in which to deploy agents (Azure location, AWS or GCP region)"
}

variable "subnet_id" {
  type        = string
  description = "Cloud provider subnet for deploying Teleport agents (subnet ID if using AWS or Azure, name or self link if using GCP)"
}

variable "userdata_scripts" {
  type        = list(string)
  description = "User data scripts to provide to VM instances. Determines the count of agents to deploy."
}
