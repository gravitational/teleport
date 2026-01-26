# AWS Discovery Terraform module

This Terraform module creates the AWS and Teleport cluster resources necessary for a Teleport cluster to discover resources in AWS:

- AWS IAM role for Teleport Discovery Service to assume. 
- AWS IAM policy attached to the IAM role that grants the AWS permissions necessary for Teleport to discover resources in AWS. 
- AWS OIDC Provider for Teleport Discovery Service to assume an IAM role using OIDC. This resource is optional - creation can be disabled using `create_aws_iam_openid_connect_provider = false`. This resource is optional to support two scenarios:
  - When there is already an AWS IAM OIDC provider in the AWS account configured to use your Teleport cluster's proxy URL. AWS restricts AWS IAM OIDC providers to one per unique URL, so if you are managing that provider already then this module cannot create another one for the same Teleport cluster.
  - When AWS IAM OIDC federation is not possible because your Teleport cluster's proxy URL is not reachable. In this case you should configure AWS IAM role credentials for your Teleport Discovery Service instances and set `discovery_service_iam_credential_source` to trust that role.
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
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.5.7 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | >= 5.0 |
| <a name="requirement_http"></a> [http](#requirement\_http) | >= 3.0 |
| <a name="requirement_teleport"></a> [teleport](#requirement\_teleport) | >= 18.5.1 |
| <a name="requirement_tls"></a> [tls](#requirement\_tls) | >= 4.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | >= 5.0 |
| <a name="provider_http"></a> [http](#provider\_http) | >= 3.0 |
| <a name="provider_teleport"></a> [teleport](#provider\_teleport) | >= 18.5.1 |
| <a name="provider_tls"></a> [tls](#provider\_tls) | >= 4.0 |

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
| [aws_iam_policy_document.teleport_discovery_service_iam_role_trust](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.teleport_discovery_service_single_account](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [http_http.teleport_ping](https://registry.terraform.io/providers/hashicorp/http/latest/docs/data-sources/http) | data source |
| [tls_certificate.teleport_proxy](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/certificate) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apply_aws_tags"></a> [apply\_aws\_tags](#input\_apply\_aws\_tags) | Additional AWS tags to apply to all created AWS resources. | `map(string)` | `{}` | no |
| <a name="input_apply_teleport_resource_labels"></a> [apply\_teleport\_resource\_labels](#input\_apply\_teleport\_resource\_labels) | Additional Teleport resource labels to apply to all created Teleport. | `map(string)` | `{}` | no |
| <a name="input_aws_iam_policy_document"></a> [aws\_iam\_policy\_document](#input\_aws\_iam\_policy\_document) | Override the AWS IAM policy document attached to the AWS IAM role for resource discovery. | `string` | `""` | no |
| <a name="input_aws_iam_policy_name"></a> [aws\_iam\_policy\_name](#input\_aws\_iam\_policy\_name) | Name for the AWS IAM policy for discovery. | `string` | `"teleport-discovery"` | no |
| <a name="input_aws_iam_policy_use_name_prefix"></a> [aws\_iam\_policy\_use\_name\_prefix](#input\_aws\_iam\_policy\_use\_name\_prefix) | Determines whether the name of the AWS IAM policy (`aws_iam_policy_name`) is used as a prefix. | `bool` | `true` | no |
| <a name="input_aws_iam_role_name"></a> [aws\_iam\_role\_name](#input\_aws\_iam\_role\_name) | Name for the AWS IAM role for discovery. | `string` | `"teleport-discovery"` | no |
| <a name="input_aws_iam_role_use_name_prefix"></a> [aws\_iam\_role\_use\_name\_prefix](#input\_aws\_iam\_role\_use\_name\_prefix) | Determines whether the name of the AWS IAM role (`aws_iam_role_name`) is used as a prefix. | `bool` | `true` | no |
| <a name="input_create"></a> [create](#input\_create) | Toggle creation of all resources. | `bool` | `true` | no |
| <a name="input_create_aws_iam_openid_connect_provider"></a> [create\_aws\_iam\_openid\_connect\_provider](#input\_create\_aws\_iam\_openid\_connect\_provider) | Toggle AWS IAM OIDC provider creation. If false and using OIDC, then the AWS IAM OIDC provider must already exist. | `bool` | `true` | no |
| <a name="input_discovery_service_iam_credential_source"></a> [discovery\_service\_iam\_credential\_source](#input\_discovery\_service\_iam\_credential\_source) | Configure the AWS credential source for Teleport Discovery Service instances. The default uses AWS OIDC integration. | <pre>object({<br/>    use_oidc_integration = optional(bool)<br/>    trust_role = optional(object({<br/>      role_arn    = string<br/>      external_id = optional(string, "")<br/>    }))<br/>  })</pre> | <pre>{<br/>  "trust_role": null,<br/>  "use_oidc_integration": true<br/>}</pre> | no |
| <a name="input_match_aws_regions"></a> [match\_aws\_regions](#input\_match\_aws\_regions) | AWS regions to discover. The default matches all AWS regions. | `list(string)` | <pre>[<br/>  "*"<br/>]</pre> | no |
| <a name="input_match_aws_resource_types"></a> [match\_aws\_resource\_types](#input\_match\_aws\_resource\_types) | AWS resource types to match when discovering resources with Teleport. Valid values are: `ec2`. | `list(string)` | n/a | yes |
| <a name="input_match_aws_tags"></a> [match\_aws\_tags](#input\_match\_aws\_tags) | AWS resource tags to match when discovering resources with Teleport. The default matches all discovered AWS resources. | `map(list(string))` | <pre>{<br/>  "*": [<br/>    "*"<br/>  ]<br/>}</pre> | no |
| <a name="input_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#input\_teleport\_discovery\_config\_name) | Name for the `teleport_discovery_config` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_discovery_config_use_name_prefix"></a> [teleport\_discovery\_config\_use\_name\_prefix](#input\_teleport\_discovery\_config\_use\_name\_prefix) | Determines whether the name of the Teleport discovery config (`teleport_discovery_config_name`) is used as a prefix. | `bool` | `true` | no |
| <a name="input_teleport_discovery_group_name"></a> [teleport\_discovery\_group\_name](#input\_teleport\_discovery\_group\_name) | Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use "cloud-discovery-group". | `string` | n/a | yes |
| <a name="input_teleport_integration_name"></a> [teleport\_integration\_name](#input\_teleport\_integration\_name) | Name for the `teleport_integration` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_integration_use_name_prefix"></a> [teleport\_integration\_use\_name\_prefix](#input\_teleport\_integration\_use\_name\_prefix) | Determines whether the name of the Teleport integration (`teleport_integration_name`) is used as a prefix. | `bool` | `true` | no |
| <a name="input_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#input\_teleport\_provision\_token\_name) | Name for the `teleport_provision_token` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_provision_token_use_name_prefix"></a> [teleport\_provision\_token\_use\_name\_prefix](#input\_teleport\_provision\_token\_use\_name\_prefix) | Determines whether the name of the Teleport provision token (`teleport_provision_token_name`) is used as a prefix. | `bool` | `true` | no |
| <a name="input_teleport_proxy_public_addr"></a> [teleport\_proxy\_public\_addr](#input\_teleport\_proxy\_public\_addr) | Teleport cluster proxy public address. | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_aws_oidc_provider_arn"></a> [aws\_oidc\_provider\_arn](#output\_aws\_oidc\_provider\_arn) | AWS resource name (ARN) of the AWS OpenID Connect (OIDC) provider that allows Teleport Discovery Service to assume an AWS IAM role using OIDC. |
| <a name="output_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#output\_teleport\_discovery\_config\_name) | Name of the Teleport dynamic `discovery_config`. Configuration details can be viewed with `tctl get discovery_config/<name>`. Teleport Discovery Service instances will use this `discovery_config` if they are in the same discovery group as the `discovery_config`. |
| <a name="output_teleport_discovery_service_iam_policy_arn"></a> [teleport\_discovery\_service\_iam\_policy\_arn](#output\_teleport\_discovery\_service\_iam\_policy\_arn) | AWS resource name (ARN) of the AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS. |
| <a name="output_teleport_discovery_service_iam_role_arn"></a> [teleport\_discovery\_service\_iam\_role\_arn](#output\_teleport\_discovery\_service\_iam\_role\_arn) | AWS resource name (ARN) of the AWS IAM role that Teleport Discovery Service will assume. |
| <a name="output_teleport_integration_name"></a> [teleport\_integration\_name](#output\_teleport\_integration\_name) | Name of the Teleport `integration` resource. The integration resource configures Teleport Discovery Service instances to assume an AWS IAM role for discovery using AWS OIDC federation. Integration details can be viewed with `tctl get integrations/<name>` or by visiting the Teleport web UI under 'Zero Trust Access' > 'Integrations'. |
| <a name="output_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#output\_teleport\_provision\_token\_name) | Name of the Teleport provision `token` that allows Teleport nodes to join the Teleport cluster using AWS IAM credentials. Token details can be viewed with `tctl get token/<name>`. |
<!-- END_TF_DOCS -->
