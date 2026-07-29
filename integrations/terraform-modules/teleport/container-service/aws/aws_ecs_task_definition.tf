################################################################################
# Task definition
################################################################################

locals {
  managed_updates_proxy_addr = try(
    replace(
      trimspace(var.teleport_config.teleport.proxy_server),
      "/^[^:]*:[/]{2}/", # trim any URL scheme
    ""),
  "")
  managed_updates_version = (
    length(data.http.managed_updates) == 1
    ? jsondecode(data.http.managed_updates[0].response_body).auto_update.agent_version
    : null
  )
  teleport_version = trimprefix(
    trimspace(
      coalesce(
        local.managed_updates_version,
        var.teleport_version
      ),
    ),
    "v"
  )
}

resource "aws_ecs_task_definition" "teleport_agent" {
  count = var.create ? 1 : 0

  container_definitions = jsonencode([
    {
      command = [
        # rewrite SIGTERM (15) to SIGQUIT (3) so ECS stop signal triggers graceful Teleport shutdown
        "--rewrite",
        "15:3",
        "--",
        "teleport",
        "start",
        "--config-string",
        base64encode(yamlencode(var.teleport_config)),
      ]
      entryPoint = ["/usr/bin/dumb-init"]
      environment = [
        for name in sort(keys(var.environment_vars)) : {
          name  = name
          value = var.environment_vars[name]
        }
      ]
      image = "${var.teleport_container_image}:${local.teleport_version}"
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = one(aws_cloudwatch_log_group.this[*].name)
          "awslogs-region"        = one(aws_cloudwatch_log_group.this[*].region)
          "awslogs-stream-prefix" = "${var.ecs_cluster_name}-${var.ecs_service_name}"
        }
      }
      name = "teleport"
    }
  ])
  cpu                      = var.ecs_task_cpu
  execution_role_arn       = one(aws_iam_role.ecs_execution[*].arn)
  family                   = var.ecs_task_name
  memory                   = var.ecs_task_memory
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  tags                     = var.apply_aws_tags
  task_role_arn            = one(aws_iam_role.ecs_task[*].arn)

  lifecycle {
    precondition {
      condition     = !var.managed_updates_enabled || local.managed_updates_proxy_addr != ""
      error_message = "Managed Updates require teleport.proxy_server in teleport_config."
    }
  }
}
