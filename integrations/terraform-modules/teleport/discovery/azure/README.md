# Azure Discovery Terraform module

This Terraform module creates the Azure and Teleport cluster resources necessary for a Teleport cluster to discover Azure virtual machines:

- **Azure user-assigned managed identity**: Used by the Teleport Discovery Service to authenticate to Azure APIs for scanning and managing VMs in matching Azure resource groups.
- **Azure federated identity credential**: Establishes trust between Azure and your Teleport cluster by allowing the managed identity to authenticate using OIDC tokens issued by your Teleport proxy.
- **Azure custom role definition and assignment**: Grants the managed identity the minimum required permissions to discover VMs and run installation commands on them.
- **Teleport `discovery_config` cluster resource**: Configures the discovery parameters (subscriptions, resource groups, tags) that determine which Azure VMs will be discovered and enrolled.
- **Teleport `integration` cluster resource**: Stores the Azure OIDC integration configuration in your Teleport cluster, linking the Azure tenant and client ID to enable authentication.
- **Teleport `token` cluster resource**: Provides the join token that discovered Azure VMs will use to authenticate and join your Teleport cluster.

## Prerequisites

- [Install Teleport Terraform Provider](https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/)
- Every Azure VM to be discovered must have a managed identity assigned to it with at least the Microsoft.Compute/virtualMachines/read permission. [Read more](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-discovery/#step-35-set-up-managed-identities-for-discovered-nodes)

## Examples

- [Discover VMs in a single Azure subscription](./examples/single-subscription)

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.5.7 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0 |
| <a name="requirement_http"></a> [http](#requirement\_http) | >= 3.0 |
| <a name="requirement_teleport"></a> [teleport](#requirement\_teleport) | >= 18.5.1 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0 |
| <a name="provider_http"></a> [http](#provider\_http) | >= 3.0 |
| <a name="provider_teleport"></a> [teleport](#provider\_teleport) | >= 18.5.1 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azurerm_federated_identity_credential.teleport_discovery_service](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/federated_identity_credential) | resource |
| [azurerm_role_assignment.teleport_discovery](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_definition.teleport_discovery](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_definition) | resource |
| [azurerm_user_assigned_identity.teleport_discovery_service](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/user_assigned_identity) | resource |
| teleport_discovery_config.azure | resource |
| teleport_integration.azure_oidc | resource |
| teleport_provision_token.azure | resource |
| [azurerm_client_config.this](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/client_config) | data source |
| [http_http.teleport_ping](https://registry.terraform.io/providers/hashicorp/http/latest/docs/data-sources/http) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apply_azure_tags"></a> [apply\_azure\_tags](#input\_apply\_azure\_tags) | Additional Azure tags to apply to all created Azure resources. | `map(string)` | `{}` | no |
| <a name="input_apply_teleport_resource_labels"></a> [apply\_teleport\_resource\_labels](#input\_apply\_teleport\_resource\_labels) | Additional Teleport resource labels to apply to all created Teleport resources. | `map(string)` | `{}` | no |
| <a name="input_azure_federated_identity_credential_name"></a> [azure\_federated\_identity\_credential\_name](#input\_azure\_federated\_identity\_credential\_name) | Name of the Azure federated identity credential created for workload identity federation. | `string` | `"teleport-federation"` | no |
| <a name="input_azure_managed_identity_location"></a> [azure\_managed\_identity\_location](#input\_azure\_managed\_identity\_location) | Azure region (location) where the managed identity will be created (e.g., "westus"). | `string` | n/a | yes |
| <a name="input_azure_managed_identity_name"></a> [azure\_managed\_identity\_name](#input\_azure\_managed\_identity\_name) | Name of the Azure user-assigned managed identity created for Teleport Discovery. | `string` | `"discovery-identity"` | no |
| <a name="input_azure_resource_group_name"></a> [azure\_resource\_group\_name](#input\_azure\_resource\_group\_name) | Name of an existing Azure Resource Group where Azure resources will be created. | `string` | n/a | yes |
| <a name="input_azure_role_definition_name"></a> [azure\_role\_definition\_name](#input\_azure\_role\_definition\_name) | Name for the Azure custom role definition created for Teleport Discovery. | `string` | `"teleport-discovery"` | no |
| <a name="input_create"></a> [create](#input\_create) | Toggle creation of all resources. | `bool` | `true` | no |
| <a name="input_match_azure_regions"></a> [match\_azure\_regions](#input\_match\_azure\_regions) | Azure regions to discover. Defaults to ["*"] which matches all regions. Region names should be the programmatic region name, e.g., "westus". | `list(string)` | <pre>[<br/>  "*"<br/>]</pre> | no |
| <a name="input_match_azure_resource_groups"></a> [match\_azure\_resource\_groups](#input\_match\_azure\_resource\_groups) | Azure resource groups to scan for VMs. Defaults to ["*"] which matches all resource groups. | `list(string)` | <pre>[<br/>  "*"<br/>]</pre> | no |
| <a name="input_match_azure_tags"></a> [match\_azure\_tags](#input\_match\_azure\_tags) | Tag filters for VM discovery; matches VMs with these tags. Defaults to {"*" = ["*"]} which matches all tags. | `map(list(string))` | <pre>{<br/>  "*": [<br/>    "*"<br/>  ]<br/>}</pre> | no |
| <a name="input_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#input\_teleport\_discovery\_config\_name) | Name for the `teleport_discovery_config` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_discovery_config_use_name_prefix"></a> [teleport\_discovery\_config\_use\_name\_prefix](#input\_teleport\_discovery\_config\_use\_name\_prefix) | Whether `teleport_discovery_config_name` is used as a name prefix (true) or as the exact name (false). | `bool` | `true` | no |
| <a name="input_teleport_discovery_group_name"></a> [teleport\_discovery\_group\_name](#input\_teleport\_discovery\_group\_name) | Teleport discovery group to use. For discovery configuration to apply, this name must match at least one Teleport Discovery Service instance's configured `discovery_group`. For Teleport Cloud clusters, use "cloud-discovery-group". | `string` | n/a | yes |
| <a name="input_teleport_installer_script_name"></a> [teleport\_installer\_script\_name](#input\_teleport\_installer\_script\_name) | Name of an existing Teleport installer script to use. | `string` | `"default-installer"` | no |
| <a name="input_teleport_integration_name"></a> [teleport\_integration\_name](#input\_teleport\_integration\_name) | Name for the `teleport_integration` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_integration_use_name_prefix"></a> [teleport\_integration\_use\_name\_prefix](#input\_teleport\_integration\_use\_name\_prefix) | Whether `teleport_integration_name` is used as a name prefix (true) or as the exact name (false). | `bool` | `true` | no |
| <a name="input_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#input\_teleport\_provision\_token\_name) | Name for the `teleport_provision_token` resource. | `string` | `"discovery"` | no |
| <a name="input_teleport_provision_token_use_name_prefix"></a> [teleport\_provision\_token\_use\_name\_prefix](#input\_teleport\_provision\_token\_use\_name\_prefix) | Whether `teleport_provision_token_name` is used as a name prefix (true) or as the exact name (false). | `bool` | `true` | no |
| <a name="input_teleport_proxy_public_addr"></a> [teleport\_proxy\_public\_addr](#input\_teleport\_proxy\_public\_addr) | Teleport cluster proxy public address in the form <host:port> (no URL scheme). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_azure_managed_identity_client_id"></a> [azure\_managed\_identity\_client\_id](#output\_azure\_managed\_identity\_client\_id) | Client ID used by the Teleport Azure OIDC integration. |
| <a name="output_azure_managed_identity_id"></a> [azure\_managed\_identity\_id](#output\_azure\_managed\_identity\_id) | Managed identity resource ID. |
| <a name="output_azure_managed_identity_principal_id"></a> [azure\_managed\_identity\_principal\_id](#output\_azure\_managed\_identity\_principal\_id) | Principal ID used for role assignment. |
| <a name="output_azure_role_assignment_id"></a> [azure\_role\_assignment\_id](#output\_azure\_role\_assignment\_id) | ID of the role assignment granting discovery permissions. |
| <a name="output_azure_role_definition_id"></a> [azure\_role\_definition\_id](#output\_azure\_role\_definition\_id) | ID of the discovery role definition. |
| <a name="output_teleport_discovery_config_name"></a> [teleport\_discovery\_config\_name](#output\_teleport\_discovery\_config\_name) | Name of the Teleport dynamic `discovery_config`. Configuration details can be viewed with `tctl get discovery_config/<name>`. Teleport Discovery Service instances will use this `discovery_config` if they are in the same discovery group as the `discovery_config`. |
| <a name="output_teleport_integration_name"></a> [teleport\_integration\_name](#output\_teleport\_integration\_name) | Name of the Teleport `integration` resource. The integration resource configures Teleport Discovery Service instances to assume an Azure managed identity for discovery using Azure OIDC federation. Integration details can be viewed with `tctl get integrations/<name>` or by visiting the Teleport web UI under 'Zero Trust Access' > 'Integrations'. |
| <a name="output_teleport_provision_token_name"></a> [teleport\_provision\_token\_name](#output\_teleport\_provision\_token\_name) | Name of the Teleport provision `token` that allows Teleport nodes to join the Teleport cluster using Azure credentials. Token details can be viewed with `tctl get token/<name>`. |
<!-- END_TF_DOCS -->
