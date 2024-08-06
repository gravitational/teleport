terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0.0"
    }

    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= 16.0"
    }
  }
}
