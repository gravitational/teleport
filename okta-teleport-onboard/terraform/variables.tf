variable "okta_org_url" {
  description = "Full Okta org URL, e.g. https://dev-123.okta.com"
  type        = string
}

variable "teleport_domain" {
  description = "Teleport proxy host, e.g. jwardtest18.cloud.gravitational.io"
  type        = string
}

variable "sso_group" {
  description = "Okta group granted SSO access to the SAML app"
  type        = string
  default     = "Everyone"
}

variable "label_suffix" {
  description = "Suffix on created object labels to avoid cross-cluster collisions (defaults to the domain)"
  type        = string
  default     = ""
}
