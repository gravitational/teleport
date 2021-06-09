terraform {
  required_version = "> 0.13"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 2.0"
    }
    template = {
      source  = "hashicorp/template"
      version = "~> 2.2.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.2.1"
    }
    local = {
      source = "hashicorp/local"
      version = "~> 2.0.0"
    }
  }
}

variable "aws_max_retries" {
  default = 5
}
