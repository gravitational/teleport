variable "admin_role_name" {
  type        = string
  description = "Custom admin role name"
  default     = "Teleport Okta Integration"
}

variable "api_app_name" {
  type        = string
  description = "API Services app name"
  default     = "Teleport Okta Integration API"
}

variable "app_name" {
  type        = string
  description = "SAML app name"
  default     = "Teleport Okta Integration"
}

variable "connector_name" {
  type        = string
  description = "Name of the Teleport SAML connector for Okta"
  default     = "okta"
}

variable "enable_bidirectional_sync" {
  type        = bool
  description = "Grant Teleport the Okta permissions it needs for bidirectional sync. When false, Teleport gets read-only access to Okta."
  default     = false
}

variable "group_names" {
  type        = list(string)
  description = "Okta groups whose members can log in to Teleport"
  default     = []
}

variable "okta_base_url" {
  type        = string
  description = "Okta organization domain, for example `okta.com` or `oktapreview.com`"
  default     = "okta.com"
}

variable "okta_org_name" {
  type        = string
  description = "Okta organization name, for example `dev-123456` for dev-123456.okta.com"
}

variable "proxy_service_address" {
  type        = string
  description = "Host and HTTPS port of the Teleport Proxy Service"
}

variable "resource_set_name" {
  type        = string
  description = "Resource set name"
  default     = "Teleport Okta Integration"
}
