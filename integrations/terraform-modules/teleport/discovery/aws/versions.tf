terraform {
  required_version = ">= 1.5.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    http = {
      source  = "hashicorp/http"
      version = ">= 3.0"
    }
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= 18.5.1"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0"
    }
  }
}
