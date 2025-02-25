---
authors: Noah Stride (noah@goteleport.com)
state: implemented (17.2.7)
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
- User Metadata
  - This is a looser category, but consider traits, labels and name of the
    User/Bot requesting the generation of the identity. This information doesn't
    come from an attestation process, but is typically "administrative" and set
    by the operator.

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
set of attributes from the attestation during Bot's Join, the Bot resource
itself and any workload attestation that has been completed by the `tbot` agent.

To use templating, the name of an attribute is enclosed between `{{` and `}}`.

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
        equals: foo
        # AND
        # The CI must be running against the "special" environment 
      - attribute: join.gitlab.environment
        equals: special
      # OR
    - conditions:
        # The CI must be running in the "bar" namespace in GitLab
      - attribute: join.gitlab.namespace_path
        equals: bar
    deny:
    - conditions:
        # The CI must not be running against the "dev" environment.
      - attribute: join.gitlab.environment
        equals: dev
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

Initially, the following operators will be supported:

- `equals`: The target attribute must equal the given value. 
- `not_equals`: The target attribute must not equal the given value 
- `matches`: The target attribute must be a string and must match the
  given regex pattern.
- `not_matches`: The target attribute must be a string and must not match
  the given regex pattern.
- `in`: The target attribute must equal one of the values in the given list.
- `not_in`: The target attribute must not equal any of the values in the given
  list.

Future iterations may introduce additional operators as we better understand the
types of conditions that will be useful to users.

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
- `user`: From the identity of the user calling the WorkloadIdentity API. E.g
  `user.bot_name` or `user.traits`. This information does not directly come from
  attestation but may be useful for referencing administrative or organizational
  values.

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
  workload_identity: my-workload-identity
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

The `workload_identity` and `workload_identity_labels` fields are mutually
exclusive, that is, a service may be configured with either a specific
WorkloadIdentity or with a set of labels to match against WorkloadIdentity
resources.

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
  reason: Rule evaluation failed. Allow rule expression `join.gitlab.environment == "production"` returned false.
- workload_identity_name: github-production
  reason: The `join.github.environment` attribute included in `spec.spiffe.id` template do not exist in the attribute set.
```

Similar functionality should be built into the Teleport Web UI. This should
initially be built into the WorkloadIdentity resource view, and allow the
resource to be tested against a set of attributes derived from an existing
Bot Instance resource.

Later, this functionality can be built into the "Create or Edit" flow for a 
WorkloadIdentity resource.

### X509 SVIDs and DNS SANs

By default, the X509 SVIDs issued for a WorkloadIdentity resource will not 
include any DNS SANs.

To include DNS SANs, the `spec.spiffe.x509.dns_sans` field can be used:

```yaml
kind: workload_identity
version: v1
metadata:
  name: gitlab
spec:
  spiffe:
    id: /my-awesome-service
    x509:
      dns_sans:
      - my.awesome.service.example.com
      - "*.my.awesome.service.example.com"
```

This will result in any X509 SVID issued using this WorkloadIdentity including
both the specified DNS SANs.

As with `spec.spiffe.id`, templating can be used to include attributes within
DNS SANs:

```yaml
kind: workload_identity
version: v1
metadata:
  name: gitlab
spec:
  spiffe:
    id: /my-awesome-service
    x509:
      dns_sans:
      - {{ join.gitlab.environment }}.gitlab.example.com 
```

### Time To Live (TTL)

Both X509 and JWT SVIDs include fields which control when the credential is
valid from and when it is valid to.

When requesting the issuance of a credential, the `tbot` agent will provide a
requested TTL for the credential - which will either be a default value or
one explicitly configured by a user.

However, the WorkloadIdentity resource will also include a `spec.spiffe.ttl.max`
field. If `tbot` requests a credential with a TTL greater than this value, the
TTL will be capped at this value. This provides the ability to enforce a 
maximum permissible TTL regardless of the configuration of the `tbot` agent.

If this field is not set, a default maximum value of 24 hours will be used.

Example:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-awesome-service 
spec:
  spiffe:
    id: /my-awesome-service
    ttl:
      max: 24h
```

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
      dns_sans:
      - example.com
