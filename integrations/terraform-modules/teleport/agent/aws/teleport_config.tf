locals {
  teleport_provision_token_allow_aws_arn = (
    var.create
    ? format(
      "arn:%v:sts::%v:assumed-role/%v/*",
      one(data.aws_partition.this[*].partition),
      one(data.aws_caller_identity.this[*].account_id),
      one(aws_iam_role.ecs_task[*].name),
    )
    : null
  )
  database_service_config_enabled = var.database_service.enabled
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? format("%v-%v",
      var.teleport_provision_token_name,
      one(random_string.name_suffix[*].result),
    )
    : var.teleport_provision_token_name
  )
  teleport_config = {
    version = "v3"
    teleport = {
      # Setting nodename to an empty string ensures that it is picked up from
      # the host's hostname.
      nodename     = ""
      proxy_server = var.teleport_proxy_public_addr
      join_params = (
        var.join_params != null
        ? var.join_params
        : local.database_service_config_enabled && var.create
        ? {
          token_name = local.teleport_provision_token_name
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
        var.database_service.resources != null
        ? var.database_service.resources
        : [merge(
          {
            labels = {
              "account-id" = [one(data.aws_caller_identity.this[*].account_id)]
              "region"     = [one(data.aws_region.this[*].region)]
              "vpc-id"     = [var.vpc_id]
            }
          },
          {
            for enabled in [var.ecs_task_role_self_assumption_allowed] : "aws" => {
              assume_role_arn = one(aws_iam_role.ecs_task[*].arn)
            }
            if enabled
          },
        )]
      )
    }
  }
}
