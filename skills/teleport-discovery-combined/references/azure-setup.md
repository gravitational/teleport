# Azure Discovery Setup

State these to the user:
- The Azure credentials running Terraform need permission to create a managed identity, a custom role definition, and a role assignment in the target subscription.
- Each VM to be discovered must have a managed identity assigned with at least the `Microsoft.Compute/virtualMachines/read` permission, or it will not enroll.

## Gather requirements

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `proxy_addr` | `$TSH status --format=json`, `active.profile_url` with the `https://` scheme stripped, such as `example.teleport.sh:443` | Ask |
| `cluster_version` | `$TCTL status` `Version` field, such as `18.8.0` | Ask |
| `deployment` | `cloud` when `proxy_addr`'s host ends in `.teleport.sh` or `.cloud.gravitational.io`, else `self-hosted` | none |
| `subscriptions` | `az account show --query id --output tsv` for the current subscription | Ask |
| `resource_group` | none | Ask |
| `location`, `create_resource_group` | `az group show --name <resource_group> --query location --output tsv`: on success set `location` to the output and `create_resource_group=false`; on failure set `create_resource_group=true` | Ask for `location`; set `create_resource_group=true` |
| `regions` | none | Omit; `["*"]` matches all |
| `resource_groups` | none | Omit; `["*"]` matches all |
| `tags` | none | Omit; `{"*": ["*"]}` matches all |
| `discovery_group` | `cloud`: `cloud-discovery-group`. `self-hosted`: confirm a service runs with `$TCTL inventory list --services=discovery`, and stop if none runs | Ask for the `discovery_group` set in the Discovery Service's `teleport.yaml` |
| `write_location` | none | A new `teleport-discovery-azure/` directory |

Azure discovery requires the Teleport provider `>= 18.7.6`. If `cluster_version` is below
`18.7.6`, stop: "Azure discovery requires Teleport 18.7.6 or later. This cluster is
v`<cluster_version>`."

Validate that every subscription ID is a UUID of the form
`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`. If any fails, ask the user to correct it.

## Write location

Into a new project, write a fresh module: a new `teleport-discovery-azure/` directory with
`versions.tf` and `main.tf`. Into an existing Terraform project, integrate following its
structure. If the project already declares the `module "azure_discovery"` block, read it,
pre-populate the gathered fields from its current values, and edit that block in place.

## Show the plan

Present this with real values, then wait for approval unless the request set
`auto_approve: true`.

```
## Environment
Cluster:         <deployment>, <proxy_addr>, v<cluster_version>
Subscriptions:   <subscriptions>
Managed identity resource group: <resource_group>, <create new | existing>, location <location>
Discovery group: <discovery_group>
Write location:  <path>, <new project | extend existing>

## Matchers
types=vm; subscriptions=<...>; regions=<... or all>; resource_groups=<... or all>; tags=<... or all>

## Plan
Teleport resources: azure-oidc integration, discovery_config, provision token
Azure resources:    managed identity, federated identity credential, custom role definition, role assignment, and the resource group when create_resource_group is true
Files:              <files written or edited>

Approve? (y/n)
```

## Write the Terraform

Declare the provider requirements and configuration. The teleport provider must be
`>= 18.7.6`, the discovery module's minimum:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = ">= 4.0" }
    http    = { source = "hashicorp/http",    version = ">= 3.0" }
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= 18.7.6"
    }
  }
}

provider "azurerm" {
  features {}
}

provider "teleport" {
  addr = "<proxy_addr>"
}
```

Write the discovery module. When `create_resource_group` is true, create the resource group
and reference it from the module:

```hcl
resource "azurerm_resource_group" "teleport_discovery" {
  name     = "<resource_group>"
  location = "<location>"
}

module "azure_discovery" {
  source = "terraform.releases.teleport.dev/teleport/discovery/azure"

  teleport_proxy_public_addr    = "<proxy_addr>"
  teleport_discovery_group_name = "<discovery_group>"

  azure_resource_group_name       = azurerm_resource_group.teleport_discovery.name
  azure_managed_identity_location = azurerm_resource_group.teleport_discovery.location

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = [<each subscription ID quoted, comma-separated>]
      # regions         = [...]   include only if set
      # resource_groups = [...]   include only if set
      # tags            = {...}   include only if set
    }
  ]
}

output "integration_name"      { value = module.azure_discovery.teleport_integration_name }
output "discovery_config_name" { value = module.azure_discovery.teleport_discovery_config_name }
output "provision_token_name"  { value = module.azure_discovery.teleport_provision_token_name }
```

When `create_resource_group` is false, reference the existing group with a `data` source
instead of the `resource` block, and set:

```hcl
data "azurerm_resource_group" "teleport_discovery" {
  name = "<resource_group>"
}

# in the module block:
  azure_resource_group_name       = data.azurerm_resource_group.teleport_discovery.name
  azure_managed_identity_location = data.azurerm_resource_group.teleport_discovery.location
```

`azure_matchers` object schema:

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `types` | list(string) | required | Only `["vm"]`. |
| `subscriptions` | list(string) | required | Azure subscription IDs to search. |
| `regions` | list(string) | `["*"]` | `["*"]` matches all regions. |
| `resource_groups` | list(string) | `["*"]` | `["*"]` matches all resource groups. |
| `tags` | map(list(string)) | `{"*": ["*"]}` | Tag filter. |
