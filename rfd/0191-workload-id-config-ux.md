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
    # Provide the path element of the workload identifier (SPIFFE ID).
    id: /my/awesome/identity
    # Optionally provide a "hint" that will be provided to workloads using the
    # SPIFFE Workload API to fetch workload identity credentials.
    hint: my-hint
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

### Templating

Certain fields, such as `spec.spiffe.id`, will support templating using a
set of attributes related to the Bot's Join, the Bot resource itself and any
workload attestation that has been completed by the `tbot` agent.

For example:

```yaml
kind: workload_identity
version: v1
metadata:
  name: gitlab
spec:
  spiffe:
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
```

This WorkloadIdentity would result in a SPIFFE ID of
`spiffe://example.teleport/sh/gitlab/my-org/my-project/production` if the Bot
had joined with the GitLab join method from a CI pipeline in the
`my-org/my-project` repository which was configured to deploy to the
`production` environment.

Including an attribute within the WorkloadIdentity will implicitly restrict the
issuance of that WorkloadIdentity to workloads that possess this attribute. In
the given example, the WorkloadIdentity could only be used where a join
has occurred using the `gitlab` join method.

### Rules

To restrict the issuance of a WorkloadIdentity based on attributes from the
join, workload attestation or the Bot itself, a set of explicit rules can be
set within the WorkloadIdentity resource:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload-identity
spec:
  spiffe:
    # Implicitly, this WorkloadIdentity can only be issued to requester's
    # with a successful join via the GitLab join method.
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
  rules:
    allow:
    - conditions:
        # The CI must be running in the "foo" namespace in GitLab
      - attribute: join.gitlab.namespace_path
        equals_string: foo
        # AND
        # The CI must be running against the "special" environment 
      - attribute: join.gitlab.environment
        equals_string: special
      # OR
    - conditions:
        # The CI must be running in the "bar" namespace in GitLab
      - attribute: join.gitlab.namespace_path
        equals_string: bar
    deny:
    - conditions:
        # The CI must not be running against the "dev" environment.
      - attribute: join.gitlab.environment
        equals_string: dev
```

Each WorkloadIdentity resource has two rulesets - `allow` and `deny`.

If the `allow` ruleset is non-empty, then at least one rule within the set must
return true for the WorkloadIdentity to be issued.

If the `deny` ruleset is non-empty, then all rules within the set must return
false for the WorkloadIdentity to be issued.

Each rule consists of either a set of conditions or an expression. A rule 
cannot have both conditions and an expression.

When conditions have been provided within a rule, then all conditions within the
rule must return true for the rule to return true. A condition consists of a
single target attribute, a single operator and a single expected value.

The following operators will be supported:

- `equals_string`: The target attribute must be a string and equal the given
  string.
- `matches_string`: The target attribute must be a string and match the given
  regex pattern.
- `not_equals_string`: The target attribute must be a string and must not equal
  the given string.
- `not_matches_string`: The target attribute must be a string and must not match
  the given regex pattern.
- `present`: The target attribute must be set.
- `not_present`: The target attribute must not be set. 

Alternatively, a rule may consist of a single expression. This expression is
configured by the user in the predicate language with access to the same
attributes as the conditions and templating. This expression must return a 
boolean value. For example:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload-identity
spec:
  spiffe:
    # Implicitly, this WorkloadIdentity can only be issued to requester's
    # with a successful join via the GitLab join method.
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
  rules:
    allow:
    - expression: join.gitlab.environment == "production"
```

See "Predicate Language" for further details.

### Attributes

The attributes available for templating and rule evaluation will come from
three sources:

- `join`: From the attestation performed when the bot joined. This will be 
  specific to the join method used. E.g `join.gitlab.project_path`.
- `workload`: From the workload attestation performed by `tbot`. The presence of 
  these attributes is dependent on whether workload attestation has been
  performed by `tbot` and which form of attestation has been performed. E.g
  `workload.k8s.namespace`.
- `traits`: From the traits configured on the Bot which is requesting the
  issuance of the WorkloadIdentity. E.g `traits.foo`.

Attributes take a hierarchical form which can be navigated within templates and
rules using `.`. For example, `join.gitlab.project_path`.

