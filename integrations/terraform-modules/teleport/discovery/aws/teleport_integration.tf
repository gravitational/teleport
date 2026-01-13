################################################################################
# Teleport cluster integration
################################################################################

locals {
  teleport_integration_name = (
    var.teleport_integration_use_name_prefix
    ? "${var.teleport_integration_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_integration_name
  )
}

resource "teleport_integration" "aws_oidc" {
  count = local.create ? 1 : 0

  metadata = {
    name        = local.teleport_integration_name
    description = "AWS OIDC integration for AWS discovery."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    aws_oidc = {
      role_arn = one(aws_iam_role.teleport_discovery_service[*].arn)
    }
  }
  sub_kind = "aws-oidc"
  version  = "v1"
}
