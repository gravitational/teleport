terraform {
  required_providers {
    okta = {
      source  = "okta/okta"
      version = "~> 8.0"
    }
  }
  required_version = ">= 1.5"
}
