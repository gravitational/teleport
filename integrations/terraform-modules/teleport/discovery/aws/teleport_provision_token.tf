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
    allow = [{
      aws_account = local.aws_account_id
    }]
    join_method = "iam"
    roles       = ["Node"]
  }
  version = "v2"
}
