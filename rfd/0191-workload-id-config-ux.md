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

The attributes available for templating and rule evaluation will come from
three sources:

- `join`: From the attestation performed at join (e.g GitLab)
- `workload`: The workload attestation perfomed by `tbot`. This may not be 
  present for all configurations.
- `traits`: The traits sourced from the Bot or User.

For ease of operation, the attributes used during a workload identity credential
issuance will be recorded in the Teleport audit log.

Attributes names are always strings, using '.' to denote levels of a hierarchy,
e.g:

- `join.gitlab.project_path`
- `traits.external.email`
- `workload.unix.gid`

For simplicity, attribute values will typically be converted to a string value.

Metadata from the join process will be explicitly converted to attribute
key-values by Teleport's internals, rather than automatically converted. This
will provide us the opportunity to adjust any casting/naming for ease of
template or rule writing.

#### Templating Language

The existing templating language used for role templating will be re-used. This
is a simple language but provides the ability to perform basic manipulation of
attribute values.

#### Rules Evaluation

Each rule consists of a set of attribute keys and values. Each attribute within
the rule is compared to the attribute set of the requester. If a key is not
present in the attribute set of the requester, then the value is considered to
be an empty string.

TODO: Should we support some kind of globby/regexy matching?? Would this be 
better served by predicate expressions at a later date.