Documentation will be generated with an exhaustive list of available attributes
and example values.

The audit log entry for the issuance of a WorkloadIdentity will include the 
full attribute set that was used during rule and template evaluation. This will
not only serve as a tool for compliance and auditing, but, will also provide
the necessary information to debug issues with the configuration of templates
and rules.

### `tbot` Configuration

In addition to this resource configuring the Teleport control plane's
authorization rules as to who and what workload identity credentials
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

Label matchers can be specified within the `tbot` configuration to issue
any WorkloadIdentity which matches those labels and to which the Bot has access: 

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identity_labels:
    env: production
```

The "wildcard" label matcher can be used within the `tbot` configuration to
issue any WorkloadIdentity to which the Bot has access:

```yaml
services:
- type: spiffe-workload-api
  listen: unix:///opt/machine-id/workload.sock
  workload_identity_labels:
    '*': '*'
```

These configuration values may also be provided using the "zero-config" CLI
flags for `tbot`:

```shell
$ tbot start workload-identity \
  --proxy-server example.teleport.sh:443 \
  --join-token my-join-token \
  --workload-identity my-workload-identity \
  --destination /opt/workload-id

$ tbot start workload-identity \
  --proxy-server example.teleport.sh:443 \
  --join-token my-join-token \
  --workload-identity-labels env:production \
  --destination /opt/workload-id

 $ tbot start workload-identity \
  --proxy-server example.teleport.sh:443 \
  --join-token my-join-token \
  --workload-identity-labels *:* \
  --destination /opt/workload-id
```

### Configuration Tooling 

The WorkloadIdentity resource will support configuration via four methods:

- The `tctl create`/`tctl update` commands with a YAML representation 
- The Teleport Terraform Provider
- The Teleport Kubernetes Operator as Kubernetes CRDs
- The Teleport gRPC API

### Diagnostics Tooling

The flexibility of the templating, rules and attributes system allows 
potentially complex configurations to be created. As such, we must provide
tooling to assist operators in testing and debugging WorkloadIdentity
configurations.

The `tctl` CLI will be extended with a new command
`tctl workload-identity test` that will locally evaluate a WorkloadIdentity,
or set of WorkloadIdentities, against a given set of attributes and return a
description of the workload identity credentials that would have been issued.

Operators will be able to specify a WorkloadIdentity from a local file which
may not yet exist in the Teleport cluster, or specify an existing
WorkloadIdentity by name.

Operators will provide the attributes as a file in YAML or JSON format. This
will be in the same structure as the attributes included in the Teleport
audit log.

For example:

```shell
$ tctl workload-identity test --workload-identity-file ./my-workload-identity.yaml --attributes-file ./attributes.yaml
Evaluating 3 WorkloadIdentity resources against the following attributes:
--snip--

1 WorkloadIdentity resources matched the given attributes. The following
credentials would have been issued:

- workload_identity_name: gitlab-production
  spiffe:
    id: spiffe://example.teleport.sh/gitlab/my-org/my-project/production
    jwt:
      sub: spiffe://example.teleport.sh/gitlab/my-org/my-project/production
      iss: https://example.teleport.sh
      -- snipped for conciseness: all JWT claims would be included --
    x509:
      subject:
        common_name: spiffe://example.teleport.sh/gitlab/my-org/my-project/production
        organization: my-org
        -- snipped for conciseness: all Subject DN fields would be included --
      sans:
      - dns: my-project.example.com
      - ip: 10.0.0.1
      - uri: spiffe://example.teleport.sh/gitlab/my-org/my-project/production 
2 WorkloadIdentity resources did not match the given attributes:

- workload_identity_name: gitlab-staging
  reason: Rule evaluation failed. Allow rule condition `join.gitlab.environment == "production"` returned false.
- workload_identity_name: github-production
  reason: The `join.github.environment` attribute included in `spec.spiffe.id` template do not exist in the attribute set.
