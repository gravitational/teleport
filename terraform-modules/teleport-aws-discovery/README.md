# AWS Discovery Terraform module

This Terraform module creates the AWS and Teleport cluster resources necessary for a Teleport cluster to discover resources in AWS:

- AWS IAM role for Teleport Discovery Service to assume. 
- AWS IAM policy attached to the IAM role that grants the AWS permissions necessary for Teleport to discover resources in AWS. 
- AWS OIDC Provider for Teleport Discovery Service to assume an IAM role using OIDC.
- Teleport `discovery_config` cluster resource that configures Teleport for AWS resource discovery.
- Teleport `integration` cluster resource for AWS OIDC.
- Teleport `token` cluster resource that allows Teleport nodes to use AWS IAM credentials to join the cluster.

## Prerequisites

- [Install Teleport Terraform Provider](https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/)

## Examples

- [Discover resources in a single AWS account](./examples/single-account)

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.0 |
| <a name="requirement_teleport"></a> [teleport](#requirement\_teleport) | >= 18.4.0 |
| <a name="requirement_tls"></a> [tls](#requirement\_tls) | ~> 4.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.0 |
| <a name="provider_teleport"></a> [teleport](#provider\_teleport) | >= 18.4.0 |
| <a name="provider_tls"></a> [tls](#provider\_tls) | ~> 4.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_iam_openid_connect_provider.teleport](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_openid_connect_provider) | resource |
| [aws_iam_policy.teleport_discovery_service](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_policy) | resource |
| [aws_iam_role.teleport_discovery_service](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy_attachment.teleport_discovery_service](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| teleport_discovery_config.aws | resource |
| teleport_integration.aws_oidc | resource |
| teleport_provision_token.aws_iam | resource |
| [aws_caller_identity.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [tls_certificate.teleport_proxy](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/certificate) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_create"></a> [create](#input\_create) | Toggle resource creation. | `bool` | `true` | no |
| <a name="input_exclude_aws_organizational_units"></a> [exclude\_aws\_organizational\_units](#input\_exclude\_aws\_organizational\_units) | AWS organizational units (OU) to exclude from Teleport discovery. The default does not exclude any OUs. | `list(string)` | `[]` | no |
| <a name="input_include_aws_organizational_units"></a> [include\_aws\_organizational\_units](#input\_include\_aws\_organizational\_units) | AWS Organizational Units (OU) to include in Teleport AWS Organization discovery. The default matches all AWS OUs. | `list(string)` | <pre>[<br/>  "*"<br/>]</pre> | no |
| <a name="input_match_aws_regions"></a> [match\_aws\_regions](#input\_match\_aws\_regions) | AWS regions to discover. The default matches all AWS regions. | `list(string)` | <pre>[<br/>  "*"<br/>]</pre> | no |
| <a name="input_match_aws_tags"></a> [match\_aws\_tags](#input\_match\_aws\_tags) | AWS resource tags to match when registering discovered resources with Teleport. The default matches all discovered AWS resources. | `map(list(string))` | <pre>{<br/>  "*": [<br/>    "*"<br/>  ]<br/>}</pre> | no |
| <a name="input_name_prefix"></a> [name\_prefix](#input\_name\_prefix) | Prefix to include in resource names. | `string` | `""` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags to apply to AWS resources. | `map(string)` | `{}` | no |
| <a name="input_teleport_cluster_name"></a> [teleport\_cluster\_name](#input\_teleport\_cluster\_name) | Teleport cluster name. | `string` | n/a | yes |
| <a name="input_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#input\_teleport\_discovery\_config\_name) | Teleport discovery config name to use instead of a generated name. | `string` | `""` | no |
| <a name="input_teleport_discovery_group_name"></a> [teleport\_discovery\_group\_name](#input\_teleport\_discovery\_group\_name) | Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use "cloud-discovery-group". | `string` | n/a | yes |
| <a name="input_teleport_discovery_service_iam_policy_name"></a> [teleport\_discovery\_service\_iam\_policy\_name](#input\_teleport\_discovery\_service\_iam\_policy\_name) | Teleport discovery AWS IAM policy name to use instead of a generated name. | `string` | `""` | no |
| <a name="input_teleport_discovery_service_iam_role_name"></a> [teleport\_discovery\_service\_iam\_role\_name](#input\_teleport\_discovery\_service\_iam\_role\_name) | Teleport discovery AWS IAM role name to use instead of a generated name. | `string` | `""` | no |
| <a name="input_teleport_integration_name"></a> [teleport\_integration\_name](#input\_teleport\_integration\_name) | Teleport integration name to use instead of a generated name. | `string` | `""` | no |
| <a name="input_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#input\_teleport\_provision\_token\_name) | Teleport provisioning token name to use instead of a generated name. | `string` | `""` | no |
| <a name="input_teleport_proxy_public_addr"></a> [teleport\_proxy\_public\_addr](#input\_teleport\_proxy\_public\_addr) | Teleport cluster proxy public address. | `string` | n/a | yes |
| <a name="input_teleport_resource_labels"></a> [teleport\_resource\_labels](#input\_teleport\_resource\_labels) | Additional labels to apply to Teleport cluster resources. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_aws_oidc_provider_arn"></a> [aws\_oidc\_provider\_arn](#output\_aws\_oidc\_provider\_arn) | AWS resource name (ARN) of the AWS OpenID Connect (OIDC) provider that allows Teleport Discovery Service to assume an AWS IAM role using OIDC. |
| <a name="output_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#output\_teleport\_discovery\_config\_name) | Name of the Teleport discovery config. |
| <a name="output_teleport_discovery_service_iam_policy_arn"></a> [teleport\_discovery\_service\_iam\_policy\_arn](#output\_teleport\_discovery\_service\_iam\_policy\_arn) | AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS. |
| <a name="output_teleport_discovery_service_iam_role_arn"></a> [teleport\_discovery\_service\_iam\_role\_arn](#output\_teleport\_discovery\_service\_iam\_role\_arn) | AWS resource name (ARN) of the AWS IAM role that Teleport Discovery Service will assume. |
| <a name="output_teleport_integration_name"></a> [teleport\_integration\_name](#output\_teleport\_integration\_name) | Name of the Teleport integration. |
| <a name="output_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#output\_teleport\_provision\_token\_name) | Name of the Teleport provision token that allows Teleport nodes to join the Teleport cluster using AWS IAM credentials. |
<!-- END_TF_DOCS -->
