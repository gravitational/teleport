---
name: teleport-discovery-config
description: >
  Configure Teleport AWS auto-discovery: stand up an AWS OIDC integration and a
  discovery_config that auto-enrolls EC2 instances or EKS clusters via a Terraform
  module, then verify and monitor enrollment. Use whenever someone wants to set
  up, enable, add to, or fix EC2/EKS auto-discovery on a Teleport cluster.
  Triggers on phrasings like "discover my EC2 instances", "enroll our EKS
  clusters", "write a discovery config", "add a region or tag to discovery", or
  "why aren't my instances getting discovered". Do NOT use for: generic AWS or
  Terraform authoring, IAM/OIDC work unrelated to Teleport, static-token or manual
  join-token node enrollment, SSH/Kubernetes access configuration, or non-AWS
  providers (Azure, GCP).
compatibility: >
  Requires tctl and tsh authenticated to the target cluster and the aws CLI with
  credentials for the target AWS account. A local Terraform apply also requires
  the Terraform CLI; other run environments are covered by their setup guides.
  Works with Teleport Cloud and self-hosted clusters.
---

# Teleport AWS Discovery Configuration

Configure auto-discovery of AWS EC2 instances and EKS clusters on a Teleport
cluster. Discovery needs three resources: an **AWS OIDC integration** so Teleport
can call AWS, a **discovery_config** holding the matchers that select resources,
and for EC2 a **provision token** so discovered instances can join. A Terraform
module builds all three. This skill writes it, applies it, then verifies and
monitors enrollment.

## How to operate

A request can chain steps, for example "write a discovery config, apply it, then
watch it sync". Work the request as an ordered plan:

1. **Scope** the goal: which services (Step 1).
2. **Gather** every required field, each from its highest-precedence source (Step 2).
3. **Show the plan** and get approval (Step 3).
4. **Write** the Terraform (Step 4).
5. **Apply** the Terraform (Step 5).
6. **Verify**, then offer to monitor or troubleshoot (Step 6).

These rules keep the work correct:

- **Scope is AWS EC2 and EKS only.** If the request targets another provider
  (Azure, GCP) or resource type (RDS, Redshift, ...), say it is not supported and
  stop. Do not improvise an unsupported path.
- **Gather before planning, plan before executing.** Never write or apply a
  resource before the user approves the plan. Discovery configs, AWS IAM roles,
  and OIDC providers are real, billable, externally visible resources.
- **Source every field by precedence**: the user's prompt first, then a value
  already in this conversation, then the tool-use command. The fields and their
  sources are documented in `references/procedure-terraform.md`. Ask the user only
  for fields no source yields, and batch those into one message. Do not invent
  defaults.
- **On any failure, stop.** Surface the verbatim error and the working directory.
  Do not auto-destroy, re-apply, or retry without approval.

## Step 1: Scope the goal

Determine which **services** to discover: `ec2`, `eks`, or both. If unstated, ask.

## Step 2: Gather requirements

Read the **Procedure requirements** and **Terraform module requirements**
sections of `references/procedure-terraform.md`. They list where to write the
Terraform and every field with its sources in precedence order. For each field,
take the value from the first source that yields one: the user's prompt, then a
value already established in this conversation, then the tool-use command shown.
Run independent tool-use commands in parallel. Ask the user, in one batched
message, only for fields no source yields.

Also determine the Terraform run environment, default local, per the **Run
environment** section of `references/procedure-apply.md`.

## Step 3: Show the plan

Present this with real values, never placeholders, and wait for approval unless
the request set `auto_approve: true`.

```
## Environment
Cluster:         <cloud|self-hosted>, <proxy-addr> (v<version>)
AWS account:     <id>
Discovery group: <group>
Terraform run:   <local | CI/cloud | Terraform Cloud | dedicated server>
Existing AWS OIDC integration: <name | none>
Existing AWS OIDC provider for proxy: <yes | no>

## Plan
1. <action> => <outcome>
2. ...

Teleport resources: <kind/name, ...>
AWS resources:      <role, policy, oidc provider>
Commands:           <command, ...>

Approve? (y/n)
```

## Step 4: Write the Terraform

Follow `references/procedure-terraform.md` step by step. It produces the
`versions.tf` and `main.tf`, or adds the module block to an existing project.

## Step 5: Apply the Terraform

Follow `references/procedure-apply.md`. The default is a local run; for any other
environment it points to that environment's setup guide. On failure, stop per the
rules above and use the apply troubleshooting table in the same file.

## Step 6: Verify, then offer monitoring

Confirm the resources exist per the **Verify** section of
`references/monitor-troubleshoot.md`. Then offer, never auto-run, to watch the
first sync per the same file. If enrollment fails, use its troubleshooting table.
