# Azure Discovery Terraform module

This Terraform module creates the Azure and Teleport cluster resources necessary for a Teleport cluster to discover Azure virtual machines:

- **Azure user-assigned managed identity**: Used by the Teleport Discovery Service to authenticate to Azure APIs for scanning and managing VMs in the specified resource groups.
- **Azure federated identity credential**: Establishes trust between Azure and your Teleport cluster by allowing the managed identity to authenticate using OIDC tokens issued by your Teleport proxy.
- **Azure custom role definition and assignment**: Grants the managed identity the minimum required permissions to discover VMs and run installation commands on them.
- **Teleport `discovery_config` cluster resource**: Configures the discovery parameters (subscriptions, resource groups, tags) that determine which Azure VMs will be discovered and enrolled.
- **Teleport `integration` cluster resource**: Stores the Azure OIDC integration configuration in your Teleport cluster, linking the Azure tenant and client ID to enable authentication.
- **Teleport `token` cluster resource**: Provides the join token that discovered Azure VMs will use to authenticate and join your Teleport cluster.

## Prerequisites

- [Install Teleport Terraform Provider](https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/)
-  Every Azure VM to be discovered must have a managed identity assigned to it with at least the Microsoft.Compute/virtualMachines/read permission. [Read more](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-discovery/#step-35-set-up-managed-identities-for-discovered-nodes)

## Examples

- [Discover VMs in a single Azure subscription](./examples/single-subscription)

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement_terraform) | >= 1.6.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement_azurerm) | ~> 4.0 |
| <a name="requirement_teleport"></a> [teleport](#requirement_teleport) | ~> 18.7 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider_azurerm) | ~> 4.0 |
| <a name="provider_teleport"></a> [teleport](#provider_teleport) | ~> 18.7 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azurerm_federated_identity_credential.teleport](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/federated_identity_credential) | resource |
| [azurerm_role_assignment.teleport_discovery_assignment](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_definition.teleport_discovery](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_definition) | resource |
| [azurerm_user_assigned_identity.teleport](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/user_assigned_identity) | resource |
| teleport_discovery_config.azure_teleport | resource |
| teleport_integration.azure_oidc | resource |
| teleport_provision_token.azure_token | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_discovery_group_name"></a> [discovery_group_name](#input_discovery_group_name) | Teleport discovery group name. | `string` | `"cloud-discovery-group"` | no |
| <a name="input_discovery_resource_group_names"></a> [discovery_resource_group_names](#input_discovery_resource_group_names) | Resource groups to scan for VMs. | `list(string)` | n/a | yes |
| <a name="input_discovery_tags"></a> [discovery_tags](#input_discovery_tags) | Tag filters for VM discovery; matches VMs with these tags. | `map(list(string))` | <pre>{<br/>  "*": [<br/>    "*"<br/>  ]<br/>}</pre> | no |
| <a name="input_identity_resource_group_name"></a> [identity_resource_group_name](#input_identity_resource_group_name) | Resource group to place identity resources; defaults to first discovery RG when empty. | `string` | `""` | no |
| <a name="input_integration_name_override"></a> [integration_name_override](#input_integration_name_override) | Override for Teleport integration name; empty to derive from prefix. | `string` | `""` | no |
| <a name="input_installer_script_name"></a> [installer_script_name](#input_installer_script_name) | Name of the Teleport installer script to use. | `string` | `"default-installer"` | no |
| <a name="input_prefix"></a> [prefix](#input_prefix) | Name prefix for created resources. | `string` | `"teleport"` | no |
| <a name="input_proxy_addr"></a> [proxy_addr](#input_proxy_addr) | Teleport proxy address (host:port). | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input_region) | Azure region for created resources (identities). | `string` | n/a | yes |
| <a name="input_subscription_id"></a> [subscription_id](#input_subscription_id) | Azure subscription ID for discovery scope. | `string` | n/a | yes |
| <a name="input_tags"></a> [tags](#input_tags) | Tags applied to Azure resources created by the module. | `map(string)` | `{}` | no |
| <a name="input_tenant_id"></a> [tenant_id](#input_tenant_id) | Azure AD tenant ID. | `string` | n/a | yes |
| <a name="input_token_name_override"></a> [token_name_override](#input_token_name_override) | Override for Teleport provision token name; empty to derive from prefix. | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_client_id"></a> [client_id](#output_client_id) | Client ID used by the Teleport Azure OIDC integration. |
| <a name="output_integration_name"></a> [integration_name](#output_integration_name) | Teleport integration resource name. |
| <a name="output_managed_identity_id"></a> [managed_identity_id](#output_managed_identity_id) | Managed identity resource ID. |
| <a name="output_principal_id"></a> [principal_id](#output_principal_id) | Principal ID used for role assignment. |
| <a name="output_role_assignment_id"></a> [role_assignment_id](#output_role_assignment_id) | ID of the role assignment granting discovery permissions. |
| <a name="output_role_definition_id"></a> [role_definition_id](#output_role_definition_id) | ID of the custom role definition. |
| <a name="output_token_name"></a> [token_name](#output_token_name) | Teleport provision token name. |
<!-- END_TF_DOCS -->
