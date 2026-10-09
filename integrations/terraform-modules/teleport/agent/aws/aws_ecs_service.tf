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
  region               = var.region
  tags                 = var.apply_aws_tags
  task_definition      = one(aws_ecs_task_definition.teleport_agent[*].arn)

  network_configuration {
    # Public subnets: tasks need a public IP for IGW egress.
    # Private subnets: tasks egress via the NAT route instead.
    assign_public_ip = local.assign_public_ip
    security_groups = concat(
      aws_security_group.teleport_agent[*].id,
      var.security_group_ids,
    )
    subnets = local.ecs_service_subnets
  }

  lifecycle {
    precondition {
      condition     = var.create_security_group || length(var.security_group_ids) > 0
      error_message = "At least one security group is required. Set var.create_security_group to true or provide security group IDs via var.security_group_ids."
    }

    precondition {
      condition     = var.create_security_group || length(var.database_service.database_security_group_ids) == 0
      error_message = "create_security_group must be true when database_service.database_security_group_ids is set because their ingress rules reference the module-created Teleport Database Service security group."
    }

    precondition {
      condition     = length(local.ecs_service_subnets) > 0
      error_message = "Teleport services require internet access, but no private subnets with a route to an AWS NAT gateway or public subnets with a route to an AWS internet gateway were found for VPC ${var.vpc_id}. Add an internet route or provide private_subnet_ids or public_subnet_ids."
    }
  }
}

resource "aws_security_group" "teleport_agent" {
  count = local.create_security_group ? 1 : 0

  description = "Teleport security group for ${var.vpc_id}."
  name_prefix = var.ecs_service_name
  region      = var.region
  tags        = var.apply_aws_tags
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_egress_rule" "allow_all_outbound_from_teleport_agent" {
  count = local.create_security_group ? 1 : 0

  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  region            = var.region
  security_group_id = one(aws_security_group.teleport_agent[*].id)
}
