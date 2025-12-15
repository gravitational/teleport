output "aws_oidc_provider_arn" {
  description = "AWS resource name (ARN) of the AWS OpenID Connect (OIDC) provider that allows Teleport Discovery Service to assume an AWS IAM role using OIDC."
  value       = local.aws_iam_oidc_provider_arn
}

output "teleport_discovery_service_iam_policy_arn" {
  description = "AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  value       = local.discovery_aws_iam_policy_arn
}

output "teleport_discovery_service_iam_role_arn" {
  description = "AWS resource name (ARN) of the AWS IAM role that Teleport Discovery Service will assume."
  value       = local.discovery_aws_iam_role_arn
}

output "teleport_discovery_config_name" {
  description = "Name of the Teleport discovery config."
  value = try(
    teleport_discovery_config.aws[0].header.metadata.name,
    local.teleport_discovery_config_name,
  )
}

output "teleport_integration_name" {
  description = "Name of the Teleport integration."
  value = try(
    teleport_integration.aws_oidc[0].metadata.name,
    local.teleport_integration_name,
  )
}

output "teleport_provision_token_name" {
  description = "Name of the Teleport provision token that allows Teleport nodes to join the Teleport cluster using AWS IAM credentials."
  value = nonsensitive(try(
    teleport_provision_token.aws_iam[0].metadata.name,
    local.teleport_provision_token_name,
  ))
}
