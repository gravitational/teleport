# Azure Discovery Setup

State these to the user:
- The Azure credentials running Terraform need permission to create a managed identity, a custom role definition, and a role assignment in the target subscription.
- Each VM to be discovered must have a managed identity assigned with at least the `Microsoft.Compute/virtualMachines/read` permission, or it will not enroll.

Resolve the common fields from the skill's Setup section, then these Azure fields.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `subscriptions` | `az account show --query id --output tsv` for the current subscription | Ask |
| `resource_group` | none | Ask |
| `location`, `create_resource_group` | `az group show --name <resource_group> --query location --output tsv`: on success set `location` to the output and `create_resource_group=false`; on failure set `create_resource_group=true` | Ask for `location`; set `create_resource_group=true` |
| `regions` | none | Ask, with `["*"]` as the default |
| `resource_groups` | none | Ask, with `["*"]` as the default |
| `tags` | none | Ask, with `{"*": ["*"]}` as the default |

If `cluster_version` is below `18.7.6`, stop: "Azure discovery with Terraform requires Teleport
18.7.6 or later. This cluster is v`<cluster_version>`."

Validate that every subscription ID is a UUID of the form
`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`. If any fails, ask the user to correct it.

## Write location

Into a new project, write a fresh module in the `write_location` directory with `versions.tf`
and `main.tf`. Into an existing Terraform project, integrate following its structure. If the
project already declares the `module "azure_discovery"` block, read it, pre-populate the gathered
fields from its current values, and edit that block in place.

## Write the Terraform

Declare the provider requirements and configuration:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
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
      # add regions, resource_groups, or tags per the schema below to narrow
    }
  ]
}

output "integration_name" {
  value = module.azure_discovery.teleport_integration_name
}

output "discovery_config_name" {
  value = module.azure_discovery.teleport_discovery_config_name
}

output "provision_token_name" {
  value = module.azure_discovery.teleport_provision_token_name
}
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

Omit any field whose value equals its default above.
