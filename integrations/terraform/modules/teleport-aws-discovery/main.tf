locals {
  create = var.create
  name_prefix = (
    var.name_prefix != ""
    ? "${trimsuffix(var.name_prefix, "-")}-"
    : ""
  )
  apply_aws_tags = merge(var.apply_aws_tags, {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    # this is the origin we set for resources created by the AWS OIDC integration web UI wizard.
    "teleport.dev/origin" = "integration_awsoidc"
  })
}
