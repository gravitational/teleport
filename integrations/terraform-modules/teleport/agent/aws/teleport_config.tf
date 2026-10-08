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
  database_service_resources = (
    var.database_service.resources != null
    ? var.database_service.resources
    : [{
      labels = {
        "account-id" = [one(data.aws_caller_identity.this[*].account_id)]
        "region"     = [one(data.aws_region.this[*].region)]
        "vpc-id"     = [var.vpc_id]
      }
      aws = null
    }]
  )
  database_service_default_aws = {
    for enabled in [var.ecs_task_role_self_assumption_allowed] : "assume_role_arn" => one(aws_iam_role.ecs_task[*].arn)
    if enabled
  }
  database_service_resources_with_aws = [
    for matcher in local.database_service_resources : {
      labels = matcher.labels
      aws = merge(
        local.database_service_default_aws,
        # Null inherits the default; an explicit empty ARN opts out.
        matcher.aws == null ? {} : {
          for key, value in matcher.aws : key => value if value != null
        },
      )
    }
  ]
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
      resources = [
        for matcher in local.database_service_resources_with_aws : merge(
          { labels = matcher.labels },
          length(matcher.aws) > 0 ? { aws = matcher.aws } : {},
        )
      ]
    }
  }
}
