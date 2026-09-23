resource "teleport_provision_token" "agent_aws_iam" {
  count = local.database_service_config_enabled && var.create && var.join_params == null ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow the Teleport database agent to join the cluster using AWS IAM credentials."
  }
  spec = {
    allow = [{
      aws_account = one(data.aws_caller_identity.this[*].account_id)
      aws_arn     = local.teleport_provision_token_allow_aws_arn
    }]
    join_method = "iam"
    roles       = ["Db"]
  }
  version = "v2"
}
