variable "teleport_domain" {
  type        = string
  description = "Domain name of your Teleport cluster (to configure WebAuthn)"
}

variable "oidc_claims_to_roles" {
  type = list(object({
    claim = string
    roles = list(string)
    value = string
  }))
  description = "Mappings of OIDC claims to lists of Teleport role names"
}

variable "oidc_client_id" {
  type        = string
  description = "The OIDC identity provider's client iD"
}

variable "oidc_connector_name" {
  type        = string
  description = "Name of the Teleport OIDC connector resource"
}

variable "oidc_redirect_url" {
  type        = string
  description = "Redirect URL for the OIDC provider."
}

variable "oidc_secret" {
  type        = string
  description = "Secret for configuring the Teleport OIDC connector. Available from your identity provider."
}

