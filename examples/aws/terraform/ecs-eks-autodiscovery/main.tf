data "aws_caller_identity" "current" {}

resource "aws_ecs_cluster" "teleport_ecs_cluster" {
  name = var.ecs_cluster
}

resource "aws_iam_role" "ecs_teleport_discover_eks_executionrole" {
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

resource "aws_iam_role_policy" "ecs_teleport_discover_eks_executionrole_logs" {
  name = "WriteLogsToCloudWatch"
  role = aws_iam_role.ecs_teleport_discover_eks_executionrole.id

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

resource "aws_iam_role" "ecs_teleport_discover_eks_taskrole" {
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

resource "aws_iam_role_policy" "ecs_teleport_discover_eks_taskrole_access_eks" {
  name = "AccessEKS"
  role = aws_iam_role.ecs_teleport_discover_eks_taskrole.id

  // From https://goteleport.com/docs/enroll-resources/auto-discovery/kubernetes/aws/#step-13-set-up-aws-iam-credentials
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid = "EKSDiscovery"
        Action = [
          "eks:DescribeCluster",
          "eks:ListClusters"
        ]
        Effect   = "Allow"
        Resource = "*"
      },
      {
        Sid = "EKSManageAccess"
        Action = [
          "eks:AssociateAccessPolicy",
          "eks:CreateAccessEntry",
          "eks:DeleteAccessEntry",
          "eks:DescribeAccessEntry",
          "eks:TagResource",
          "eks:UpdateAccessEntry"
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
  })
}

locals {
  discovery_group = "aws-prod"
}

resource "aws_ecs_task_definition" "teleport_discovery_kube_services" {
  family                   = "teleport-discovery-kube-services"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "2048"
  memory                   = "4096"
  network_mode             = "awsvpc"
  task_role_arn            = aws_iam_role.ecs_teleport_discover_eks_taskrole.arn
  execution_role_arn       = aws_iam_role.ecs_teleport_discover_eks_executionrole.arn
  container_definitions = jsonencode([
    {
      name       = "discovery-kube-service"
      image      = var.teleport_image,
      entryPoint = ["/usr/bin/dumb-init"]
      command = [
        "--rewrite",
        "15:3",
        "--",
        "teleport",
        "start",
        "--config-string",
        base64encode(yamlencode({
          version = "v3"
          teleport = {
            join_params = {
              token_name = var.teleport_iam_token_name
              method     = "iam"
            }
            proxy_server = var.teleport_proxy_server
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
            enabled         = "yes"
            discovery_group = local.discovery_group
            aws = [
              {
                types   = ["eks"]
                regions = [var.aws_region]
                tags    = var.discover_eks_tags
              }
            ]
          }
          kubernetes_service = {
            enabled = "yes"
            resources = [
              {
                labels = merge(
                  var.discover_eks_tags, {
                    "region"                                 = var.aws_region
                    "account-id"                             = data.aws_caller_identity.current.account_id
                    "teleport.dev/cloud"                     = "AWS"
                    "teleport.dev/discovery-type"            = "eks"
                    "teleport.internal/discovery-group-name" = local.discovery_group
                })
              }
            ],
          }
        }))
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = format("ecs-%s", aws_ecs_cluster.teleport_ecs_cluster.name)
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "teleport-discovery-kube-services"
          "awslogs-create-group"  = "true"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "discovery_kube_service" {
  name            = "discovery-kube-service"
  desired_count   = 2
  launch_type     = "FARGATE"
  task_definition = aws_ecs_task_definition.teleport_discovery_kube_services.arn
  cluster         = aws_ecs_cluster.teleport_ecs_cluster.id

  network_configuration {
    subnets          = var.teleport_agent_subnets
    security_groups  = var.teleport_agent_security_groups
    assign_public_ip = true
  }
}
