## Deploy the Teleport Database Service

This example uses a single AWS provider configuration to deploy a VPC and
Teleport Database Service to AWS ECS in both `us-east-1` and `us-west-1`. It
provisions NAT gateways for each VPC, passes their private subnet outputs to the
corresponding agent module instance, and joins both Database Services to an
example Teleport cluster using IAM join tokens.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| aws | ~> 6.0 |
| http | ~> 3.0 |
| teleport | ~> 18.5 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | ~> 6.0 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| teleport\_database\_service | ../.. | n/a |
| vpc | terraform-aws-modules/vpc/aws | 6.6.0 |

## Resources

| Name | Type |
| ---- | ---- |
| [aws_availability_zones.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/availability_zones) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| teleport\_proxy\_addr | The address of the Teleport Proxy Service in host:port form. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| teleport\_database\_services | n/a |
<!-- END_TF_DOCS -->
