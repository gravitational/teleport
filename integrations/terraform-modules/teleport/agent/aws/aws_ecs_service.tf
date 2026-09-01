locals {
  create_security_group = var.create && var.create_security_group
}

################################################################################
# ECS Service
################################################################################

resource "aws_ecs_service" "teleport_agent" {
  count = var.create ? 1 : 0

  # Roles are usable by ECS only once their inline policies are
  # attached; the role-ARN reference alone does not order this.
  depends_on = [
    aws_iam_role_policy.ecs_execution,
    aws_iam_role_policy.ecs_task,
  ]

  cluster              = one(aws_ecs_cluster.teleport_agent[*].id)
  desired_count        = var.ecs_task_desired_count
  force_new_deployment = var.ecs_task_force_new_deployment
  launch_type          = "FARGATE"
  name                 = var.ecs_service_name
  tags                 = var.apply_aws_tags
  task_definition      = one(aws_ecs_task_definition.teleport_agent[*].arn)

  network_configuration {
    # Public subnets: tasks need a public IP for IGW egress.
    # Private subnets: tasks egress via the NAT route instead.
    assign_public_ip = var.assign_public_ip
    security_groups  = concat(aws_security_group.teleport_agent[*].id, var.security_group_ids)
    subnets          = var.ecs_service_subnets
  }

  lifecycle {
    precondition {
      condition     = var.create_security_group || length(var.security_group_ids) > 0
      error_message = "At least one security group is required. Set var.create_security_group to true or provide security group IDs via var.security_group_ids."
    }

    precondition {
      condition     = length(var.ecs_service_subnets) > 0
      error_message = "At least one subnet must be provided for the Teleport agent ECS deployment."
    }

    precondition {
      condition = alltrue([
        for subnet in data.aws_subnet.teleport_agent :
        subnet.vpc_id == var.vpc_id
      ])
      error_message = "Each Teleport agent subnet must belong to the configured vpc_id."
    }
  }
}

resource "aws_security_group" "teleport_agent" {
  count = local.create_security_group ? 1 : 0

  description = "Teleport agent security group for ${var.vpc_id}."
  name_prefix = var.ecs_service_name
  tags        = var.apply_aws_tags
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_egress_rule" "allow_all_outbound_from_teleport_agent" {
  count = local.create_security_group ? 1 : 0

  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  security_group_id = one(aws_security_group.teleport_agent[*].id)
}
