terraform {
  required_version = ">= 1.5"
  required_providers {
    okta = {
      source  = "okta/okta"
      version = "~> 4.13"
    }
  }
}

# Auth comes from the environment (SSWS token):
#   OKTA_ORG_NAME, OKTA_BASE_URL, OKTA_API_TOKEN
provider "okta" {}
