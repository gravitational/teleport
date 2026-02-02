variable "proxy_public_addr" {
  description = "Teleport Proxy public address (with or without https:// prefix, e.g., 'teleport.example.com' or 'https://teleport.example.com')"
  type        = string
}

variable "auth_connector_name" {
  description = "Name of the SAML connector in Teleport"
  type        = string
  default     = "entra-id"
}

variable "attributes_to_roles" {
  description = "List of attribute mappings from Azure AD groups to Teleport roles"
  type = list(object({
    name  = string
    value = string
    roles = list(string)
  }))
  default = [
    {
      name  = "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
      value = "*"
      roles = ["requester"]
    }
  ]
}

variable "display" {
  description = "Display name for the SAML connector in Teleport UI"
  type        = string
  default     = "Entra ID"
}

variable "allow_idp_initiated" {
  description = "Allow IdP-initiated SAML login"
  type        = bool
  default     = false
}