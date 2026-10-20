## Discover and access Azure Kubernetes Service clusters

This example configures AKS discovery and deploys a Teleport Kubernetes Service in Azure Container Instances to access the discovered clusters.

The discovery module creates a managed identity, discovery permissions, and the Teleport OIDC integration and discovery configuration.
The agent module deploys the Kubernetes Service with a separate managed identity and AKS access permissions.

The discovery module uses the tags defined in `azure_matchers` to filter which AKS clusters it registers.
The Kubernetes Service selects registered AKS clusters in the same discovery group, subscription, resource groups, and regions.
This can include matching clusters registered by other discovery configurations in that group.

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

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| azure\_agent | ../../../../agent/azure | n/a |
| azure\_discovery | ../.. | n/a |

## Resources

| Name | Type |
| ---- | ---- |
| [azurerm_resource_group.example](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_client_config.current](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/client_config) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| teleport\_proxy\_addr | The address of the Teleport Proxy Service in host:port form. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| azure\_agent | Kubernetes Service identity, container deployments, and commands to follow their logs. |
| azure\_discovery | Discovery configuration, OIDC integration, and Azure discovery identity. |
<!-- END_TF_DOCS -->
