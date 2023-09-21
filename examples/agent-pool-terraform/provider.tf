terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "TELEPORT_VERSION"
    }

    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

provider "teleport" {
  # Update addr to point to your Teleport Cloud tenant URL's host:port
  addr               = var.proxy_service_address
  identity_file_path = "terraform-identity"
}
