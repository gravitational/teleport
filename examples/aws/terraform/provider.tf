terraform {
  required_version = "~> 0.10.8"
}

provider "random" {
  version = "~> 1.0"
}

provider "template" {
  version = "~> 1.0"
}

variable "aws_max_retries" {
  default = 5
}

provider "aws" {
  version                 = "~> 1.7"
}
