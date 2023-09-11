terraform {
  required_version = "~> 1.5.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.12"
    }
    postgresql = {
      source  = "cyrilgdn/postgresql"
      version = "~> 1.20"
    }
    mysql = {
      source  = "petoju/mysql"
      version = "~> 3.0"
    }
  }
}
