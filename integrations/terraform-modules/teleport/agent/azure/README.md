## Teleport Agent Deployment on Azure Container Instances

This module deploys a Teleport Agent Service to Azure Container Instances (ACI).

It is not intended to deploy a Teleport cluster or its Auth Service and Proxy Service control plane.
This module does not provision the persistent backend and audit event storage, load balancing, DNS, or TLS certificates required to operate a self-hosted Teleport cluster.

Currently, only the Teleport Kubernetes Service can be deployed.

## Prerequisites
- An existing Teleport cluster with Auth Service and Proxy Service
- A reachable Teleport Proxy Service endpoint
- An existing Azure resource group for the agent resources.
- A Teleport cluster supporting Generic OIDC joining (v18.11.0 or later), and an agent supporting `teleport.join_params.generic_oidc.http_request` (v18.11.2 or later).
<!-- lint ignore absolute-docs-links -->
- [Configure Teleport Terraform Provider](https://goteleport.com/docs/configuration/terraform-provider/)
- [Configure Azure Terraform provider](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs)


## Examples

Refer to the [examples](./examples) for example usage of this module.

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

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
| http | >= 3.0 |
| random | >= 3.0 |
| teleport | >= 18.11.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [azurerm_container_group.teleport_agent](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/container_group) | resource |
| [azurerm_log_analytics_workspace.this](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/log_analytics_workspace) | resource |
| [azurerm_role_assignment.teleport_agent](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_definition.teleport_agent](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_definition) | resource |
| [azurerm_user_assigned_identity.teleport_agent](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/user_assigned_identity) | resource |
| [random_string.name_suffix](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/string) | resource |
| teleport_provision_token.agent | resource |
| [azurerm_client_config.this](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/client_config) | data source |
| [http_http.managed_updates](https://registry.terraform.io/providers/hashicorp/http/latest/docs/data-sources/http) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| apply\_azure\_tags | Additional Azure tags to apply to all created Azure resources. | `map(string)` | `{}` | no |
| container\_group\_cpu | The required number of CPU cores of the containers. | `number` | `2` | no |
| container\_group\_desired\_count | Desired number of Teleport container groups to run. | `number` | `2` | no |
| container\_group\_memory | The required memory of the containers in GB. | `number` | `4` | no |
| container\_group\_name | Name for the container group where the agent runs. | `string` | `"teleport-agent"` | no |
| container\_group\_use\_name\_prefix | Determines whether `var.container_group_name` is used as a prefix of the container group name. The revision and replica index are always appended to container group names. | `bool` | `true` | no |
| create | Toggle creation of all resources. | `bool` | `true` | no |
| environment\_vars | Environment variables to set on the Teleport Azure Container Instance. | `map(string)` | `{}` | no |
| kubernetes\_service | Configuration for running this agent as a Kubernetes Service. `resource_matcher_labels` is required and contains the label selectors to match and proxy Kubernetes resources. | ```object({ enabled = optional(bool, true) resource_matcher_labels = optional(map(list(string)), {}) })``` | n/a | yes |
| location | Azure location for the agent. Example: eastus, westus. | `string` | n/a | yes |
| log\_analytics\_workspace\_name | Name of the Log Analytics workspace for the Teleport containers. | `string` | `"teleport"` | no |
| log\_analytics\_workspace\_retention\_days | Retention of container logs in the created Log Analytics workspace. | `number` | `30` | no |
| log\_analytics\_workspace\_use\_name\_prefix | Determines whether log\_analytics\_workspace\_name is used as a prefix of the Log Analytics workspace name. | `bool` | `true` | no |
| managed\_identity\_role | Custom role for the agent's managed identity. Defaults to permissions for the configured service, scoped to the provider's subscription. Override actions, data\_actions, or scopes as needed; set to null to disable role creation and assignments. | ```object({ actions = optional(list(string)) data_actions = optional(list(string)) scope = optional(string) })``` | `{}` | no |
| managed\_updates\_enabled | Whether to resolve the Teleport version from the configured Managed Updates endpoint when applying this module. | `bool` | `true` | no |
| managed\_updates\_group | Update group to query through the v2 Managed Updates endpoint. | `string` | `"default"` | no |
| resource\_group\_name | Resource group where the resources will be created. Must already exist. | `string` | n/a | yes |
| teleport\_container\_image | Container image used for Teleport Azure Container Instance tasks. | `string` | `"public.ecr.aws/gravitational/teleport-ent-distroless"` | no |
| teleport\_discovery\_group\_name | If specified, this is added as part of the `resource_matcher_labels`, to match the resources discovered in that discovery group. | `string` | `""` | no |
| teleport\_proxy\_public\_addr | Teleport cluster proxy public address `host:port`. | `string` | n/a | yes |
| teleport\_version\_override | teleport\_version\_override is intended for development and MUST NOT be used to control the Teleport version in a typical deployment. This module is designed to run a specific Teleport version. You will face compatibility issues trying to run a different Teleport version with it. If you want to run Teleport version `X.Y.Z`, you should adjust the module version to `X.Y.Z` instead. | `string` | `null` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| container\_group\_ids | Resource IDs of the deployed agents, one per replica. |
| container\_group\_names | Names of the deployed agents, one per replica. |
| container\_group\_urls | Azure portal URLs for the deployment status and container logs of each deployed agent. |
| container\_logs\_follow\_command | The command to follow the logs of the deployed agents' containers. |
| log\_analytics\_workspace\_id | Resource ID of the Log Analytics workspace that collects agent logs. |
| log\_analytics\_workspace\_name | Name of the Log Analytics workspace that collects agent logs. |
| managed\_identity | The user-assigned managed identity used by the deployed agents. |
<!-- END_TF_DOCS -->
