################################################################################
# ECS Cluster
################################################################################

resource "aws_ecs_cluster" "teleport_agent" {
  count = var.create ? 1 : 0

  name = (
    var.ecs_cluster_use_name_prefix
    ? format("%v-%v",
      var.ecs_cluster_name,
      one(random_string.name_suffix[*].result),
    )
    : var.ecs_cluster_name
  )
  tags = var.apply_aws_tags
}
