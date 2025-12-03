variable "subscription_id" {
  description = "Azure subscription ID that Teleport discovery and role assignments target."
  type        = string
}

variable "tenant_id" {
  description = "Azure tenant ID for the subscription (kept as input for completeness even if unused)."
  type        = string
}

variable "region" {
  description = "Azure region where discovery should look for resources."
  type        = string
}

variable "proxy_addr" {
  description = "Teleport proxy address the installer uses when enrolling nodes."
  type        = string
}

variable "discovery_resource_group_name" {
  description = "Resource group to scope discovery against."
  type        = string
}

variable "prefix" {
  description = "Prefix for resource names"
  type        = string
}

variable "discovery_group_name" {
  description = "Discovery group to use."
  type        = string
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}

variable "use_managed_identity" {
  description = "If true, use a User Assigned Identity. If false, use an Azure AD Application (Service Principal)."
  type        = bool
  default     = true
}
