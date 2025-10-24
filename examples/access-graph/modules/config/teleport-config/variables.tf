variable "nodename" {
  description = "Teleport node name"
  type        = string
}

variable "address" {
  description = "Public address for Teleport (e.g., 'tele.local')"
  type        = string
}

variable "enable_access_graph" {
  description = "Enable Access Graph integration"
  type        = bool
  default     = true
}

variable "enable_ssh_service" {
  description = "Enable SSH service in Teleport"
  type        = bool
  default     = false
}

variable "enable_audit_log" {
  description = "Enable audit log for Access Graph (when Identity Activity Center is enabled)"
  type        = bool
  default     = false
}
