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

  # Enable managed updates
  managed_updates_enabled = true
  managed_updates_group   = "default"

  apply_aws_tags         = { "example" = "true" }
  assign_public_ip       = true # must be true when using public subnets
  ecs_cluster_name       = "${local.namespace}-cluster"
  ecs_service_name       = "${local.namespace}-svc"
  ecs_service_subnets    = module.vpc.public_subnets
  ecs_task_desired_count = 1
  ecs_task_name          = "${local.namespace}-task"
  environment_vars       = { EXAMPLE_VAR = "EXAMPLE_VALUE" }
  vpc_id                 = module.vpc.vpc_id

  teleport_config = {
    version = "v3"
    teleport = {
      join_params = {
        token_name = "${local.namespace}-iam"
        method     = "iam"
      }
      proxy_server = var.teleport_proxy_addr
      log = {
        severity = "DEBUG"
      }
    }
    auth_service = {
      enabled = "no"
    }
    proxy_service = {
      enabled = "no"
    }
    ssh_service = {
      enabled = "no"
    }
    discovery_service = {
      enabled = "no"
    }
    db_service = {
      enabled = "yes"
      resources = [
        {
          labels = {
            "env" = "example"
          }
        }
      ]
    }
  }
}

resource "teleport_provision_token" "iam" {
  metadata = {
    name        = "${local.namespace}-iam"
    description = "Allow the Teleport ECS agent to join the cluster using AWS IAM credentials."
  }
  spec = {
    allow = [{
      aws_arn = module.teleport_database_service.teleport_provision_token_allow_aws_arn
    }]
    join_method = "iam"
    roles       = ["Db"]
  }
  version = "v2"
}