```

It stands to reason that this functionality could eventually be integrated into 
the Teleport Web UI.

### Future: Customizing workload identity credentials

In a future iteration, we'd introduce the ability for the `WorkloadIdentity`
resource to specify more than just the SPIFFE ID/workload identifier of the
generated identity credential.

This ability serves many purposes:

- Allow the encoding of additional information that is not considered for
  authorization but may be useful to be included in workload audit logs.
- Allow the encoding of additional information that is only rarely used for
  authorization - or - does not fit well into the structure of a SPIFFE ID for
  other reasons.
- Support compatibility with services which may require information to be
  encoded into non-standard fields.

The fields for controlling customization of the credentials would also accept
the same templating logic as `spec.spiffe.id`.

For JWTs, the following options are good candidates for inclusion:

- Additional claims using information from attributes, or, encoding all
  attributes into a specific claim.
- Constraining acceptable audiences
- Inclusion or exclusion of standard claims (e.g `jti`)

For X509 certificates, the following options are good candidates for inclusion:

- Additional SANs
- Control of the Subject CN behaviour (e.g should the CN match the SPIFFE ID or
  the first DNS SAN)
- Overriding Subject DN entirely (e.g populate DN attributes with values from
  the requester's attributes)

The following example configuration is given for demonstrative purposes and
does not represent a decision on field naming or structure - which is out of
scope of this RFD:

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
        "gitlab_ref_path": {{ join.gitlab.ref_path }}
    x509:
      override_subject:
        cn: my-lovely-common-name
      additional_sans:
        dns:
        - example.com
```

We may wish to consider a second-order configuration resource
(e.g a WorkloadIdentityProfile) which would allow customization settings to be
shared between WorkloadIdentity resources. This would provide a unified way
to control customizations which may be necessary for compliance or compatability
purposes.

### Revisiting our use-case

Let's revisit the use-case we outlined at the beginning of this RFD.

Now, the operator must only configure a single WorkloadIdentity, Bot, Join Token
and Role:

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

The `tbot` configuration is now identical across all workflows and
consists of just a handful of flags:

```yaml
tbot start workload-api \
  --proxy-server example.teleport.sh:443 \
  --join-token gitlab-workload-id \
  --join-method gitlab \
  --workload-identity-labels "'*':'*'"  \
  --listen-addr unix:///opt/workload.sock
```

## Technical Design

### Attributes

For the purposes of templating, rule evaluation and auditing, a set of
attributes is derived during the issuance of a WorkloadIdentity.

Attributes are organized into a hierarchical structure, with three top-level
keys:

- `join`
- `workload`
- `traits`

The attribute hierarchy will be defined using Protobuf. This will enable:

- Generation of documentation.
- Strong typing of rules, templates and expressions provided by users.
- Serialization and deserialization of attributes for the purposes of
  persistence, auditing or transport.

```protobuf
package teleport.workloadidentity.v1;

message AttributeSet {
  JoinAttributes join = 1;
  WorkloadAttributes workload = 2;
  TraitAttributes traits = 3;
}
```

#### Trait Attributes

The trait attributes are extracted during the RPC invocation from the X509
identity of the caller.

These values provide a simple way to attach specific values to a specific
Bot, which may not be available through any attestation process. These may
have organizational or administrative significance.

```protobuf
package teleport.workloadidentity.v1;

message TraitAttributes {
  // TODO: Work out how to encode this.
}
```

#### Workload Attributes

The workload attributes are provided by the `tbot` agent at the time of
attestation as part of the RPC request.

Depending on whether workload attestation has been performed, or which form of
attestation has been performed, the attributes available will differ.

```protobuf
package teleport.workloadidentity.v1;

message WorkloadAttributes {
  // TODO: Work out how to encode this.
}
```

#### Join Attributes

The `join` attributes will be encoded into

```protobuf
package teleport.workloadidentity.v1;

message JoinAttributes {
  // TODO: Work out how to encode this.
}
```

##### Propagating Join Attributes

In order to access the join attributes during WorkloadIdentity evaluation,
we will need to persist them in some form.

This persistence must remain over:

- Renewals of the certificate using the GenerateUserCert RPC.
- The generation of role impersonated certificates using the GenerateUserCert
  RPC.

