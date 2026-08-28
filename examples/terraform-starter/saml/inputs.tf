variable "teleport_domain" {
  type        = string
  description = "Domain name of your Teleport cluster (to configure WebAuthn)"
}

variable "saml_connector_name" {
  type        = string
  description = "Name for the SAML authentication connector created by this module"
}

variable "saml_attributes_to_roles" {
  type = list(object({
    name  = string
    roles = list(string)
    value = string
  }))
  description = "Mappings of SAML attributes to lists of Teleport role names"
}

variable "saml_acs" {
  type        = string
  description = "URL (scheme, domain, port, and path) for the SAML assertion consumer service"
}

variable "saml_entity_descriptor" {
  type        = string
  description = "SAML entity descriptor"
}
