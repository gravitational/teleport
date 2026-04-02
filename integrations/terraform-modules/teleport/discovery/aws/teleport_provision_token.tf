################################################################################
# Teleport cluster provision token for AWS
################################################################################

locals {
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? "${var.teleport_provision_token_name}-${local.teleport_resource_name_suffix}"
    : coalesce(
      var.teleport_provision_token_name,
      local.teleport_resource_name_suffix,
    )
  )
}

resource "teleport_provision_token" "aws_iam" {
  count = local.create ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow Teleport nodes to join the cluster using AWS IAM credentials."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    integration = (local.organization_deployment ? local.teleport_integration_name : null)
    allow = [{
      aws_account         = (local.single_account_deployment ? local.aws_account_id : null)
      aws_organization_id = (local.organization_deployment ? local.aws_organization_id : null)
    }]
    join_method = "iam"
    roles       = ["Node"]
  }
  version = "v2"
}
