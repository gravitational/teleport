data "aws_caller_identity" "current" {}

resource "aws_ecs_cluster" "teleport_ecs_cluster" {
  name = var.ecs_cluster
}

resource "aws_iam_role" "ecs_teleport_agent_executionrole" {
  name = var.ecs_executionrole
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy" "ecs_teleport_agent_executionrole_logs" {
  name = "WriteLogsToCloudWatch"
  role = aws_iam_role.ecs_teleport_agent_executionrole.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:CreateLogGroup",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
  })
}

resource "aws_iam_role" "ecs_teleport_agent_taskrole" {
  name = var.ecs_taskrole
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy" "ecs_teleport_agent_taskrole" {
  name   = "ECSGuideAgentAPIAccess"
  role   = aws_iam_role.ecs_teleport_agent_taskrole.id
  policy = jsonencode(var.ecs_taskrole_policy)
}

// get teleport version from teleport cluster endpoint
data "http" "teleport_version" {
  url = "https://${var.teleport_proxy_server}/v1/webapi/automaticupgrades/channel/default/version"
}
locals {
  teleport_image = format("public.ecr.aws/gravitational/teleport-ent-distroless:%s",
    trimprefix(data.http.teleport_version.response_body, "v")
  )
}

resource "aws_ecs_task_definition" "teleport_agent_task" {
  family                   = var.teleport_task_family
  requires_compatibilities = ["FARGATE"]
  cpu                      = "2048"
  memory                   = "4096"
  network_mode             = "awsvpc"
  task_role_arn            = aws_iam_role.ecs_teleport_agent_taskrole.arn
  execution_role_arn       = aws_iam_role.ecs_teleport_agent_executionrole.arn
  container_definitions = jsonencode([
    {
      name       = "agent"
      image      = local.teleport_image
      entryPoint = ["/usr/bin/dumb-init"]
      command = [
        "--rewrite",
        "15:3",
        "--",
        "teleport",
        "start",
        "--config-string",
        base64encode(yamlencode(var.teleport_agent_config))
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = format("ecs-%s", aws_ecs_cluster.teleport_ecs_cluster.name)
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "teleport-agent-services"
          "awslogs-create-group"  = "true"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "teleport_agent" {
  name            = "teleport-agent"
  desired_count   = 2
  launch_type     = "FARGATE"
  task_definition = aws_ecs_task_definition.teleport_agent_task.arn
  cluster         = aws_ecs_cluster.teleport_ecs_cluster.id

  network_configuration {
    subnets          = var.teleport_agent_subnets
    security_groups  = var.teleport_agent_security_groups
    assign_public_ip = true
  }
}