variable "gcp_zone" {
  type        = string
  default     = ""
  description = "GCP zone to associate agents with"
}

variable "google_project" {
  type        = string
  default     = ""
  description = "GCP project to associate agents with"
}

variable "insecure_direct_access" {
  type        = bool
  default     = false
  description = "Whether to enable direct access to agent instances. Only enable this in low-security demo environments."
}

variable "subnet_id" {
  type        = string
  description = "Cloud provider subnet for deploying Teleport agents (subnet ID if using AWS or Azure, name or self link if using GCP)"
}

variable "userdata_scripts" {
  type        = list(string)
  description = "User data scripts to provide to VM instances. Determines the count of agents to deploy."
}
