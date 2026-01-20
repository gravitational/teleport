################################################################################
# Required variables
################################################################################

variable "azure_managed_identity_location" {
  type        = string
  description = "Azure region (location) where the managed identity will be created (e.g., \"westus\")."
  nullable    = false
}

variable "azure_resource_group_name" {
  type        = string
  description = "Name of an existing Azure Resource Group where Azure resources will be created."
  nullable    = false
}

variable "teleport_discovery_group_name" {
  description = "Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use \"cloud-discovery-group\"."
  type        = string
  nullable    = false
}

variable "teleport_proxy_public_addr" {
  description = "Teleport cluster proxy public address in the form <host:port> (no URL scheme)."
  type        = string
  nullable    = false

  validation {
    condition     = !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme."
  }

  validation {
    condition     = var.teleport_proxy_public_addr == "" || strcontains(var.teleport_proxy_public_addr, ":")
    error_message = "The address must be in the form <host:port>."
  }
}

################################################################################
# Optional variables
################################################################################

variable "apply_azure_tags" {
  type        = map(string)
  description = "Additional Azure tags to apply to all created Azure resources."
  default     = {}
  nullable    = false
}

variable "apply_teleport_resource_labels" {
  description = "Additional Teleport resource labels to apply to all created Teleport resources."
  type        = map(string)
  default     = {}
  nullable    = false
}

variable "azure_federated_identity_credential_name" {
  description = "Name of the Azure federated identity credential created for workload identity federation."
  type        = string
  default     = "teleport-federation"
  nullable    = false
}

variable "azure_managed_identity_name" {
  description = "Name of the Azure user-assigned managed identity created for Teleport Discovery."
  type        = string
  default     = "discovery-identity"
  nullable    = false
}

variable "azure_role_definition_name" {
  description = "Name for the Azure custom role definition created for Teleport Discovery."
  type        = string
  default     = "teleport-discovery"
  nullable    = false
}

variable "create" {
  description = "Toggle creation of all resources."
  type        = bool
  default     = true
  nullable    = false
}

variable "match_azure_regions" {
  type        = list(string)
  description = "Azure regions to discover. Defaults to [\"*\"] which matches all regions. Region names should be the programmatic region name, e.g., \"westus\"."
  default     = ["*"]
  nullable    = false
}

variable "match_azure_resource_groups" {
  type        = list(string)
  description = "Azure resource groups to scan for VMs. Defaults to [\"*\"] which matches all resource groups."
  default     = ["*"]
  nullable    = false
}

variable "match_azure_tags" {
  type        = map(list(string))
  description = "Tag filters for VM discovery; matches VMs with these tags. Defaults to {\"*\" = [\"*\"]} which matches all tags."
  default = {
    "*" = ["*"]
  }
  nullable = false
}

variable "teleport_discovery_config_name" {
  description = "Name for the `teleport_discovery_config` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_discovery_config_use_name_prefix" {
  description = "Whether `teleport_discovery_config_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_installer_script_name" {
  type        = string
  description = "Name of an existing Teleport installer script to use."
  default     = "default-installer"
  nullable    = false
}

variable "teleport_integration_name" {
  description = "Name for the `teleport_integration` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_integration_use_name_prefix" {
  description = "Whether `teleport_integration_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}

variable "teleport_provision_token_name" {
  description = "Name for the `teleport_provision_token` resource."
  type        = string
  default     = "discovery"
  nullable    = false
}

variable "teleport_provision_token_use_name_prefix" {
  description = "Whether `teleport_provision_token_name` is used as a name prefix (true) or as the exact name (false)."
  type        = bool
  default     = true
  nullable    = false
}
