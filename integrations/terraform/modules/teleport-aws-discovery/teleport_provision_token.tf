################################################################################
# Teleport cluster provision token for AWS
################################################################################

locals {
  apply_teleport_resource_labels  = var.apply_teleport_resource_labels
  aws_account_id                  = try(data.aws_caller_identity.this[0].account_id, "")
  create_teleport_provision_token = local.create
  default_teleport_resource_name  = "aws-account-${local.aws_account_id}"
  teleport_provision_token_name = "${local.name_prefix}${coalesce(
    var.teleport_provision_token_name,
    local.default_teleport_resource_name,
  )}"
}

resource "teleport_provision_token" "aws_iam" {
  count = local.create_teleport_provision_token ? 1 : 0

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
