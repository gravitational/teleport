## Discover and access AKS clusters in a management group

Configure Teleport to discover AKS clusters and provide access to them by deploying the Kubernetes Service.

The Kubernetes Service must be able to reach the AKS cluster.
If you have private AKS clusters, ensure you deploy multiple Kubernetes Service that can reach those AKS clusters using the `subnet_id` to place them.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.5.7 |
| azurerm | >= 5.0 |
| http | >= 3.0 |
| random | >= 3.0 |
| teleport | >= 18.11.0 |

## Providers

| Name | Version |
| ---- | ------- |
| azurerm | >= 5.0 |
| azurerm.private | >= 5.0 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| azure\_agent | ../../../../agent/azure | n/a |
| azure\_agent\_private | ../../../../agent/azure | n/a |
| azure\_discovery | ../.. | n/a |

## Resources

| Name | Type |
| ---- | ---- |
| [azurerm_resource_group.example](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_resource_group.private](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| teleport\_proxy\_addr | The address of the Teleport Proxy Service in host:port form. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| azure\_agent | Public Kubernetes Service identity, replicas, and log commands. |
| azure\_agent\_private | Private Kubernetes Service identity, replicas, and log commands. |
| azure\_discovery | AKS discovery configuration, OIDC integration, and Azure discovery identity. |
<!-- END_TF_DOCS -->
