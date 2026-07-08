terraform {
  required_version = ">= 1.5"
  required_providers {
    okta = {
      source  = "okta/okta"
      version = "~> 4.13"
    }
    teleport = {
      source = "terraform.releases.teleport.dev/gravitational/teleport"
      # Pinned to the cluster's major version. Edit to match cluster (`tctl status`).
      version = "~> 18.0"
    }
  }
}
