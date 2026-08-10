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

module "teleport_db_agent" {
  source = "../.."

  assign_public_ip           = true # Required when using public subnets.
  ecs_service_subnets        = module.vpc.public_subnets
  managed_updates_enabled    = true
  teleport_proxy_public_addr = var.teleport_proxy_addr
  vpc_id                     = module.vpc.vpc_id

  ecs_task_role_inline_policy           = data.aws_iam_policy_document.example_statement_override.json
  database_types_for_default_iam_policy = ["rds"]
}

data "aws_iam_policy_document" "example_statement_override" {
  statement {
    # matches the SID of a statement in default IAM policy for "rds" database
    # and overrides it
    sid    = "RDSAutoEnableIAMAuth"
    effect = "Allow"

    actions = [
      "rds:ModifyDBCluster",
      "rds:ModifyDBInstance",
    ]
    resources = [
      # restrict the default modify IAM permissions to specific databases instead of "*"
      "arn:aws:rds:${data.aws_region.this.region}:${data.aws_caller_identity.this.account_id}:db:example"
    ]
  }
}
