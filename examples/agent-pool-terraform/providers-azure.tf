// Delete this file if not using Azure

terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "TELEPORT_VERSION"
    }

    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0.0"
    }
  }
}

provider "teleport" {
  # Update addr to point to your Teleport Cloud tenant URL's host:port
  addr               = var.proxy_service_address
  identity_file_path = "terraform-identity"
}

provider "azurerm" {
  features {}
}
