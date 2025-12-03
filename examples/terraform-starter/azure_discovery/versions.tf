terraform {
  required_version = ">= 1.6.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.0"
    }

    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "19.0.0-dev"
    }
  }
}
