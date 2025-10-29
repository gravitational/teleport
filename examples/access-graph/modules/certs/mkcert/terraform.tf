terraform {
  required_version = ">= 1.12.1"

  required_providers {
    external = {
      source  = "hashicorp/external"
      version = ">= 2.3.0"
    }
    local = {
      source  = "hashicorp/local"
      version = ">= 2.5.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
  }
}
