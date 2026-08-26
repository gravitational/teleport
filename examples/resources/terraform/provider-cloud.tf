terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> (=teleport.major_version=).0"
    }
  }
}

provider "teleport" {
  # Update addr to point to your Teleport Enterprise (managed) tenant URL's host:port
  addr               = "mytenant.teleport.sh:443"
  identity_file_path = "terraform-identity/identity"
}

# creates a test role, if we don't declare resources, Terraform won't try to
# connect to Teleport and we won't be able to validate the setup.
resource "teleport_role" "test" {
  version = "v7"
  metadata = {
    name        = "test"
    description = "Dummy role to validate Terraform Provider setup"
    labels = {
      test = "yes"
    }
  }

  spec = {
  }
}
