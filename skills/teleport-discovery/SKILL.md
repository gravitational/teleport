---
name: teleport-discovery
description: >
  Configure Teleport Auto-Discovery to connect cloud resources to Teleport. Use when the user
  asks to set up auto-discovery, enroll cloud resources into Teleport, configure the Teleport
  Discovery Service, onboard Azure VMs, or AWS resources like EC2 instances and EKS clusters 
  using Terraform and an OIDC integration. Trigger on phrases like "configure teleport discovery",
  "set up auto-discovery", "enroll my Azure VMs", "enroll EC2 instances", or "connect my cloud resources to Teleport".
  Also trigger when the user wants to check the enrollment status or troubleshoot enrollment of cloud resources.
compatibility: >
  Requires: Teleport CLI tools (tsh, tctl) authenticated to target cluster. Terraform. Azure CLI required for Azure.
allowed-tools:
  - Bash(az account show --query id --output tsv)
---

# Teleport Auto-Discovery

Connect your cloud resources to Teleport automatically with Auto-Discovery. Configures
the Teleport Discovery Service via Terraform modules and creates an OIDC integration for
your cloud provider (Azure, AWS).

## Determine Intent

Classify the user's request into one of two paths:

- **Guided Setup** — configure discovery for the first time, generate or update Terraform, apply it → [Prerequisites](#prerequisites) then [Guided Setup](#guided-setup)
- **Discovery Status** — check enrollment status or diagnose failures → [Prerequisites](#prerequisites) then [Discovery Status](#discovery-status)

## Prerequisites

Shared by both paths.

### Find `tsh`

If `TSH` is already set, use it. Otherwise run `which tsh` — if successful, set `TSH=tsh`. If neither, stop:

> "tsh is required. Download it from https://goteleport.com/download"

### Check authentication

Run silently:

```bash
$TSH status --format=json
```

Parse the `active` field only — do **not** read or log `active.traits` (it contains PII). Extract:
- `PROXY_ADDR` <- `active.profile_url`, stripping any `https://` scheme (e.g. `https://example.teleport.sh:443` -> `example.teleport.sh:443`)
- `CLUSTER` <- `active.cluster`

If `active` is null or the command exits non-zero, stop — do not proceed to find tctl or any cloud CLI:

> "You're not logged in to Teleport. Log in first with:
>
> ```
> tsh login --proxy=<your-cluster-proxy>
> ```
>
> Then run this skill again."

If `profiles` contains more than one entry, notify the user and proceed — do not prompt or read any other files:

> "Using active cluster: `<CLUSTER>`."

### Find `tctl`

If `TCTL` is already set, use it. Otherwise run `which tctl` — if successful, set `TCTL=tctl`. If neither, stop:

> "tctl is required. Download it from https://goteleport.com/download"

### Verify cluster and Terraform

**Run all of these in a single Bash call. Do not display raw output to the user.**

```bash
$TCTL status
terraform version
```

Extract silently:
- From `$TCTL status`: `CLUSTER_VERSION` (e.g. `18.8.0`). Set `MODULE_VERSION` = major.minor (e.g. `18.8`).
- From `terraform version`: confirm it is present. Ignore provider list and upgrade notices.

If any command fails, stop and tell the user what to fix. Otherwise, confirm success in one line:

> "Connected to `<CLUSTER>` (v`<CLUSTER_VERSION>`). Terraform v`<terraform version>` found."

### Detect Cloud Provider

If the cloud provider is already clear from the prompt (e.g., the user mentioned "Azure",
"AWS", or specific resource types like "EC2 instances" or "Azure VMs"), proceed directly
to that provider without asking.

Otherwise, ask:

> "Which cloud provider do you want to configure discovery for?
> - **Azure** — discover and enroll Azure VMs
> - **AWS** — discover and enroll EC2 instances *(coming soon)*"

Set `CLOUD` based on the answer (e.g. `CLOUD=azure`).

---

## Guided Setup

Configure and apply Terraform to set up discovery and the OIDC integration.

**Azure** — Read and follow [Azure Discovery](references/azure-discovery.md).
`PROXY_ADDR`, `CLUSTER_VERSION`, and `MODULE_VERSION` from Prerequisites carry over.

**AWS** — Stop and inform the user:

> "AWS discovery support is not yet available in this skill. For Azure VM discovery,
> start again and specify Azure."

After generating Terraform files, proceed to [Apply Terraform](#apply-terraform).

### Apply Terraform

Present the commands to the user:

> **You're ready to apply.**
>
> ```bash
> cd <WORKDIR>
>
> # Download the discovery module and cloud provider
> terraform init
>
> # Generate short-lived Teleport credentials
> eval "$(tctl terraform env)"
>
> # Apply the Terraform configuration
> terraform apply
> ```
>
> Run these when you're ready, or ask to apply Terraform to continue the setup.

When executing terraform, chain with `eval "$($TCTL terraform env)" &&` in a single call — env vars don't persist between calls. This is not needed for `terraform init`. Do not include this chaining in the commands presented to the user.

**If the user asks you to apply**, run the commands:

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

1. Try `terraform output -json` and parse `teleport_integration_name` from the result.
2. If no output is available, fall back to `$TCTL get integrations --format=json` and find the integration with subkind `azure-oidc` (Azure) or `aws-oidc` (AWS).

Link to the integration in the web UI — use the hostname from the proxy address, without the port (e.g. `example.teleport.sh:443` -> `https://example.teleport.sh`):
- If `INTEGRATION_NAME` is available: `https://<PROXY_HOST>/web/integrations/overview/azure-oidc/<INTEGRATION_NAME>`
- Otherwise: `https://<PROXY_HOST>/web/integrations`

After apply completes, proceed to [Discovery Status](#discovery-status).

---

## Discovery Status

Check enrollment status or troubleshoot failures. Used both as the final step of Guided Setup and as a standalone troubleshooting path. Uses `TCTL` and `CLOUD` from Prerequisites.

**Diagnosis sources** — diagnose only from `tctl discovery nodes` output and Teleport documentation. Do not read Terraform configurations or other project files unless the user specifically asks.

**Run the nodes report:**

```bash
$TCTL discovery nodes --cloud=<CLOUD> --last=24h --format=json
```

Show the command to the user before running it. Parse the JSON output and present it as a readable table. Never show raw JSON. Status values: `Online`, `Installed (offline)`, `Failed (<reason>)`.

If no rows appear, inform the user that no instances were seen in the last 24 hours and the Discovery Service polls every few minutes. Verify the matcher configuration (subscriptions, tags, regions) in the discovery config matches running resources.

- **Cloud**: uses the fixed `cloud-discovery-group` discovery group.
- **Self-hosted**: `discovery_group` must match the Discovery Service configured in `teleport.yaml`; verify the service is running.

If the issue persists, suggest the user can verify the expected resources were created. The integration should have subkind `azure-oidc` (Azure) or `aws-oidc` (AWS), and a corresponding discovery config should exist:

    tctl get integrations --format=json
    tctl get discovery_config --format=json

**If any failures exist**, fetch the troubleshooting guide to get resolution steps:

```
WebFetch:
  URL: https://goteleport.com/docs/enroll-resources/auto-discovery/servers/troubleshooting.md
  Prompt: "Extract all troubleshooting content for <CLOUD> discovery. Include exit code meanings, status interpretations, common errors, and resolution steps."
```

Use the guide to match each failure's status, exit code, and details to its resolution steps. Present only the relevant resolution steps to the user.
