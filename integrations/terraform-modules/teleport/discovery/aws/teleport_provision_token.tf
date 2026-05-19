################################################################################
# Teleport cluster provision token for AWS
################################################################################

locals {
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? "${var.teleport_provision_token_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_provision_token_name
  )
}

resource "teleport_provision_token" "aws_iam" {
  count = local.create && local.uses_ec2 ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow Teleport nodes to join the cluster using AWS IAM credentials."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    integration = (
      local.organization_discovery_with_integration ?
      try(teleport_integration.aws_oidc[0].metadata.name, local.teleport_integration_name) :
      null
    )
    allow = [{
      aws_account              = (local.single_account_deployment ? local.aws_account_id : null)
      aws_organization_id      = (local.organization_deployment ? local.aws_organization_id : null)
      aws_organizational_units = (local.organization_deployment ? var.aws_organization_discovery.organizational_units : null)
    }]
    join_method = "iam"
    roles       = ["Node"]
  }
  version = "v2"
}
