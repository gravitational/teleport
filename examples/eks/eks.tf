// Region is AWS region, the region should support EFS
variable "region" {
  type = "string"
  default = "us-west-2"
}

// Script creates a separate VPC with demo deployment
variable "vpc_cidr" {
  type = "string"
  default = "172.31.0.0/16"
}

variable "cluster-name" {
  default = "terraform-eks-demo"
  type    = "string"
}

provider "aws" {
  version    = "~> 2.1.0"
  region     = "${var.region}"
}



