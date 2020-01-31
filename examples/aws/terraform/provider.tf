terraform {
  required_version = "~> 0.12"
}

provider "random" {
  version = "~> 2.2.1"
}

provider "template" {
  version = "~> 2.1.2"
}

variable "aws_max_retries" {
  default = 5
}

provider "aws" {
  version = "~> 2.0"
  region  = var.region
}

