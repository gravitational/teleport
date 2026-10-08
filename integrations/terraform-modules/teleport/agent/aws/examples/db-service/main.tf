locals {
  namespace = "example"
  regions   = toset(["us-east-1", "us-west-1"])
}

module "vpc" {
  for_each = local.regions

  source  = "terraform-aws-modules/vpc/aws"
  version = "6.6.0"

  region = each.key
  azs    = slice(data.aws_availability_zones.this[each.key].names, 0, 2)
  cidr   = "10.0.0.0/16"
  name   = "${local.namespace}-vpc"

  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24"]
  private_subnets = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway = true
}

module "teleport_database_service" {
  for_each = local.regions

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
  ecs_cluster_name           = "${local.namespace}-cluster"
  ecs_service_name           = "${local.namespace}-svc"
  ecs_task_desired_count     = 1
  environment_vars           = { EXAMPLE_VAR = "EXAMPLE_VALUE" }
  private_subnet_ids         = module.vpc[each.key].private_subnets
  region                     = each.key
  teleport_proxy_public_addr = var.teleport_proxy_addr
  vpc_id                     = module.vpc[each.key].vpc_id
}
