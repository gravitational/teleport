# Azure Discovery

## Find `az` CLI

If `AZ` is already set, use it. Otherwise run `which az` silently — if successful, set `AZ=az`. If neither, stop:

> "The Azure CLI (`az`) is required. Install it from https://learn.microsoft.com/en-us/cli/azure/install-azure-cli"

Run `$AZ account show --query id --output tsv` silently. If not logged in, stop:

> "You're not logged in to Azure. Run `az login` and then run this skill again."

Set `SUBSCRIPTION_ID` from the output.

## Teleport Version Check

If `CLUSTER_VERSION` is below `18.8`, stop:

> "Azure Discovery requires Teleport 18.8 or later. Your cluster is running v<CLUSTER_VERSION>."

## Prerequisites

Inform the user: this will guide you through creating or updating an existing Terraform configuration using the teleport-discovery-azure module. This will configure the Teleport Discovery Service and the required Azure resources for auto-discovery. Before continuing:

1. Your Azure account needs permissions to create resource groups, managed identities, role definitions, and role assignments in the target subscription(s).
2. Each VM to be discovered must have a managed identity assigned (system-assigned or user-assigned).
3. VMs must run a supported Linux distribution (Ubuntu, Debian, RHEL, Amazon Linux 2, or similar).

## Collect Configuration

Extract all values already provided in the prompt. For each missing required field, ask
conversationally. Present the menu to show current state; redisplay after each change.

**Menu** (redisplay after each change):

```
Azure Discovery Configuration

  Managed Identity
    Resource group: <value or "(required)">
    Location:       <value or "eastus (default)">

  Discovery Matchers
    Subscriptions:   <value or "(required)">
    Regions:         <value or "* (all)">
    Resource groups: <value or "* (all)">
    Tags:            <value or "* (all)">

  Terraform directory: <value or "./teleport-azure-discovery">

  Discovery group: cloud-discovery-group (fixed)   ← Teleport Cloud
                   <value or "(required)">         ← self-hosted

```

Render only one Discovery group line based on whether `PROXY_ADDR` ends in `.teleport.sh` or `.cloud.gravitational.io` (both are Teleport Cloud domains).

**Configuration** — present all questions together in a single `AskUserQuestion`
call (3 questions for Teleport Cloud, 4 for self-hosted).

```
Question 1 (header: "Identity"):
  "Where should the managed identity be created?"
  Options:
    - Create 'teleport-discovery' in eastus — "Defines a resource group in Terraform where the module's Azure resources will be created"
    - Use an existing resource group — "Specify your resource group name and location"

Question 2 (header: "Matchers"):
  "Which VMs should Teleport discover? (subscription <SUBSCRIPTION_ID> is pre-selected)"
  Options:
    - All VMs in this subscription
    - Match by region
    - Match by resource group
    - Match by tags (e.g. teleport-auto-enroll=true)

Question 3 (header: "Terraform"):
  "Where should I write the Terraform files?"
  Options:
    - Create a new project — "./teleport-azure-discovery in the current directory"
    - Use a different directory — "Specify a path"

Question 4 (header: "Discovery", self-hosted only — omit for Teleport Cloud):
  "What is the discovery_group name from your Discovery Service config?"
  Options:
    - default — "Uses the default discovery group name"
    - Custom name — "Must match discovery_group in your Discovery Service config"
```

Do not mark any option as recommended.

For Question 1:
- **Create 'teleport-discovery' in eastus** → set `AZURE_MANAGED_IDENTITY_RESOURCE_GROUP=teleport-discovery`, `AZURE_MANAGED_IDENTITY_LOCATION=eastus`. No follow-up needed.
- **Use an existing resource group** → follow up: "Which resource group and location? e.g. `my-rg` or `my-rg in westeurope` (location defaults to eastus)".

After resolving `AZURE_MANAGED_IDENTITY_RESOURCE_GROUP`, check if the resource group already exists in Azure:

```bash
$AZ group show --name <AZURE_MANAGED_IDENTITY_RESOURCE_GROUP> --query location --output tsv 2>/dev/null
```

- If the command succeeds → set `CREATE_RESOURCE_GROUP=false` and use the returned location as `AZURE_MANAGED_IDENTITY_LOCATION`.
- If the command fails (not found) → set `CREATE_RESOURCE_GROUP=true`.

For Question 2, follow up after the response if values are needed:
- **All VMs**: "Any additional subscription IDs to enroll? (comma-separated, or leave blank)"
- **Region**: "Which region(s)? e.g. `westus, eastus`" (if unsure: suggest `az account list-locations --output table`)
- **Resource group**: "Which resource group(s)?"
- **Tags**: "Which tag(s)? e.g. `env=prod, teleport-auto-enroll=true`"

