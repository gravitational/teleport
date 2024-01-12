variable "azure_resource_group" {
  type        = string
  default     = ""
  description = "Azure location in which to deploy agents"
}

variable "insecure_direct_access" {
  type        = bool
  default     = false
  description = "Whether to enable direct access to agent instances. Only enable this in low-security demo environments."
}

variable "public_key_path" {
  type        = string
  description = "Path to a valid RSA public key with at least 2048 bits. The key is only used to pass validation in Azure, and is deleted from VMs created by this module."
  default     = ""
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
