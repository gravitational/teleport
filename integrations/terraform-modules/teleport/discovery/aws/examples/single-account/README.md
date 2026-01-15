# Teleport AWS Account Discovery Example

Configuration in this directory creates AWS and Teleport resources necessary for Teleport to discover resources in a single AWS account.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | >= 5.0 |
| <a name="requirement_teleport"></a> [teleport](#requirement\_teleport) | >= 18.5.1 |
| <a name="requirement_tls"></a> [tls](#requirement\_tls) | >= 4.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | >= 5.0 |

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_aws_discovery"></a> [aws\_discovery](#module\_aws\_discovery) | ../.. | n/a |

## Resources

| Name | Type |
|------|------|
| [aws_iam_policy_document.teleport_discovery_service_single_account](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |

## Inputs

No inputs.

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_aws_discovery"></a> [aws\_discovery](#output\_aws\_discovery) | n/a |
<!-- END_TF_DOCS -->