If the user typed specific values directly (e.g. "westus"), apply them without a follow-up.

`subscriptions` always starts with `<SUBSCRIPTION_ID>`; append any extras. Validate that every
subscription ID is a well-formed Azure UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`). If any
ID fails validation, tell the user and ask them to correct it before continuing.
→ HCL: `subscriptions = ["id1", "id2"]`

Parse into HCL fields. Omit fields not set:
→ `regions`, `resource_groups`, `tags` (tag values are lists)

Do not infer or suggest tags from existing Terraform files.

Set `WORKDIR` from Question 3. Detect whether `WORKDIR` contains an existing Terraform project and adapt accordingly:

- **No existing project** → create a complete, self-contained Terraform project (providers + module) ready to `terraform apply`.
- **Existing project** → integrate into it: find and update an existing module definition, or add a new one. Edit provider config as needed.

When pre-populating from an existing project, only suggest values explicitly set in
project files (e.g. `*.tfvars`, module arguments). Do not present variable defaults as
confirmed values — they may be overridden by tfvars, environment variables, or CLI flags.

Use Grep to search for an existing module reference — scoped to `WORKDIR` only:

```
Grep: "terraform.releases.teleport.dev/teleport/discovery/azure"
path: <WORKDIR>
glob: "*.tf"
```

**If the module reference is found (existing discovery config):**

Read the matching file and extract current values:
- `teleport_proxy_public_addr`
- `azure_resource_group_name`
- `azure_managed_identity_location`
- `azure_matchers` (subscriptions, regions, resource_groups, tags)

Pre-populate values from the existing config. If all required fields are resolved, skip to the confirmation summary. Otherwise present the menu and AskUserQuestion with existing values as recommended options.

After confirmation, use the `Edit` tool to update the module definition in the file where it was found.

**If `.tf` files exist but no module reference (existing project without discovery):**

Use the `Edit` tool to add any missing providers to the existing provider configuration file:

```hcl
terraform {
  required_providers {
    # Add if not already present:
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= <CLUSTER_VERSION>"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

# Add if not already present:
provider "teleport" {
  addr = "<PROXY_ADDR>"
}

provider "azurerm" {
  features {}
}
```

Then use the `Write` tool to create `<WORKDIR>/azure_discovery.tf` with the module definition.

**If no `.tf` files (new project):**

Use the `Write` tool to create both files:
- `<WORKDIR>/versions.tf` — providers and versions
- `<WORKDIR>/azure_discovery.tf` — module definition and output

**Discovery group** — Cloud: `cloud-discovery-group` (fixed). Self-hosted: use the value from Question 4. For private clusters see the [Azure VM Auto-Discovery (Terraform) docs](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-vm-discovery/azure-vm-discovery-terraform/).

**"confirm"** — require resource group and subscriptions before proceeding. Present summary:

> Install managed identity in resource group `<RG>` (location: `<LOCATION>`)
> Enroll subscription(s) `<SUBSCRIPTIONS>`
> [Match VMs by `<REGIONS>` / resource group `<RG_MATCHERS>` / tags `<TAG_MATCHERS>` — omit if not set]
> Write Terraform files to `<WORKDIR>`
>
Ask with `AskUserQuestion`:
- "Yes, generate the configuration"
- "Change something" → return to Collect Configuration

## Generate Terraform Files

Fill in all values from the configuration step and use the `Write` or `Edit` tool to propose
the files. The user will see a diff and can approve or reject each change. After writing,
print a configuration summary.

**`versions.tf` template:**

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= <CLUSTER_VERSION>"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

provider "teleport" {
  addr = "<PROXY_ADDR>"
}

provider "azurerm" {
  features {}
}
```

**`azure_discovery.tf` template:**

If `CREATE_RESOURCE_GROUP=true` (user chose the default), include the resource group resource
and reference it from the module:

```hcl
resource "azurerm_resource_group" "teleport_discovery" {
  name     = "teleport-discovery"
  location = "eastus"
}

module "azure_discovery" {
  source  = "terraform.releases.teleport.dev/teleport/discovery/azure"
  version = "~> <MODULE_VERSION>"

  teleport_proxy_public_addr    = "<PROXY_ADDR>"
  teleport_discovery_group_name = "<DISCOVERY_GROUP>"

  azure_resource_group_name       = azurerm_resource_group.teleport_discovery.name
  azure_managed_identity_location = azurerm_resource_group.teleport_discovery.location

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = [<SUBSCRIPTIONS — each ID quoted, comma-separated>]
      # regions         = [...] — include only if configured
      # resource_groups = [...] — include only if configured
      # tags            = {...} — include only if configured
    }
  ]
}

output "azure_discovery" {
  value = module.azure_discovery
}
```

If `CREATE_RESOURCE_GROUP=false` (resource group already exists in Azure), use a `data`
source to reference it:

```hcl
data "azurerm_resource_group" "teleport_discovery" {
  name = "<AZURE_MANAGED_IDENTITY_RESOURCE_GROUP>"
}

module "azure_discovery" {
  source  = "terraform.releases.teleport.dev/teleport/discovery/azure"
  version = "~> <MODULE_VERSION>"

  teleport_proxy_public_addr    = "<PROXY_ADDR>"
  teleport_discovery_group_name = "<DISCOVERY_GROUP>"

  azure_resource_group_name       = data.azurerm_resource_group.teleport_discovery.name
  azure_managed_identity_location = data.azurerm_resource_group.teleport_discovery.location

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = [<SUBSCRIPTIONS — each ID quoted, comma-separated>]
      # regions         = [...] — include only if configured
      # resource_groups = [...] — include only if configured
      # tags            = {...} — include only if configured
    }
  ]
}

output "azure_discovery" {
  value = module.azure_discovery
}
```

After writing the files, print a configuration summary:

```
Configuration summary:
- Cluster proxy:                   <PROXY_ADDR>
- Subscriptions:                   <SUB_1>, <SUB_2>, ...
- Discovery group:                 <DISCOVERY_GROUP>
- Managed Identity Resource Group: <AZURE_MANAGED_IDENTITY_RESOURCE_GROUP>
- Managed Identity Location:       <AZURE_MANAGED_IDENTITY_LOCATION>
- Output directory:                <WORKDIR>
```

## Apply Terraform

Present the commands to the user:

> **You're ready to apply.**
>
> ```bash
> cd <WORKDIR>
>
> # Download the Teleport discovery module and Azure provider
> terraform init
>
> # Use tbot to generate short-lived Teleport credentials
> eval "$(tctl terraform env)"
>
> # Apply Terraform configuration to create the managed identity, 
> # role definitions, and role assignments in Azure,
> # and register the OIDC integration with Teleport
> terraform apply
> ```
>
> Run these when you're ready, or ask to apply Terraform to continue the setup.

**If the user asks you to apply**, execute the commands. Env vars do not persist between Bash calls, so chain the credential generation with terraform commands in a single Bash call.

First, run `terraform init` and `terraform plan`:

```bash
cd <WORKDIR>
terraform init
eval "$($TCTL terraform env)" && terraform plan
```

Review the plan output. If it contains any `destroy` or `replace` actions, stop and warn the user — show which resources would be affected and ask for confirmation before proceeding.

Once the plan looks safe (or the user explicitly approves destructive changes), apply:

```bash
cd <WORKDIR>
eval "$($TCTL terraform env)" && terraform apply -auto-approve
```

**Truncated output** — `terraform plan` or `terraform apply` can produce long output. If output is truncated, check the exit code:
- **Exit code 0** → command succeeded. Proceed to next step.
- **Non-zero** → tell the user the command failed but the error was cut off, then re-run with `| tail` to capture the error.

**After a successful apply**, resolve `INTEGRATION_NAME`:

1. Try `terraform output -json azure_discovery` and parse `teleport_integration_name` from the result.
2. If no output is available, fall back to `$TCTL get integrations --format=json` and find the `azure-oidc` integration.

## Verify

Run silently after the user confirms apply is complete:

```bash
$TCTL discovery nodes --cloud=azure --format=json
```

Parse the JSON output and present it as a readable table listing each VM Teleport has attempted to enroll, with status (`Online`, `Installed (offline)`, `Failed`). Never show raw JSON. Only report discovery status from `tctl` output — do not interpret or comment on project-specific resources. If no rows appear yet:

> "Discovery runs automatically every ~5 minutes. Check back soon with:
>
> ```
> tctl discovery nodes --cloud=azure
> ```"

Link to the integration in the web UI — use the hostname from the proxy address, without the port (e.g. `example.teleport.sh:443` → `https://example.teleport.sh`):
- If `INTEGRATION_NAME` is available: `https://<PROXY_HOST>/web/integrations/overview/azure-oidc/<INTEGRATION_NAME>`
- Otherwise: `https://<PROXY_HOST>/web/integrations`
