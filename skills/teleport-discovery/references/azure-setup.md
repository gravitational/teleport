# Azure Discovery Setup

Version gate: if `cluster_version` is below `18.7.6`, stop: "Azure discovery with Terraform
requires Teleport 18.7.6 or later. This cluster is v`<cluster_version>`."

Resolve the common fields from the skill's Setup section, then these Azure fields.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `subscriptions` | `az account show --query id --output tsv` for the current subscription | Ask |
| `resource_group` | none | Ask |
| `location` | see **Resource group and location** below | Ask, per **Resource group and location** |
| `regions` | none | Ask, with `["*"]` as the default |
| `resource_groups` | none | Ask, with `["*"]` as the default |
| `tags` | none | Ask, with `{"*": ["*"]}` as the default |

Validate that every subscription ID is a UUID of the form
`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`. If any fails, ask the user to correct it.

## Resource group and location

This procedure sets `create_resource_group`. Do not ask for it.

When `resource_group` resolves before the question round, run
`az group show --name <resource_group> --query location --output tsv`. On success, set
`location` to the output and `create_resource_group=false`, and do not ask for `location`.
On failure, set `create_resource_group=true` and ask for `location` in the round.

When `resource_group` goes to the round, ask for `location` in the same round, phrased as
where the resource group will be created if it does not exist. After the round, run the same
`az group show` for the answered group. On success, set `location` to the output and
`create_resource_group=false`. On failure, keep the answered `location` and set
`create_resource_group=true`.

## Write the Terraform

Let `<major>` be `cluster_version`'s major version. Set `<provider_version>` to
`>= 18.7.6, < 19.0.0` when `<major>` is 18, else `~> <major>.0`.

Declare the provider requirements and configuration:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "<provider_version>"
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
  source  = "terraform.releases.teleport.dev/teleport/discovery/azure"
  version = "~> <major>.0"

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
