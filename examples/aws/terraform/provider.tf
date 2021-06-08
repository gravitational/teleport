terraform {
  required_version = ">0.13"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 2.46"
    }
    template = {
      source  = "hashicorp/template"
      version = "~> 2.2.0"
    }
    random = {
      source = "hashicorp/random"
      version = "~> 2.2.1"
    }
  }
}

# This region must match the az_list that you configure in the module
provider "aws" {
  region = "us-east-1"
}
