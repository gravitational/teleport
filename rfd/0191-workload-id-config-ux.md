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

At the core of the UX overhaul is the introduction of a new resource, the
WorkloadIdentity. This resource is designed to capture the configuration of
Teleport Workload Identity for a single workload, or, a group of similar 
workloads for which a structured identifier can be generated from a template.
It effectively acts as a template for the generation of workload identity
credentials (e.g SVIDs).

The WorkloadIdentity resource in its simplest form merely specifies an static
SPIFFE ID path - in this example - the resulting SPIFFE ID would be 
`spiffe://example.teleport.sh/my/awesome/identity`:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload-identity
  labels:
    env: production
spec:
  spiffe:
    id: /my/awesome/identity
```

As with most Teleport resources, a basic layer of RBAC based on the label 
matcher mechanism will be present. A user or bot must hold a role with the 
appropriate label matchers for a WorkloadIdentity in order to execute it:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload-identity
  labels:
    env: production
spec:
  spiffe:
    id: /my/awesome/identity
---
kind: role
metadata:
  name: production-workload-identity
spec:
  allow:
    workload_identity_labels:
      env: production
```

In addition to this resource effectively configuring the Teleport control
plane's authorization rules as to who and what workload identity credentials
can be issued, the WorkloadIdentity resource will also serve as a form of
remote configuration for the `tbot` workload identity agent - removing the need
for explicit configuration of workload identity credentials at the edge.

For example, to configure `tbot` to attempt to issue a specific
WorkloadIdentity:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identities:
  - my-workload-identity
```

Within the `tbot` configuration, the label mechanism can also be leveraged to
select multiple WorkloadIdentity instances:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identity_labels:
    env: production
```

This will attempt to issue any WorkloadIdentity that the Bot has access to that
matches the specified label.

If the Bot only has access to WorkloadIdentitys with a specific set of labels
by virtue of its role, then the label matcher within `tbot` will effectively
select a subset of these WorkloadIdentitys. This enables another configuration
mode using the '*' wildcard, configuring `tbot` to attempt to issue any
WorkloadIdentity to which it has access:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identity_labels:
    '*': '*'
```

### Templating and Rules

As well as being compatible with Teleport's label mechanism, the
WorkloadIdentity resource will also be governed by a more powerful authorization
mechanism internally that leverages attributes of the Bot or User's identity,
including attested metadata from the join or workload attestation.

The WorkloadIdentity resource will support templating using these attributes.
For example, attested metadata from the join process can be included in the
SPIFFE ID - in this case producing
`spiffe://example.teleport.sh/gitlab/my-org/my-project/staging`:

```yaml
kind: workload_identity
version: v1
metadata:
  name: default-gitlab
spec:
  spiffe:
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
```

Including an attribute within the WorkloadIdentity will implicitly restrict the
issuance of that WorkloadIdentity to workloads that possess this attribute. In
the given example, the WorkloadIdentity could only be used where a join
has occurred using the `gitlab` join method.

A more explicit form of restriction on issuance can also be set in the form
of "rules" based on the requester's attributes.

All specified attributes within a rule must match the attributes of the
requester for the rule to be considered to match. Where multiple rules are
specified, only one rule must match. This gives a logical AND within a rule
and a logical OR across rules within a set.

Rules may either be "allow" or "deny". As with the rest of Teleport RBAC,
"deny" rules take precedence over "allow" rules.

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload-identity
spec:
  rules:
    allow:
    # (
      # The CI must be running in the "foo" namespace in GitLab
    - join.gitlab.namespace_path: "foo"
      # AND
      # The CI must be running against the "special" environment
      join.gitlab.environment: "special"
    # ) OR (
      # The CI must be running in the "bar" namespace in GitLab
    - join.gitlab.namespace_path: "bar"
    # )
    deny:
    # The CI must not be running against the "dev" environment
    - join.gitlab.environment: "dev"
  spiffe:
    # Implicitly, this WorkloadIdentity can only be issued to requester's
    # with a successful join via the GitLab join method.
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
```

Altogether, the following things are considered in the authorization of issuing
a credential from a WorkloadIdentity:

1. Does the requester hold a role which grants access to the WorkloadIdentity
  via label matchers?
2. Does the requester's attributes not match any deny rule?
3. Does the requester's attributes match any allow rule?
4. Does the requester have the appropriate attributes for templating to succeed?

Whilst this seems complex, in a majority of cases, the label matching
functionality may effectively be disabled by assigning a single role including
only a wildcard WorkloadIdentity label matcher. This simplifies usage by only
considering the explicit and implicit attribute based rules - which are a better
fit for majority of workload identity uses.

#### Attributes

#### Templating Language

#### Rules Evaluation

### Revisiting our use-case

To give the example of our GitLab use-case, all workflows can now share a single
Bot, Role and Join Token - and they will automatically issue the correct 
SPIFFE ID in-line with the centralized policy.

```yaml
kind: workload_identity
version: v1
metadata:
  name: gitlab
  labels:
    platform: gitlab
spec:
  spiffe:
    # Results in spiffe://example.teleport.sh/gitlab/my-org/my-project/42
    id: "/gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.pipeline_id }}"
---
kind: role
metadata:
  name: gitlab-workload-id
spec:
  allow:
    workload_identity_labels:
      platform: gitlab
---
kind: bot
metadata:
  name: gitlab-workload-id
spec:
  roles:
  - gitlab-workload-id
---
kind: token
version: v2
metadata:
  name: gitlab-workload-id
spec:
  roles: [Bot]
  join_method: gitlab
  bot_name: gitlab-workload-id
  gitlab:
    domain: gitlab.example.com
    # Allow rules are intentionally very permissive as we'd like to allow any
    # CI run to request a WorkloadIdentity using the template.
    allow:
    - namespace_path: my-org
```

The profile can then explicitly be specified inside of the `tbot` configuration:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  profiles:
  - gitlab
```

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
kind: workload_identity
version: v1
metadata:
  name: gitlab
spec:
  spiffe:
    id: "/gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.pipeline_id }}"
    jwt:
      extra_claims:
        "ref_path": {{ join.gitlab.ref_path }}
```

TODO: What about an option to map all attestation values into a claim - rather
than requiring it to be attribute by attribute?

## Technical Design

### Encoding Join Metadata into Certificates

## Alternatives

### Role Templating and `tbot` SPIFFE ID templating

One alternative to introducing a new resource is to introduce support for role
templating of the SPIFFE fields.

This would still require work to propagate the join metadata into the
certificate as trait-like things for the purposes of RBAC evaluation.

We'd also need to implement similar templating into the `tbot` configuration
itself, we could likely reuse much of the same functionality here.

Positives:

1. "Simpler" to implement (but not significantly so)
2. Keeps configuration within an existing Teleport primitive (roles) rather than
  further spreading it out into a new resource.

Negatives:

1. TBot configuration still remains important in the SVID issuance process,
  meaning you still need to manage configuration for a large fleet of `tbot`
  instances.
2. The configuration is split between Teleport resources and the `tbot` 
  configuration. If these drift, SVIDs will fail to be issued.
3. Teleport Roles are already pretty complex, and this is introducing further
  complexity.

## Security Considerations