variable "proxy_addr" {
  description = "Teleport Proxy address. E.g., example.teleport.sh:443."
  type        = string
}

variable "acl_owner" {
  description = "Teleport user who owns the load test Access Lists."
  type        = string
}

variable "acl_count" {
  description = <<-EOT
    Number of load-test Access Lists. k6-group-0 .. k6-group-<acl_count-1>.
    In a consecutive run, a lower value deletes the lists above it.
  EOT
  type        = number

  validation {
    condition     = var.acl_count >= 1
    error_message = "acl_count must be at least 1."
  }
}
