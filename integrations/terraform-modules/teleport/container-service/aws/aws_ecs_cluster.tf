################################################################################
# ECS Cluster
################################################################################

resource "aws_ecs_cluster" "teleport_agent" {
  count = var.create ? 1 : 0

  name = var.ecs_cluster_name
  tags = var.apply_aws_tags
}
