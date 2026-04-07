terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 15.0"
    }
  }
}

provider "teleport" {
  # Update addr to point to Teleport Auth/Proxy
  # addr              = "telepop.example.com:3025"
  addr               = "teleport.example.com:443"
  identity_file_path = "../../tmp/will-terraform.pem"
}
