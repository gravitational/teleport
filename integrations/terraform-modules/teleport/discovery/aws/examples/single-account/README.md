# Teleport AWS Account Discovery Example

Configuration in this directory creates AWS and Teleport resources necessary for Teleport to discover resources in a single AWS account.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.0 |
| aws | >= 5.0 |
| teleport | >= 18.5.1 |
| tls | >= 4.0 |

## Providers

| Name | Version |
|------|---------|
| aws | >= 5.0 |

## Modules

| Name | Source | Version |
|------|--------|---------|
| aws\_discovery | ../.. | n/a |

## Resources

| Name | Type |
|------|------|
| [aws_iam_policy_document.teleport_discovery_service_single_account](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |

## Inputs

No inputs.

## Outputs

| Name | Description |
|------|-------------|
| aws\_discovery | n/a |
<!-- END_TF_DOCS -->
