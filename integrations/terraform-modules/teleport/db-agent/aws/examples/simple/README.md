# Simple AWS Database Agent

This example deploys a Teleport Database Service agent to Amazon ECS in a new VPC. It uses public subnets and requires the address of an existing Teleport proxy.

```shell
terraform init
terraform apply -var='teleport_proxy_addr=teleport.example.com:443'
```

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| aws | ~> 6.0 |
| teleport | ~> 18.8 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | ~> 6.0 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| teleport\_db\_agent | ../.. | n/a |
| vpc | terraform-aws-modules/vpc/aws | 6.6.0 |

## Resources

| Name | Type |
| ---- | ---- |
| [aws_availability_zones.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/availability_zones) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| teleport\_proxy\_addr | The address of the Teleport proxy service in host:port form. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| teleport\_db\_agent | n/a |
<!-- END_TF_DOCS -->