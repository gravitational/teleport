terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0"
    }
    teleport = {
      source = "terraform.releases.teleport.dev/gravitational/teleport"
      # TODO(marco): update to the version that includes Organizational Unit filtering in the IAM token rules.
      # https://github.com/gravitational/teleport/pull/66242
      version = ">= 18.8.1"
    }
  }
}
