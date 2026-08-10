## Deploy Teleport Database Service to ECS

This example deploys the Teleport Database Service to AWS ECS and joins it to an example Teleport cluster using an IAM join token.

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
| teleport | ~> 18.5 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| teleport\_database\_service | ../.. | n/a |
| vpc | terraform-aws-modules/vpc/aws | 6.6.0 |

## Resources

| Name | Type |
| ---- | ---- |
| teleport_provision_token.iam | resource |
| [aws_availability_zones.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/availability_zones) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| teleport\_proxy\_addr | The address of the Teleport proxy service in host:port form. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| teleport\_database\_service | n/a |
<!-- END_TF_DOCS -->