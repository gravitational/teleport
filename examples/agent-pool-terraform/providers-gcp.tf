// Delete this file if not using Google Cloud

terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "TELEPORT_VERSION"
    }

    google = {
      source  = "hashicorp/google"
      version = "~> 5.5.0"
    }

  }
}

provider "google" {
  project = var.google_project
  region  = var.region
}

provider "teleport" {
  # Update addr to point to your Teleport Cloud tenant URL's host:port
  addr               = var.proxy_service_address
  identity_file_path = "terraform-identity"
}
