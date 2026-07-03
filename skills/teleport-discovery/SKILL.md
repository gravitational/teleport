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
  requires Terraform and credentials for the target cloud. The aws and az CLIs are optional.
  Setup uses them when present to detect an existing AWS OIDC provider, the Azure
  subscription, and the Azure resource group location. Works with Teleport Cloud and
  self-hosted clusters.
allowed-tools:
  - Read
  - WebFetch(domain:goteleport.com)
  - Bash(terraform init:*)
  - Bash(terraform plan:*)
  - Bash(tsh status:*)
  - Bash(tctl status:*)
  - Bash(tctl get discovery_config:*)
  - Bash(tctl get user_tasks:*)
  - Bash(tctl inventory list:*)
  - Bash(tctl discovery nodes:*)
  - Bash(tctl tokens ls:*)
  - Bash(aws iam list-open-id-connect-providers:*)
  - Bash(aws iam get-open-id-connect-provider:*)
  - Bash(az account show:*)
  - Bash(az group show:*)
---

# Teleport Auto-Discovery

## Communicating

Open with one or two sentences stating which procedures will run and what each produces or
checks. After that, address the user only to ask questions and to report each procedure's
outcome or stop. Never report individual field derivations, commands run, or intermediate
results.

## Determine the cloud

Set `CLOUD` before anything else. Infer `aws` when the request names EC2, EKS, or an AWS
account. Infer `azure` when it names VMs, a subscription, or a resource group. If
the request implies neither, stop and ask the user which cloud. Do not run `aws` or `az` commands
and do not write Terraform until `CLOUD` is set.

## Resolving fields

Resolve each field from the prompt first, then from its tool derivation, then from its
Default column. Treat a tool that is unavailable, ambiguous, or erroring as yielding
nothing. Where a procedure gathers fields, it lists them as `| Field | Tool derivation | Default |`.

In commands, `$TSH` and `$TCTL` stand for the tsh and tctl binaries, using the paths the
user gives or plain `tsh` and `tctl` otherwise.

When tsh or tctl fails for lack of a session, ask the user to run
`$TSH login --proxy=<proxy_addr>` in a separate terminal, then retry. Interactive logins
fail in the session, even with the `!` prefix.

## Asking

Each question states what the value controls in the final configuration, for example "Which
AWS regions should discovery search for EC2 instances?". Make the default the first option
and the other options concrete values. Write question text,
option labels, and option descriptions in the user's voice, such as "Run it for me",
because a bare I or you is ambiguous between you and the user. Free-form values arrive through the built-in Other
option. Never ask a follow-up round to refine an answer. When an answer is unusable, state
why and re-ask that single question.

AskUserQuestion takes at most 4 questions per call, so a round may span consecutive calls:
group matcher-scope questions such as regions, tags, and subscriptions together, and
logistics questions such as write location and apply choices together.

## Procedures

Run the procedures the request asks for, in order: Setup, Apply, then Monitor. "Set up
discovery" with no narrower scope runs all three.

### Setup

Write, generate, configure, or extend the discovery Terraform. Gather these common fields, then
the cloud-specific fields in `references/aws-setup.md` for `aws` or `references/azure-setup.md`
for `azure`. Run the reference's version gate as soon as `cluster_version` resolves, before
asking the user anything. When `cluster_version` itself must be asked, run the gate on the
answer before writing. Collect every field that resolves to Ask, including `write_location`
when the prompt does not specify it, then ask for all of them in a single round with the
AskUserQuestion tool rather than one at a time. When Apply will run, include the two apply
questions from `references/apply.md` in the same round.

| Field | Tool derivation | Default |
|-------|-----------------|---------|
| `proxy_addr` | `$TSH status --format=json`, `active.profile_url` with the `https://` scheme stripped, such as `example.teleport.sh:443` | Ask |
| `cluster_version` | `$TCTL status` `Version` field, such as `18.8.0` | Ask |
| `deployment` | `cloud` when `proxy_addr`'s host ends in `.teleport.sh`, `.cloud.gravitational.io`, or `.beams.sh`, else `self-hosted` | none |
| `discovery_group` | `cloud`: `cloud-discovery-group`. `self-hosted`: see **Self-hosted discovery group** | Ask, per **Self-hosted discovery group** |
| `write_location` | none | Ask, with a new `teleport-discovery/` directory as the default |

#### Self-hosted discovery group

Confirm a Discovery Service runs with `$TCTL inventory list --services=discovery`, and stop
when none does. The inventory output does not show groups, so collect the `discovery_group`
values from `$TCTL get discovery_config --format=json` and offer them as options. The
question states the value must match `discovery_group` in a running Discovery Service's
configuration.

#### Write location

Into a new project, write a fresh module in the `write_location` directory with `versions.tf`
and `main.tf`. Into an existing Terraform project, integrate following its structure. If the
project already declares the `module "aws_discovery"` or `module "azure_discovery"` block,
read it, pre-populate the gathered fields from its current values, and edit that block in
place.

### Apply

Apply the Terraform to create the resources, with `references/apply.md`. Precede it with Setup
when the Terraform is not written yet.

### Monitor and Troubleshoot

Check status, watch a sync, or diagnose why resources are not enrolling, with
`references/monitor.md`.
