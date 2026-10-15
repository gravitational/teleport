variable "proxy_addr" {
  description = "Teleport Proxy address. E.g., example.teleport.sh:443."
  type        = string
}

variable "connector_name" {
  description = "Name of the SAML connector."
  type        = string
  default     = "loadtest"
}

variable "entity_descriptor_file" {
  description = "Path to the IdP SAML metadata file."
  type        = string
}