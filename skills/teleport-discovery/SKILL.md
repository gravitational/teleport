---
name: teleport-discovery
description: >
  Configure and troubleshoot Teleport auto-discovery for AWS EC2 instances, AWS EKS
  clusters, and Azure VMs via the Teleport discovery Terraform module. Use to set up or
  extend auto-discovery, add a region, tag, or subscription, apply the discovery Terraform,
  check enrollment status, or diagnose why cloud resources are not enrolling. Not for GCP,
  AWS RDS or database discovery, static or manual join-token enrollment, or Teleport
  configuration unrelated to discovery.
compatibility: >
  Requires tsh and tctl authenticated to the target cluster. Applying the generated Terraform
  requires Terraform and credentials for the target cloud. The aws and az CLIs are optional;
  setup uses them when present to detect account, region, and subscription details. Works
  with Teleport Cloud and self-hosted clusters.
allowed-tools:
  - Read
  - Glob
  - Write(**/*.tf)
  - Edit(**/*.tf)
  - AskUserQuestion
  - WebFetch(domain:goteleport.com)
  - Bash(terraform init:*)
  - Bash(terraform plan:*)
  - Bash(terraform apply:*)
  - Bash(tsh status:*)
  - Bash(tctl status:*)
  - Bash(tctl get:*)
  - Bash(tctl inventory list:*)
  - Bash(tctl discovery nodes:*)
  - Bash(tctl tokens ls:*)
  - Bash(aws iam list-open-id-connect-providers:*)
  - Bash(aws iam get-open-id-connect-provider:*)
  - Bash(aws configure get region:*)
  - Bash(az account show:*)
  - Bash(az group show:*)
---

# Teleport Auto-Discovery

## Determine the cloud

Set `CLOUD` before anything else. Infer `aws` when the request names EC2, EKS, or an AWS
account. Infer `azure` when it names VMs, a subscription, or a resource group. If
the request implies neither, stop and ask the user which cloud. Do not run `aws` or `az` commands
and do not write Terraform until `CLOUD` is set.

## Resolving fields

Resolve every field in this order: take it from the prompt; otherwise run its tool
derivation; if the tool is unavailable, ambiguous, or errors, use its default or ask the
user. Where a procedure gathers fields, it lists them as `| Field | Tool derivation | Default |`.

In commands, `$TSH` and `$TCTL` are the tsh and tctl binaries; use the paths the user gives,
otherwise `tsh` and `tctl`.

## Procedures

Run the procedures the request asks for, in order: Setup, Apply, then Monitor. "Set up
discovery" with no narrower scope runs all three.

### Setup

Write, generate, configure, or extend the discovery Terraform. Gather these common fields, then
the cloud-specific fields in `references/aws-setup.md` for `aws` or `references/azure-setup.md`
for `azure`. Collect every field that resolves to Ask, including `write_location` when the
prompt does not specify it, then ask for all of them in a single round with the AskUserQuestion
tool rather than one at a time.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `proxy_addr` | `$TSH status --format=json`, `active.profile_url` with the `https://` scheme stripped, such as `example.teleport.sh:443` | Ask |
| `cluster_version` | `$TCTL status` `Version` field, such as `18.8.0` | Ask |
| `deployment` | `cloud` when `proxy_addr`'s host ends in `.teleport.sh`, `.cloud.gravitational.io`, or `.beams.sh`, else `self-hosted` | none |
| `discovery_group` | `cloud`: `cloud-discovery-group`. `self-hosted`: confirm a service runs with `$TCTL inventory list --services=discovery`, and stop if none runs | Ask, with `cloud-discovery-group` as the default |
| `write_location` | none | Ask, with a new `teleport-discovery/` directory as the default |

### Apply

Apply the Terraform to create the resources, with `references/apply.md`. Precede it with Setup
when the Terraform is not written yet.

### Monitor and Troubleshoot

Check status, watch a sync, or diagnose why resources are not enrolling, with
`references/monitor.md`.