At a later date, the WorkloadIdentity resource could be extended to support
predicate expressions
(e.g [CEL](https://github.com/google/cel-spec/blob/master/doc/langdef.md)) for
more advanced rule construction. This is out of scope of the initial build.

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
    environment: production
spec:
  spiffe:
    # Results in spiffe://example.teleport.sh/gitlab/my-org/my-project/42
    id: "/gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.pipeline_id }}"
---
kind: role
metadata:
  name: production-workload-id
spec:
  allow:
    workload_identity_labels:
      environment: production
---
kind: bot
metadata:
  name: gitlab-workload-id
spec:
  roles:
  - production-workload-id
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

With the following `tbot` configuration:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identity_labels:
   '*': '*'
```

### Future: Ability to further customize workload identity credentials

In a future iteration, we'd introduce the ability for the `WorkloadIdentity`
resource to specify more than just the SPIFFE ID of the generated identity
credential.

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

### WorkloadIdentity Resource

The WorkloadIdentity resource abides the guidelines set out in
[RFD 153: Resource Guidelines](./0153-resource-guidelines.md).

```proto
// WorkloadIdentity represents a single, or group of similar, workload
// identities and configures the structure of workload identity credentials and
// authoirzation rules. is a resource that represents the configuration of a trust
// domain federation.
message WorkloadIdentity {
  // The kind of resource represented.
  string kind = 1;
  // Differentiates variations of the same kind. All resources should
  // contain one, even if it is never populated.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources share.
  teleport.header.v1.Metadata metadata = 4;
  // The configured properties of the WorkloadIdentity
  WorkloadIdentitySpec spec = 5;
}

// WorkloadIdentityRules holds the allow and deny authorization rules for the
// WorkloadIdentitySpec.
//
// Deny rules take precedence over allow rules.
message WorkloadIdentityRules {
  // Allow is a list of rules, each containing a set of attribute matchers.
  // For a rule to match, all matchers within the rule must match.
  // If rules are specified, then at least one rule must match for issuance to
  // be permitted.
  // If no rules are specified, issuance is permitted.
  repeated map<string, string> allow = 1;
  // Deny is a set of rules, each containing a set of attribute matchers.
  // For a rule to match, all matchers within the rule must match.
  // If rules are specified, then all rules must not match for issuance to be
  // permitted.
  // If no rules are specified, issuance is permitted.
  repeated map<string, string> deny = 2;
}

// WorkloadIdentitySPIFFE holds configuration for the issuance of
// SPIFFE-compatible workload identity credentials when this WorkloadIdentity
// is used.
message WorkloadIdentitySPIFFE {
  // Id is the path of the SPIFFE ID that will be issued in workload identity
  // credentials when this WorkloadIdentity is used. It can be templated using
  // attributes.
  //
  // Examples:
  // - `/no/templating/used` -> `spiffe://example.teleport.sh/no/templating/used`
  // - `/gitlab/{{ join.gitlab.project_path }}` -> `spiffe://example.teleport.sh/gitlab/org/project`
  string id = 1; 
}

// WorkloadIdentitySpec holds the configuration element of the WorkloadIdentity
// resource.
message WorkloadIdentitySpec {
  // Rules holds the authorization rules that must pass for this
  // WorkloadIdentity to be used. See [WorkloadIdentityRules] for further
  // documentation.
  WorkloadIdentityRules rules = 1;
  // SPIFFE holds configuration for the structure of SPIFFE-compatible 
  // workload identity credentials for this WorkloadIdentity. See 
  // [WorkloadIdentitySPIFFE] for further documentation.
  WorkloadIdentitySPIFFE spiffe = 2;
}
```

As per RFD 153, CRUD RPCs will be included for the WorkloadIdentity resource.
The ability to execute these RPCs will be governed by the standard verbs with a 
noun of `workload_identity`.

The proto specification of the RPCs is omitted for conciseness.

### Propagating Join Attributes into X509 Certificates

In order to use the metadata from the join process as part of rule evaluation
and templating, a few things need to be adjusted:

1. Upon join, the attested metadata must be converted to the equivelant
  attributes and persisted in the issued Teleport X509 user certificate.
2. When issuing Teleport X509 user certificates through role impersonation, the
  attributes should be propagated to the issued certificate if they exist in the
  original certificate.

The process of converting attested metadata to attribute key-values will be 
tailored to each individual join method. This is a suitable place for any 
adjustments to be made to the values and keys to optimize them for ease of 
templating and rule evaluation.

The attribute key-value set can be represented as a map, with the key being a
string and the value being a string.

The attribute key-value set will be encoded as JSON into the X509 certificate
using a new extension (1.3.9999.2.21).

Whilst out of scope of this RFD, the encoding of these values into the X509
certificate would enable the inclusion of join metadata within audit logs for
actions taken using Machine ID, allowing specific actions to be directly linked
to a specific join without correlating the Bot Instance ID from the audit event
with a `bot.join` event.

### New SVID Issuance RPC

```proto
// WorkloadIdentityService provides the signing of workload identity documents.
service WorkloadIdentityService {
  rpc IssueWorkloadIdentity(IssueWorkloadIdentityRequest) returns (IssueWorkloadIdentityResponse) {}
}

message WorkloadIdentityNameSelector {
  string name = 1;
}

message WorkloadIdentityLabelSelector {
  map<string, string> labels = 1;
}

message WorkloadAttestationAttributes {
  map<string, string> attributes = 2;
}

message IssueWorkloadIdentityRequest {
  // Selector.
  oneof selector {
    WorkloadIdentityNameSelector name = 1;
    WorkloadIdentityLabelSelector labels = 2;
  }
  WorkloadAttestationAttributes workload_attributes = 3;
}

message IssueWorkloadIdentityResponse {

}
```

When a specific WorkloadIdentity is specified by-name by the client:

1. Fetch WorkloadIdentity resource by name.
2. Compare `workload_identity_labels` of the client roleset against
  `metadata.labels` of the WorkloadIdentity. Reject if no access.
3. Perform rules evaluation of `spec.rules.deny` against the attributes of the
  client. Reject if no access.
4. If `spec.rules.allow` is non-zero, complete rules evaluation against the
  attributes of the client. Reject if no access.
5. Evaluate templated fields, if attribute included in template missing from
  attributes of the client, reject.
6. Issue workload identity credential, emit audit log, and return credential to
  client.

Where the client has provided label selectors:

1. Fetch all WorkloadIdentity resources.
2. For each WorkloadIdentity, compare client's requested labels against 
  `metadata.labels`, drop if no match.
3. For each remaining WorkloadIdentity, compare `workload_identity_labels` of
  client's roleset against `metadata.labels`, drop if no match.
4. For each remaining WorkloadIdentity:
4a. Perform rules evaluation of `spec.rules.deny` against the attributes of the
   client. Drop if no access.
4b. If `spec.rules.allow` is non-zero, complete rules evaluation against the
  attributes of the client. Drop if no access. 
4c. Evaluate templated fields, if attribute included in template missing from
  attributes of the client, drop. 
4d. Issue workload identity credential, emit audit log.
5. Return issued credentials to client. 

If, after evaluation of labels and rules, more than 10 WorkloadIdentity
resources remain, the client should receive an error encouraging the user to
leverage labels so that the set of returned WorkloadIdentity falls below this
threshold. This limits the runaway resource consumption if there is a 
misconfiguration.

### Performance

In the initial release, the standard resource cache will be used to improve the
performance of issuing identities to workloads.

The resource requirements of the initial design scale with the number of 
WorkloadIdentity resources in use. We should work closely with design partners
to determine the number of WorkloadIdentity resources in use, and proactively
perform benchmarks and performance improvements with growing magnitudes of 
scale in mind.

The flexible templating mechanism is intended to limit the number of Roles,
Bots and WorkloadIdentitys used for large-scale deployment. For example,
thousands of CI workflows can share the same Bot and WorkloadIdentity.
Where possible, we can mitigate the need for growing numbers of WorkloadIdentity
resources by adding more advanced templating and authorization rule
functionality.

We should bear in mind the following potential performance improvements:

- Tailored cache with label-based indexing to improve the performance of 
  resolving label matchers to specific WorkloadIdentity resources.
- Edge cache in `tbot` that can be enabled to return recently-issued identities
  without hitting the Auth Service. This will involve some elements of the
  authorization logic being executed at the edge for the cache.
- Decoupling the Workload Identity CA and issuance process from the Auth
  Service.

OpenTelemetry tracing should be included across the credential issuance process
to highlight performance hot-spots.

### Deprecation

For now, the old RBAC and RPCs for Workload Identity will be left in place as
they provide effectively an "administrative" way of generating workload 
identity credentials.

When the new mechanism has been available for 2 major versions, we should
revisit the deprecation of the legacy RPCs and RBAC.

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

### Auditing

The CRUD RPCs for the WorkloadIdentity resource will emit the standard audit 
events:

- `workload_identity.create`
- `workload_identity.update`
- `workload_identity.delete`

The new RPC for issuing a credential based on a WorkloadIdentity will emit a
new audit event `workload_identity.generate` which will contain the following
information:

- Connection Metadata
- The User or Bot Instance that has requested the issuance
- Details of the issued credential:
  - X509 SVID:
    - SPIFFE ID
    - Serial Number
    - NotBefore, NotAfter
    - Included SANs and Subject
    - Public Key
  - JWT SVID:
    - SPIFFE ID
    - Claims (e.g iat, exp, jti, sub, aud and any custom claims)
- The full attribute set that was used during rule and template evaluation.
- Which WorkloadIdentity was used for the issuance.

### Trustworthiness of Workload Attestation Attributes

The `workload` subset of the attributes is sourced from the workload attestation
performed by `tbot`. A malicious `tbot`, or someone with credentials exfiltrated
from `tbot`, is able to provide any values it desires for these attributes.

This is not a risk that has been introduced by the design proposed by this RFD
and is effectively an underlying concern of most workload identity
implementations that include an element of workload attestation that is 
completed using information that is local to the workload.

To make this risk easier to mitigate, care should be taken to distinguish 
attributes sourced from workload attestation against the more verifiable
attributes sourced from the join process. This has been achieved by placing
them under seperate root attribute keys (e.g `join` vs `workload`).

Largely, the blast radius of this risk is mitigated by the use of best practices
such as ensuring that attributes from the join are included in rules or 
templates with a higher precedence than that of attributes sourced from 
workload attestation. This limits a bad actors abiliy to issue credentials
to a subset of the workload identifier namespace. Authorization rules that
process workload identities should also abide this.

We should ensure that this risk and the best practices are explained in the
product documentation, and, provide a set of example WorkloadIdentity resources 
for common use-cases.

To simplify responding to this kind of breach, the audit log must include
sufficient information to trace back issued workload identities to the identity
that requested them.
