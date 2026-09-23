locals {
  namespace = "example"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.6.0"

  azs  = slice(data.aws_availability_zones.this.names, 0, 3)
  cidr = "10.0.0.0/16"
  name = "${local.namespace}-vpc"

  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  private_subnets = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

module "teleport_database_service" {
  source = "../.."

  database_service = {
    allow_database_modification       = true
    default_iam_policy_database_types = ["rds"]
    enabled                           = true
  }

  # Enable managed updates
  managed_updates_enabled = true
  managed_updates_group   = "default"

  apply_aws_tags             = { "example" = "true" }
  assign_public_ip           = true # must be true when using public subnets
  ecs_cluster_name           = "${local.namespace}-cluster"
  ecs_service_name           = "${local.namespace}-svc"
  ecs_service_subnets        = module.vpc.public_subnets
  ecs_task_desired_count     = 1
  environment_vars           = { EXAMPLE_VAR = "EXAMPLE_VALUE" }
  teleport_proxy_public_addr = var.teleport_proxy_addr
  vpc_id                     = module.vpc.vpc_id
}
