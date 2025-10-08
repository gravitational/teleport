variable "tenant_id" {
  type        = string
  description = "Entra ID Tenant ID"
}

variable "app_name" {
  type        = string
  description = "Enterprise application name"
  default     = "teleport_entra_id_integration"
}

variable "proxy_service_address" {
  type        = string
  description = "Host and HTTPS port of the Teleport Proxy Service"
}

variable "group_membership_claims" {
  type        = list(string)
  description = "Group claim to be used in the SAML assertion"
  default     = ["SecurityGroup"]
}

# Warning! An expired certificate will break user authentication.
# If you update this value, you should also update the Entra ID 
# Auth Connector in Teleport with a new entity descriptor. 
variable "certificate_expiry_date" {
  type        = string
  description = "Expiry date for the certificate that will be created for SAML assertion signing"
}

variable "use_system_credentials" {
  type        = bool
  default     = false
  description = "Defines how Teleport will authenticate with the Microsoft Graph API"
}

# Only required if use_system_credentials=true
variable "graph_permission_ids" {
  type        = list(string)
  default     = []
  description = "Permission IDs to be assigned to the managed identity"
}

# Only required if use_system_credentials=true
# You can also reference a system assigned managed identity configured in the VM resource:
# azurerm_virtual_machine.my_vm.identity[0].principal_id
variable "managed_id" {
  type        = string
  default     = ""
  description = "Principal ID of the managed identity"
}
