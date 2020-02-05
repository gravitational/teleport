terraform {
  required_version = "~> 0.12.20"
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
  version = "~> 2.46"
  region  = var.region
}

