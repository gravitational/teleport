locals {
  teleport_provision_token_name = (
    var.teleport_provision_token_use_name_prefix
    ? "${var.teleport_provision_token_name}-${try(data.aws_caller_identity.this[0].account_id, "")}"
    : var.teleport_provision_token_name
  )
}

resource "teleport_provision_token" "agent_aws_iam" {
  count = var.create && var.join_params == null ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow Teleport db agent to join the cluster using AWS IAM credentials."
  }
  spec = {
    allow = [{
      aws_account = try(data.aws_caller_identity.this[0].account_id, "")
      aws_arn     = module.teleport_db_service.teleport_provision_token_allow_aws_arn
    }]
    join_method = "iam"
    roles       = ["Db"]
  }
  version = "v2"
}
