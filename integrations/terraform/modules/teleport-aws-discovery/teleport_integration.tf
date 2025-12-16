################################################################################
# Teleport cluster integration
################################################################################

locals {
  create_teleport_integration = local.create && local.use_oidc_integration
  discovery_aws_iam_role_arn = try(
    aws_iam_role.teleport_discovery_service[0].arn,
    data.aws_iam_role.teleport_discovery_service[0].arn,
    ""
  )
  teleport_integration_name = "${local.name_prefix}${coalesce(
    var.teleport_integration_name,
    local.default_teleport_resource_name,
  )}"
}

resource "teleport_integration" "aws_oidc" {
  count = local.create_teleport_integration ? 1 : 0

  metadata = {
    name        = local.teleport_integration_name
    description = "AWS OIDC integration for AWS discovery."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    aws_oidc = {
      role_arn = local.discovery_aws_iam_role_arn
    }
  }
  sub_kind = "aws-oidc"
  version  = "v1"
}