```

We may wish to consider a second-order configuration resource
(e.g a WorkloadIdentityProfile) which would allow customization settings to be
shared between WorkloadIdentity resources. This would provide a unified way
to control customizations which may be necessary for compliance or compatibility
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
  --workload-identity-labels *:* \
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
- `user`

The attribute hierarchy will be defined using Protobuf. This will enable:

- Generation of documentation.
- Strong typing of rules, templates and expressions provided by users.
- Serialization and deserialization of attributes for the purposes of
  persistence, auditing or transport.

Specification:

```protobuf
package teleport.workloadidentity.v1;

// AttributeSet is the top-level structure that contains all attributes that
// are considered during the issuance of a WorkloadIdentity.
message AttributeSet {
  // Attributes pertaining to the join process.
  JoinAttributes join = 1;
  // Attributes pertaining to the workload attestation process.
  WorkloadAttributes workload = 2;
  // Attributes pertaining to the user that is requesting the WorkloadIdentity. 
  UserAttributes user  = 3;
}
```

Example (marshalled as YAML):

```yaml
join:
  meta:
    token_name: my-gitlab-join-token
    method: gitlab
  gitlab:
    project_path: my-org/my-project
    pipeline_id: 42
    # -- snipped for conciseness --
workload:
  unix:
    attested: true
    pid: 1234
    uid: 1000
    gid: 1000
  k8s:
    attested: true
    pod_name: my-pod-ab837c
    namespace: my-namespace
    service_account: my-service-account
user:
  name: bot-gitlab-workload-identity
  is_bot: true
  bot_name: gitlab-workload-identity
  traits:
  - name: foo
    values:
    - bar
    - bizz
  - name: logins
    values:
    - root
```

#### User Attributes

The user attributes are extracted during the RPC invocation from the X509
identity of the caller, and include information such as the name of the user,
whether the user is a bot, and any traits attached to that user.

Specification:

```protobuf
package teleport.workloadidentity.v1;

// Attributes pertaining to the user that is requesting the WorkloadIdentity. 
message UserAttributes {
  // Name of the user requesting the generation of the WorkloadIdentity.
  // Example: `bot-gitlab-workload-identity`
  string name = 1;
  // Whether or not the user requesting the generation of the WorkloadIdentity
  // is a bot.
  bool is_bot = 2;
  // If the user is a bot, the name of the bot.
  // Example: `gitlab-workload-identity`
  string bot_name = 3;
  // The traits of the user configured within Teleport or determined during
  // SSO.
  Traits traits = 4;
  // The ID of the Bot Instance, should this user be a bot. Otherwise empty.
  string bot_instance_id = 5;
}
```

Example (marshalled as YAML):

```yaml
name: bot-gitlab-workload-identity
is_bot: true
bot_name: gitlab-workload-identity
traits:
- name: foo
  values:
  - bar
  - bizz
- name: logins
  values:
  - root
```

#### Workload Attributes

The workload attributes are provided by the `tbot` agent at the time of
attestation as part of the RPC request.

Depending on whether workload attestation has been performed, or which form of
attestation has been performed, the attributes available will differ.

```protobuf
package teleport.workloadidentity.v1;

// Attributes pertaining to unix workload attestation.
message WorkloadUnix {
  // Whether or not this attestation method has been performed successfully.
  bool attested = 1;
  // The PID of the process that connected to the workload API.
  int32 pid = 2;
  // The UID of the process that connected to the workload API.
  uint32 uid = 3;
  // The GID of the process that connected to the workload API.
  uint32 gid = 4;
  // -- snipped for conciseness --
}

// Attributes pertaining to Kubernetes workload attestation.
message WorkloadKubernetes {
  // Whether or not this attestation method has been performed successfully.
  bool attested = 1;
  // The name of the pod that connected to the workload API.
  string pod_name = 2;
  // The namespace of the pod that connected to the workload API.
  string namespace = 3;
  // The service account of the pod that connected to the workload API.
  string service_account = 4;
  // -- snipped for conciseness --
}

