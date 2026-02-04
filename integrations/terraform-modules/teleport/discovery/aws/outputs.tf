output "aws_oidc_provider_arn" {
  description = "AWS resource name (ARN) of the AWS OpenID Connect (OIDC) provider that allows Teleport Discovery Service to assume an AWS IAM role using OIDC."
  value       = local.aws_iam_oidc_provider_arn
}

output "teleport_discovery_service_iam_policy_arn" {
  description = "AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  value       = one(aws_iam_policy.teleport_discovery_service[*].arn)
}

output "teleport_discovery_service_iam_role_arn" {
  description = "AWS resource name (ARN) of the AWS IAM role that Teleport Discovery Service will assume."
  value       = one(aws_iam_role.teleport_discovery_service[*].arn)
}

output "teleport_discovery_config_name" {
  description = "Name of the Teleport dynamic `discovery_config`. Configuration details can be viewed with `tctl get discovery_config/<name>`. Teleport Discovery Service instances will use this `discovery_config` if they are in the same discovery group as the `discovery_config`."
  value       = try(teleport_discovery_config.aws[0].header.metadata.name, null)
}

output "teleport_integration_name" {
  description = "Name of the Teleport `integration` resource. The integration resource configures Teleport Discovery Service instances to assume an AWS IAM role for discovery using AWS OIDC federation. Integration details can be viewed with `tctl get integrations/<name>` or by visiting the Teleport web UI under 'Zero Trust Access' > 'Integrations'."
  value       = try(teleport_integration.aws_oidc[0].metadata.name, null)
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision `token` that allows Teleport nodes to join the Teleport cluster using AWS IAM credentials. Token details can be viewed with `tctl get token/<name>`."
  value       = nonsensitive(try(teleport_provision_token.aws_iam[0].metadata.name, null))
}

output "iam_role_to_create_in_child_accounts" {
  description = "AWS IAM role that must be created in child AWS accounts under the organization for discovering resources."
  value = local.organization_deployment ? {
    target             = "Create this role in all the accounts under the organization.",
    name               = var.aws_iam_role_name_for_child_accounts,
    assume_role_policy = data.aws_iam_policy_document.allow_assume_role_for_child_accounts.json,
    policy             = data.aws_iam_policy_document.teleport_discovery_service_single_account.json,
  } : null
}