provider "aws" {
  region = var.region
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "all" {
  vpc_id = data.aws_vpc.default.id
}

data "aws_ami" "base" {
  most_recent = true
  owners      = [126027368216]

  filter {
    name   = "name"
    values = [var.ami_name]
  }
}

data "aws_route53_zone" "cluster" {
  name = var.route53_zone
}

data "aws_caller_identity" "current" {
}

data "aws_region" "current" {
  name = var.region
}

data "aws_availability_zones" "available" {
}

// SSM is picking alias for key to use for encryption in SSM
data "aws_kms_alias" "ssm" {
  name = var.kms_alias_name
}
