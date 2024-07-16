terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 15.0"
    }
  }
}

provider "teleport" {
  # Update addr to point to your Teleport Cloud tenant URL's host:port
  addr               = "mytenant.teleport.sh:443"
  identity_file_path = "terraform-identity/identity"
}
