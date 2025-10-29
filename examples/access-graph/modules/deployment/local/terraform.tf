terraform {
  required_version = ">= 1.12.1"

  required_providers {
    local = {
      source  = "hashicorp/local"
      version = ">= 2.5.0"
    }
  }
}
