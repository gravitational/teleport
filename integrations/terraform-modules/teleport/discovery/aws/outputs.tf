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
  value       = try(nonsensitive(teleport_provision_token.aws_iam[0].metadata.name), null)
}

output "teleport_organization_account_enumeration_iam_policy_arn" {
  description = "AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to enumerate accounts in the AWS organization. Only set when aws_organization_discovery is configured."
  value       = one(aws_iam_policy.teleport_organization_account_enumeration[*].arn)
}

output "teleport_organization_join_validation_iam_policy_arn" {
  description = "AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for the Teleport Auth Service to validate IAM-method join attempts against the organization. Only set when aws_organization_discovery is configured."
  value       = one(aws_iam_policy.teleport_organization_join_validation[*].arn)
}

output "aws_child_account_iam_role_template" {
  description = "Create this AWS IAM Role in each Organization's child account, so that the Discovery Service can assume it to discover resources in those accounts."
  value = local.create && local.organization_deployment ? {
    role_name          = var.aws_child_account_iam_role_name,
    assume_role_policy = data.aws_iam_policy_document.allow_assume_role_for_child_accounts[0].json,
    policy             = data.aws_iam_policy_document.teleport_discovery_service_single_account[0].json,
  } : null
}

output "teleport_organization_account_enumeration_iam_role_template" {
  description = "Create this AWS IAM Role in the management account, then provide credentials that can assume it to the Discovery Service (e.g., via the AWS_* environment variables)."
  value = local.create && local.organization_discovery_without_integration ? {
    role_name = "teleport-organization-account-enumeration",
    policy = coalesce(
      var.aws_organization_iam_policies.account_enumeration.document,
      data.aws_iam_policy_document.teleport_organization_account_enumeration[0].json,
    )
  } : null
}

output "teleport_organization_join_validation_iam_role_template" {
  description = "Create this AWS IAM Role in the management account, then provide credentials that can assume it to the Auth Service (e.g., via the AWS_* environment variables)."
  value = local.create && local.organization_discovery_without_integration ? {
    role_name = "teleport-organization-join-validation",
    policy = coalesce(
      var.aws_organization_iam_policies.join_validation.document,
      data.aws_iam_policy_document.teleport_organization_join_validation[0].json,
    )
  } : null
}
