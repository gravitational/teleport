---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 0191 - Workload Identity Configuration and RBAC Overhaul

## Required Approvers

* Engineering: @timothyb89
* Product: @thedevelopnik

## What

Overhaul the Workload Identity configuration and RBAC UX to resolve challenges 
with operating Workload Identity at Scale.

This will be achieved by introducing a new resource to Teleport, the
WorkloadIdentity, which will become the primary resource used to configure
Teleport Workload Identity.

## Why

Today, Teleport Workload Identity is configured via a combination of mechanisms:
Roles, Join Tokens, Bots and the `tbot` configuration itself. This worked fine
for the initial proof of concept where operation at scale was not a primary
concern, however, it has become apparent that without change, this will not
scale nicely.

Let's consider this within the context of a common use-case.

I wish to issue my GitLab CI workflows SPIFFE IDs that uniquely identify them.
For example, `spiffe://example.teleport.sh/gitlab/my-org/my-repo/my-workflow`.
I have 1000 of these.

Today, I need to create a Join Token, Bot, Role for each workflow. I must also
maintain a unique configuration for the `tbot` within each workflow.

This has the following negative impacts:

- Maintaining a large number of similar configuration items is laborious and
  frustrating.
- Enforcing policies/compliance across a larger number of configuration items
  is more challenging.
- Configuration in `tbot` can "mismatch" what is allowed by Teleport's RBAC
  engine, leading to errors. Therefore, `tbot` configuration must be kept
  "in-step" with Teleport resource-based configuration.
- Larger numbers of resources places a greater amount of pressure on the 
  Teleport Control Plane and cache.

This leads to a couple of "desires":

- The ability to "template" the SPIFFE IDs of issued SVIDs using metadata from
  attestations, reducing the number of configuration resources required.
- The ability to modify the issuance of SPIFFE IDs without needing to modify
  the configuration of individual `tbot` installations. If I want to change the
  structure of the SPIFFE IDs, I should be able to do this without modifying
  each individual workflow's configuration.
- The ability to centrally control elements of issuance that are critical for
  compliance (e.g TTL, Key types etc).

## Background

When issuing a workload an identity, there's a number of pieces of metadata that
can be used in that decision, and could potentially included in the resulting
identity document:

- Join Metadata (Node Attestation)
  - When a bot authenticates/joins via the Teleport Auth Service, rich metadata
    about the delegated identity is attested to the Auth Service. For example,
    when joining with the GitLab join method, the Auth Service can determine the
    organization, repository and workflow names.
- Workload Attestation Metadata
  - When a workload connects to the SPIFFE Workload API, the `tbot` agent can
    determine metadata about the workflow itself. For example, on Kubernetes 
    it can determine the name of the pod, namespace and service account.
  - It's worth bearing in mind that whilst the `tbot` agent is able to "trust"
    this information, the Auth Service cannot transitively "trust" that this
    has not been tampered by a malicious `tbot`. This should be considered
    when designing SPIFFE IDs, with information that can be attested from the 
    Join itself coming before information from the workload attestation.
  - In some environments, this is likely to be uninformative. For example, with
    a CI/CD run in GitLab, the Join Metadata containing information such as the
    repository or workflow is more interesting for the purposes of templating a
    SPIFFE ID than the UID or PID of the workload that has connected to tbot
    within the workflow execution.
- Bot Metadata
  - This is a looser category, but consider traits, labels and name of the Bot
    requesting the generation of the identity. This information doesn't come 
    from an attestation process, but is typically "administrative" and set by
    the operator.

Today:

- `tbot` authenticates to the Auth Service in the join process, and if the 
  attested delegated identity matches the Join Token rules, it is granted a 
  certificate for the attached Bot identity including the Bot's roles.
- A Teleport Role can contain rules that determine what SPIFFE IDs can be 
  requested in SVIDs by a principal that holds this role. Basic wildcarding is
  possible.
- `tbot` itself decides which SPIFFE ID should be requested for a given
  workload based on its configuration and which workload attestation rules
  within that configuration have passed.
- Effectively, a combination of local and central configuration controls what
  SVIDs are issued by `tbot`.

## UX

### Future: Ability to further customize SVIDs

In a future iteration, we'd introduce the ability for the `WorkloadIdentity`
resource to specify more than just the SPIFFE ID of the generated identity
document.

This ability serves many purposes:

- Allow the encoding of additional information that is not considered for 
  authorization but may be useful to be included in workload audit logs.
- Allow the encoding of additional information that is only rarely used for
  authorization - or - does not fit well into the structure of a SPIFFE ID for
  other reasons.

For JWT SVIDs, this would be the configuration of additional claims. For X509
SVIDs, this would be the additional SANs or customisation of the Subject
Distinguished Name.

Example configuration:

```yaml

```

## Technical Design

### Encoding Join Metadata into Certificates

## Alternatives

### Role Templating and `tbot` SPIFFE ID templating

One alternative to introducing a new resource is to introduce support for role
templating of the SPIFFE fields.

This would still require work to propagate the join metadata into the
certificate as trait-like things for the purposes of RBAC evaluation.

We'd also need to implement similar templating into the `tbot` configuration
itself, we could likely reuse much of the same functionality here.

This alternative would be slightly simpler to implement, but I think delivers
worse UX for a few reasons:

1. TBot configuration still remains important in the SVID issuance process,
  meaning you still need to manage configuration for a large fleet of `tbot`
  instances.
2. The configuration is split between Teleport resources and the `tbot` 
  configuration. If these drift, SVIDs will fail to be issued.

## Security Considerations