// Attributes sourced from the workload attestation process.
message WorkloadAttributes {
  // Attributes pertaining to Unix workload attestation.
  WorkloadUnix unix = 1;
  // Attributes pertaining to Kubernetes workload attestation.
  WorkloadKubernetes k8s = 2;
}
```

Example (marshalled as YAML):

```yaml
unix:
  attested: true
  pid: 1234
  uid: 1000
  gid: 1000
k8s:
  attested: true
  pod_name: my-pod-ab837c
  namespace: my-namespace
  service_account: my-service-account
```

#### Join Attributes

The `join` attributes are sourced from the attestation process that occurs when
a principal (Bot, Agent etc) initially authenticates using a Join Token.

At the top level of the join attributes, the `meta` key will hold attributes
that apply to all joins. For example, the name of the join token used and the
method of the join.

A top level key will exist for each join method that is supported by Teleport.
Under this key, attributes specific to that join method will be present.
Typically, these attributes will be sourced directly from some identity
document provided by the principal upon joining (e.g an ID Token issued by
GitLab).

Specification:

```protobuf
package teleport.workloadidentity.v1;

// Attributes present for all joins, including metadata about the join token
// itself.
message JoinMeta {
  // The name of the join token used to join. If the method of the join is
  // `token`, then this field will not be set as the name of the join token
  // is sensitive.
  string token_name = 1;
  // The join method that was used, e.g `gitlab`, `github`, `token`.
  string method = 2;
}

// Attributes specific to the GitLab join method.
message JoinGitLab {
  // The path of the project in GitLab that the CI pipeline is running in.
  // For example `strideynet/my-project`.
  string project_path = 1;
  // The namespace path of the project in GitLab that the CI pipeline is running
  // in.
  // For example `strideynet`.
  string namespace_path = 2;
  // -- snipped for conciseness --
}

// Attributes specific to the TPM join method.
message JoinTPM {
  // The hash of the EKPub presented during the join process
  string ekpub_hash = 1;
  // The description of the join token rule that matched during the join
  // process.
  string rule_description = 2;
  // -- snipped for conciseness --
}

// Attributes sourced from the join process.
message JoinAttributes {
  // Attributes of the join token itself (e.g name, method)
  JoinMeta meta = 1;
  // Attributes specific to the GitLab join method.
  JoinGitLab gitlab = 2;
  // Attributes specific to the TPM join method.
  JoinTPM tpm = 3;
  // Snipped for conciseness. A field will exist for each join method.
}
```

Example (marshalled as YAML):

```yaml
meta:
  token_name: my-gitlab-join-token
  method: gitlab
gitlab:
  project_path: my-org/my-project
  namespace_path: my-org
  # -- snipped for conciseness --
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
a new extension - `1.3.9999.2.21`. When unmarshaling, unknown fields should be
ignored to ensure forwards compatibility.

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

// The individual condition of a rule. A condition consists of a single target
// attribute to evaluate against and an operator to use when evaluating.
// Only one operator field should be set.
message WorkloadIdentityCondition {
  // The attribute name to evaluate.
  // Example: `join.gitlab.project_path`
  string attribute = 1;
  // The exact string the attribute value must match.
  string equals = 2;
  // The regex pattern the attribute value must match.
  string matches = 3;
  // The exact string the attribute value must not match.
  string not_equals = 4;
  // The regex pattern the attribute value must not match.
  string not_matches = 5;
  // A list of strings that the attribute value must equal at least one of. 
  repeated string in = 6;
  // A list of strings that the attribute value must not equal any of.
  repeated string not_in = 7;
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
  // The set of conditions to evaluate. All condition may return true for this
  // rule to return true.
  repeated WorkloadIdentityCondition conditions = 1;
  // The predicate language expression to evaluate. Mutually exclusive with
  // conditions.
  string expression = 2;
}


