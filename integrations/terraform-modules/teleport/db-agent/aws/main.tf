module "teleport_db_service" {
  # TODO(gavin): change this to the published module version
  # source  = "terraform.releases.teleport.dev/teleport/container-service/aws"
  source = "../../container-service/aws"

  apply_aws_tags                               = var.apply_aws_tags
  assign_public_ip                             = var.assign_public_ip
  create                                       = var.create
  create_security_group                        = var.create_security_group
  ecs_cluster_name                             = var.ecs_cluster_name
  ecs_service_name                             = var.ecs_service_name
  ecs_service_subnets                          = var.ecs_service_subnets
  ecs_task_cloudwatch_log_group_name           = var.ecs_task_cloudwatch_log_group_name
  ecs_task_cloudwatch_log_group_region         = var.ecs_task_cloudwatch_log_group_region
  ecs_task_cloudwatch_log_group_retention_days = var.ecs_task_cloudwatch_log_group_retention_days
  ecs_task_cloudwatch_log_group_skip_destroy   = var.ecs_task_cloudwatch_log_group_skip_destroy
  ecs_task_cpu                                 = var.ecs_task_cpu
  ecs_task_desired_count                       = var.ecs_task_desired_count
  ecs_task_force_new_deployment                = var.ecs_task_force_new_deployment
  ecs_task_memory                              = var.ecs_task_memory
  ecs_task_name                                = var.ecs_task_name
  ecs_task_role_inline_policy                  = var.ecs_task_role_inline_policy
  environment_vars                             = var.environment_vars
  managed_updates_enabled                      = var.managed_updates_enabled
  managed_updates_group                        = var.managed_updates_group
  security_group_ids                           = var.security_group_ids
  teleport_container_image                     = var.teleport_container_image
  teleport_version                             = var.teleport_version
  vpc_id                                       = var.vpc_id

  teleport_config = {
    version = "v3"
    teleport = {
      nodename     = "" # setting nodename to an empty string ensures that it's picked up from the host's hostname
      proxy_server = var.teleport_proxy_public_addr
      join_params = (
        var.join_params != null
        ? var.join_params
        : var.create
        ? {
          token_name = teleport_provision_token.agent_aws_iam[0].metadata.name
          method     = "iam"
        }
        : null
      )
      log = {
        output   = "stderr"
        severity = var.log_level
      }
    }
    auth_service = {
      enabled = false
    }
    proxy_service = {
      enabled = false
    }
    ssh_service = {
      enabled = false
    }
    db_service = {
      enabled = true
      resources = (
        var.database_service_resources != null
        ? var.database_service_resources
        : [{
          aws = null
          labels = {
            "account-id" = data.aws_caller_identity.this[*].account_id
            "region"     = data.aws_region.this[*].name
            "vpc-id"     = [var.vpc_id]
          }
        }]
      )
    }
  }
}