To achieve this persistence, the JoinAttributes protobuf message will be
encoded using `protojson.Marshal` and stored within the X509 certificate using
a new extension - `1.3.9999.2.21`. When unmarshalling, unknown fields should be
ignored to ensure forwards compatability.

The GenerateUserCert RPC will be modified to propagate the JoinAttributes,
if present, to any certificates issued.

The Join RPCs will be modified to pass the newly generated JoinAttributes to
the certificate generation internals.

Whilst out of scope for this RFD, the persistence of the JoinAttributes into
the X509 certificate could be leveraged in future work to provide a richer
metadata for audit logging of Bot actions.

### WorkloadIdentity Resource

The WorkloadIdentity resource abides the guidelines set out in
[RFD 153: Resource Guidelines](./0153-resource-guidelines.md).

```protobuf
syntax = "proto3";

package teleport.workloadidentity.v1;

// WorkloadIdentity represents a single, or group of similar, workload
// identities and configures the structure of workload identity credentials and
// authorization rules. is a resource that represents the configuration of a trust
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

message WorkloadIdentityCondition {
    // The attribute to evaluate.
    string attribute = 1;
    string equals_string = 2;
    string matches_string = 3;
    string not_equals_string = 4;
    string not_matches_string = 5;
}

// WorkloadIdentityRule is an individual rule which can be evaluated against
// the context attributes to produce a boolean result.
//
// A rule contains either a set of conditions or an expression. A rule cannot
// contain both conditions and an expression.
//
// When conditions are provided, the rule evaluates to true if all conditions
// evaluate to true.
message WorkloadIdentityRule {
  repeated WorkloadIdentityCondition conditions = 1;
  string expression = 2;
}


// WorkloadIdentityRules holds the allow and deny authorization rules for the
// WorkloadIdentitySpec.
//
// Deny rules take precedence over allow rules.
message WorkloadIdentityRules {
  // Allow is a list of rules. If any rule evaluate to true, then the allow
  // ruleset is consdered satisfied.
  //
  // If the allow ruleset is empty, then the allow ruleset is considered to be
  // satisfied.
  repeated WorkloadIdentityRule allow = 1;
  // Deny is a list of rules. If any rule evaluates to true, then the
  // WorkloadIdentity cannot be issued.
  repeated WorkloadIdentityRule deny = 2;
}

// WorkloadIdentitySPIFFEX509 holds configuration specific to the issuance of
// an X509 SVID from a WorkloadIdentity.
message WorkloadIdentitySPIFFEX509 {
}

// WorkloadIdentitySPIFFEJWT holds configuration specific to the issuance of
// a JWT SVID from a WorkloadIdentity.
message WorkloadIdentitySPIFFEJWT {
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
  // Hint is an optional hint that will be provided to workloads using the
  // SPIFFE Workload API to fetch workload identity credentials.
  string hint = 2;
  // x509 holds configuration specific to the issuance of X509 SVIDs from this
  // WorkloadIdentity resource.
  WorkloadIdentitySPIFFEX509 x509 = 3;
  // jwt holds configuration specific to the issuance of JWT SVIDs from this
  // WorkloadIdentity resource.
  WorkloadIdentitySPIFFEJWT jwt = 4;
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

### IssueWorkloadIdentity RPC 

```proto
// WorkloadIdentityService provides the signing of workload identity documents.
service WorkloadIdentityService {
  rpc IssueWorkloadIdentity(IssueWorkloadIdentityRequest) returns (IssueWorkloadIdentityResponse) {}
}

message WorkloadIdentityNameSelector {
  string name = 1;
}

message WorkloadIdentityLabelSelector {
  string key = 1;
  repeated string values = 2;
}

message WorkloadIdentityLabelSelectors {
  repeated WorkloadIdentityLabelSelector = 1;
}

message WorkloadAttestationAttributes {
  map<string, string> attributes = 2;
}

message IssueWorkloadIdentityJWTSVIDRequest {
  // The value that should be included in the JWT SVID as the `aud` claim.
  // Required.
  repeated string audiences = 1;
  // The requested TTL for the JWT SVID. This request may be modified by
  // the server according to its policies. It is the client's responsibility
  // to check the TTL of the returned workload identity credential.
  google.protobuf.Duration ttl = 2; 
}

