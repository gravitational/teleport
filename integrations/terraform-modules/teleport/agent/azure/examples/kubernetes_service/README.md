## Deploy a Kubernetes Service in Azure

This example deploys the Teleport Kubernetes Service to Azure Container Instances and joins it to an example Teleport cluster.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| azurerm | ~> 5.0 |
| http | ~> 3.0 |
| random | ~> 3.0 |
| teleport | ~> 18.11 |

## Providers

| Name | Version |
| ---- | ------- |
| azurerm | ~> 5.0 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| agent | ../.. | n/a |

## Resources

| Name | Type |
| ---- | ---- |
| [azurerm_resource_group.example](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_client_config.this](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/client_config) | data source |

## Inputs

No inputs.

## Outputs

| Name | Description |
| ---- | ----------- |
| teleport\_kubernetes\_service | Kubernetes Service identity, deployment and logs location. |
<!-- END_TF_DOCS -->