// WorkloadIdentityRules holds the allow and deny authorization rules for the
// WorkloadIdentitySpec.
//
// Deny rules take precedence over allow rules.
message WorkloadIdentityRules {
  // Allow is a list of rules. If any rule evaluate to true, then the allow
  // ruleset is considered satisfied.
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
  // The DNS SANs to include in the issued X509 SVID. Each provided entry
  // supports templating using attributes.
  repeated string dns_sans = 1;
}

// WorkloadIdentitySPIFFEJWT holds configuration specific to the issuance of
// a JWT SVID from a WorkloadIdentity.
message WorkloadIdentitySPIFFEJWT {
  // Currently empty - but will eventually permit the modification and insertion
  // of custom JWT claims.
}

message WorkloadIdentityTTL {
  // The maximum TTL that can be requested for credential using this
  // WorkloadIdentity.
  // If the requested TTL is greater than this value, the TTL will be capped at
  // this value.
  // If not set, a default value of 24 hours will be used.
  google.protobuf.Duration max = 1;
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
  // ttl holds configuration specific to the TTL of credentials issued from
  // this WorkloadIdentity resource.
  WorkloadIdentityTTL ttl = 5;
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

Exhaustive representation in YAML:

```yaml
kind: workload_identity
version: v1
metadata:
  name: gitlab
  labels:
    environment: production
spec:
  rules:
    allow:
    - conditions:
      - attribute: join.gitlab.user_email
        in:
        - admin-bob@example.com
        - admin-jane@example.com
    - conditions:
      - attribute: join.gitlab.namespace_path
        equals: my-org
      - attribute: join.gitlab.user_login
        not_equals: noah 
      - attribute: join.gitlab.environment
        not_matches: "^abc-.*$"
    - expression: join.gitlab.pipeline_id > 100
    deny:
    - conditions:
      - attribute: join.gitlab.ref_type
        equals: branch
      - attribute: join.gitlab.ref
        not_in: [main, master]
    - conditions:
      - attribute: join.gitlab.environment
        matches: "^xyz-.*$"
    - expression: join.gitlab.project_path == "my-org/my-project"
  spiffe:
    id: /gitlab/{{ join.gitlab.project_path }}/{{ join.gitlab.environment }}
    ttl:
      max: 12h
    x509:
      dns_sans:
      - {{ join.gitlab.environment }}.gitlab.example.com 
```

As per RFD 153, CRUD RPCs will be included for the WorkloadIdentity resource.
The ability to execute these RPCs will be governed by the standard verbs with a 
noun of `workload_identity`.

The proto specification of the RPCs is omitted for conciseness.

### IssueWorkloadIdentity RPC 

```protobuf
syntax = "proto3";

package teleport.workloadidentity.v1;

// WorkloadIdentityService provides the signing of workload identity documents.
service WorkloadIdentityService {
  // IssueWorkloadIdentity issues a workload identity credential from a 
  // specific named WorkloadIdentity resource.
  rpc IssueWorkloadIdentity(IssueWorkloadIdentityRequest) returns (IssueWorkloadIdentityResponse) {}
  // IssueWorkloadIdentityWithLabels issues workload identity credentials from
  // WorkloadIdentity resources that match the provided label selectors.
  rpc IssueWorkloadIdentityWithLabels(IssueWorkloadIdentityWithLabelsRequest) returns (IssueWorkloadIdentityResponse) {}
}

// The request parameters specific to the issuance of a JWT SVID.
message JWTSVIDRequest {
  // The value that should be included in the JWT SVID as the `aud` claim.
  // Required.
  repeated string audiences = 1;
  // The requested TTL for the JWT SVID. This request may be modified by
  // the server according to its policies. It is the client's responsibility
  // to check the TTL of the returned workload identity credential.
  google.protobuf.Duration ttl = 2; 
}

// The request parameters specific to the issuance of an X509 SVID.
message X509SVIDRequest {
  // A PKIX, ASN.1 DER encoded public key that should be included in the x509
  // SVID.
  // Required.
  bytes public_key = 1;
  // The requested TTL for the JWT SVID. This request may be modified by
  // the server according to its policies. It is the client's responsibility
  // to check the TTL of the returned workload identity credential.
  google.protobuf.Duration ttl = 2; 
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
  // The name of the Workload Identity resource used to issue this credential.
  string workload_identity_name = 6;
  // The revision of the Workload Identity resource used to issue this
  // credential.
  string workload_identity_revision = 7;
}

message IssueWorkloadIdentityRequest {
  // The name of the Workload Identity resource to attempt issuance using.
  string name = 1;

  // The credential_type oneof selects the type of credential (e.g JWT vs X509)
  // to be issued and also provides any parameters specific to that type.
  oneof credential_type {
    // Parameters specific to the issuance of a JWT SVID.
    JWTSVIDRequest jwt_svid = 2;
    // Parameters specific to the issuance of an X509 SVID.
    X509SVIDRequest x509_svid = 3;
  }

  // The results of any workload attestation performed by `tbot`.
  WorkloadAttributes workload_attributes = 4;
}

// Response for IssueWorkloadIdentity 
message IssueWorkloadIdentityResponse {
  // The issued credential.
  WorkloadIdentityCredential credential = 1;
}

// An individual label selector to be used to filter WorkloadIdentity resources.
message LabelSelector {
  // The name of the label to match.
  string key = 1;
  // The values to accept within the label. If multiple are provided, then any
  // may match. 
  repeated string values = 2;
}

// Request for IssueWorkloadIdentityWithLabels 
message IssueWorkloadIdentityWithLabelsRequest {
  // The label matchers which should be used to select a subset of the
  // WorkloadIdentity resources to attempt issuance using.
  repeated LabelSelector labels = 1;

  // The credential_type oneof selects the type of credential (e.g JWT vs X509)
  // to be issued and also provides any parameters specific to that type.
  oneof credential_type {
    // Parameters specific to the issuance of a JWT SVID.
    JWTSVIDRequest jwt_svid = 2;
    // Parameters specific to the issuance of an X509 SVID.
    X509SVIDRequest x509_svid = 3;
  }

  // The results of any workload attestation performed by `tbot`.
  WorkloadAttributes workload_attributes = 4;
}

// Response for IssueWorkloadIdentityWithLabels 
message IssueWorkloadIdentityWithLabelsResponse {
  // The issued credentials.
  repeated WorkloadIdentityCredentials credential = 1;
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

If, after evaluation of labels and rules, more than 20 WorkloadIdentity
resources remain, the client should receive an error encouraging the user to
leverage labels so that the set of returned WorkloadIdentity falls below this
threshold. This limits the runaway resource consumption if there is a 
misconfiguration.

For the initial release, this limit should be configurable via an environment
variable, allowing this to be tuned without requiring a new release of Teleport
should we discover a use-case that legitimately requires the issuance of a 
larger number of identities.

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
Bots and WorkloadIdentities used for large-scale deployment. For example,
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
- CEL has a built-in "cost estimation" mechanism which could be used to prevent
  the configuration of overly computationally expensive expressions.

Negatives:

- Inconsistency with other functionality in Teleport may be confusing for users.
- We already have internal expertise about the functionality of the Teleport
  predicate language and understand the existing limitations of it, CEL
  introduces a new set of limitations and challenges, many of which are unknown
  to us. That being said, CEL is more mature and battle-tested, it's unlikely
  we will encounter problems which are not already well-documented.

At this time, it is felt that consistency with the rest of the product is a 
higher priority. We can revisit the introduction of CEL at a later date.

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
  - The name and the revision to allow the specific version to be identified
    if this has been modified since.

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
workload attestation. This limits a bad actors ability to issue credentials
to a subset of the workload identifier namespace. Authorization rules that
process workload identities should also abide this.

We should ensure that this risk and the best practices are explained in the
product documentation, and, provide a set of example WorkloadIdentity resources 
for common use-cases.

To simplify responding to this kind of breach, the audit log must include
sufficient information to trace back issued workload identities to the identity
that requested them.
