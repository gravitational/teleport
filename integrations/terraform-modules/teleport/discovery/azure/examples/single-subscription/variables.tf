variable "subscription_id" {
  type        = string
  description = "Azure subscription ID for discovery scope."
}

variable "tenant_id" {
  type        = string
  description = "Azure AD tenant ID."
}

variable "region" {
  type        = string
  description = "Azure region for created resources (identities)."
}

variable "discovery_resource_group_names" {
  type        = list(string)
  description = "Resource groups to scan for VMs."
}

variable "proxy_addr" {
  type        = string
  description = "Teleport proxy address (host:port)."
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to Azure resources created by the module."
  default     = {}
}

variable "identity_resource_group_name" {
  type        = string
  description = "Resource group to place identity resources; defaults to first discovery RG when empty."
  default     = null
}

variable "discovery_group_name" {
  type        = string
  description = "Teleport discovery group name."
  default     = "cloud-discovery-group"
}

variable "discovery_tags" {
  type        = map(list(string))
  description = "Tag filters for VM discovery; matches VMs with these tags."
  default = {
    "*" = ["*"]
  }
}

variable "installer_script_name" {
  type        = string
  description = "Name of the Teleport installer script to use."
  default     = "default-installer"
}