message IssueWorkloadIdentityX509SVIDRequest {
    // A PKIX, ASN.1 DER encoded public key that should be included in the x509
  // SVID.
  // Required.
  bytes public_key = 1;
  // The requested TTL for the JWT SVID. This request may be modified by
  // the server according to its policies. It is the client's responsibility
  // to check the TTL of the returned workload identity credential.
  google.protobuf.Duration ttl = 2; 
}

message IssueWorkloadIdentityRequest {
  oneof selector {
    // Request a specific WorkloadIdentity by name.
    WorkloadIdentityNameSelector name = 1;
    // Request WorkloadIdentity's by label matcher. All specified labels must
    // exist on the WorkloadIdentity for it to be selected.
    WorkloadIdentityLabelSelector labels = 2;
  }
  oneof type {
    IssueWorkloadIdentityJWTSVIDRequest jwt_svid = 3;
    IssueWorkloadIdentityX509SVIDRequest x509_svid = 4;
  }

  WorkloadAttestationAttributes workload_attributes = 3;
}

message WorkloadIdentityCredential {
  oneof credential {
    // Signed JWT-SVID
    string jwt_svid = 1;
    // An ASN.1 DER encoded X509-SVID
    bytes x509_svid = 2;
  }
  // The TTL that was chosen by the server.
  google.protobuf.Duration ttl = 3;
  // The time that the TTL is reached for this credential.
  google.protobuf.Timestamp expiry = 4;  
  // The hint configured for this Workload Identity - if any. This is provided
  // to workloads using the SPIFFE Workload API to fetch credentials.
  string hint = 5;

  // TODO: Is it worth returning the full associated WorkloadIdentity resource?
  string workload_identity_name = 2;
}

message IssueWorkloadIdentityResponse {
  repeated WorkloadIdentityCredential credentials = 1;
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

### Predicate Language

For the purposes of rule evaluation and templating, a predicate language and
engine must be implemented. 

Today, Teleport already includes a predicate language engine that serves
extremely similar functionality (e.g role templating, login rules).

This engine and language will be reused.

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

### Implementation Pathway

This work is well-suited to being broken down into a number of smaller tasks: 

1. Implement the persistence of join metadata as attributes into X509
  certificates.
2. Implement the WorkloadIdentity resource and CRUD RPCs.
3. Implement the IssueWorkloadIdentity RPC without templating or rules
  evaluation.
4. Implement rules and template evaluation.
5. Implement label-based IssueWorkloadIdentity RPC functionality.
6. Implement changes in `tbot` to support using the IssueWorkloadIdentity RPC.
7. Implement the `tctl workload-identity test` command.

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

### CEL vs Teleport Predicate Language

Rather than use the existing Teleport predicate language, we could base the
templating and rule evaluation functionality in Workload Identity on Google's
open-source Common Expression Language (CEL).

Positives:

- A more widely-used and understood language. Users are more likely to have
  encountered this previously, and, there exists a wealth of tooling and
  documentation.
- Native support for encoding compiled expressions in Protobuf. This would 
  support the pre-compilation of expressions at creation time for the purposes
  of validation and performance.
- Native support for environments/contexts defined in Protobuf. Our AttributeSet
  will be defined in Protobuf and would natively be supported by CEL. We would
  need to extend the Teleport predicate language to support this.

Negatives:

- Inconsistency with other functionality in Teleport may be confusing for users.
- We already have internal expertise about the functionality of the Teleport
  predicate language and understand the existing limitations of it, CEL
  introduces a new set of limitations and challenges, many of which are unknown
  to us. That being said, CEL is more mature and battle-tested, it's unlikely
  we will encounter problems which are not already well-documented.

## Security Considerations

### Auditing

The CRUD RPCs for the WorkloadIdentity resource will emit the standard audit 
events:

- `workload_identity.create`
- `workload_identity.update`
- `workload_identity.delete`

The new RPC for issuing a credential based on a WorkloadIdentity will emit a
new audit event `workload_identity.generate` for each issued identity. This will
contain the following information:

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
them under separate root attribute keys (e.g `join` vs `workload`).

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
