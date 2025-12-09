output "aws_oidc_provider_arn" {
  description = "AWS resource name (ARN) of the AWS OpenID Connect (OIDC) provider that allows Teleport Discovery Service to assume an AWS IAM role using OIDC."
  value       = one(aws_iam_openid_connect_provider.teleport[*].arn)
}

output "teleport_discovery_service_iam_policy_arn" {
  description = "AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  value       = one(aws_iam_policy.teleport_discovery_service[*].arn)
}

output "teleport_discovery_service_iam_role_arn" {
  description = "AWS resource name (ARN) of the AWS IAM role that Teleport Discovery Service will assume."
  value       = one(aws_iam_role.teleport_discovery_service[*].arn)
}

output "teleport_integration_name" {
  description = "Name of the Teleport integration."
  value       = one(teleport_integration.aws_oidc[*].metadata.name)
}

output "teleport_discovery_config_name" {
  description = "Name of the Teleport discovery config."
  value       = one(teleport_discovery_config.aws[*].header.metadata.name)
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision token that allows Teleport nodes to join the Teleport cluster using AWS IAM credentials."
  value       = nonsensitive(one(teleport_provision_token.aws_iam[*].metadata.name))
}
