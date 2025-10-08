terraform {
  required_version = ">= 1.12.1"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
  }
}
