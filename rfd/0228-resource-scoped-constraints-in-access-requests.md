---
authors: Maxim Dietz (maxim.dietz@goteleport.com)
state: draft
---

# RFD 0228 - Resource-Scoped Constraints in Access Requests

## Required Approvers

- Engineering: (@r0mant && @smallinsky)

## Related

- [#58307](https://github.com/gravitational/teleport/issues/58307): Privilege Escalation - Improve resources access request UX
- [#59486](https://github.com/gravitational/teleport/issues/59486): Request Access to Individual DBs
- [RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md):
  Component Feature Advertisement (blocker for Web UI to know which resources support constraints)
- https://github.com/gravitational/customer-sensitive-requests/issues/468

## What

This RFD proposes an extensible way to bind constraints to requested resources in resource-based Access Requests and
have Teleport:

1. Resolve a minimal role set that satisfies those per-resource constraints, and
2. Scope authorization to the requested `(resource, constraints)` pairs.

Initial focus: AWS Console & SSH nodes. The shape is generic and able to support database principals, Kubernetes
users/groups, Windows Desktop logins, Azure/GCP identities, and other future resource-specific constraints.

## Why

Users frequently already have _some_ access to a resource, but need to temporarily escalate for a task (e.g., switch to
an admin AWS role, use a higher-privileged DB role, or log in as a different SSH user). Today, making that escalation
via Access Requests can be opaque: granted vs. requestable options aren't presented together per resource, and roles
granting multiple principals aren't constrained to the requested principals on assumption. Presenting granted and
requestable options side-by-side and scoping authorization to the requested constraints will make escalation more
intuitive, explicit, and least-privilege by default.

## Background

### Terms

- **Principal (RFD-specific)**: Umbrella term for the resource-scoped identity a user selects when connecting (e.g., AWS
  role ARN, AWS IC permission set, database role/user, SSH login).
- **Constraint**: A per-resource selector that narrows what principals a user wants to, or is allowed to, use on that
  resource.
- **Granted**: Immediately usable given the requester's current login state (no request needed).
- **Requestable**: Not currently usable, but eligible to be approved via Access Request per policy.

### Types of Access Requests

- **Role-based**: The requester asks for one or more Teleport roles by name. If approved, those roles are added to the
  requester's cert for the session. These are out of scope; role-based requests don't identify a specific target
  resource, so per-resource constraints don't apply.
- **Resource-based**: The requester asks for access to one or more concrete resources by ID (cluster/kind/name;
  Kubernetes may also use `SubResourceName`). If approved, the cert lists the specific resources the requester may
  reach. This is the path we extend to carry per-resource constraints (e.g., "this ARN for this app", "these logins for
  this node").

### How resource-based requests work today

1. **Creation (Web UI/Proxy)**
   The Web UI sends a request with `requested_resource_ids` (each a `ResourceID`), optionally with a reason and other
   metadata. If the request doesn't explicitly include roles, it's treated as a resource-based request.
2. **Validation & resolution (Auth server)**
   Auth first builds a new `RequestValidator` with role expansion enabled. If no roles were provided, the validator
   computes a minimal set of `search_as_roles` that can access all requested resources (sometimes guided by an
   optional "SSH login hint"). The validator then deduplicates resources, enforces size limits, builds
   thresholds/annotations/suggested reviewers, and computes the session/request TTLs.
3. **Approval & issuance (Auth server)**
   On approval/assumption, Auth issues a new TLS identity that includes:
   - the resolved Teleport roles (in `Groups`), and
   - the approved `AllowedResourceIDs` (slash-delimited string paths that name the specific resources).

   The identity also carries allowed principals derived from the roles (e.g., `AWSRoleARNs`, `DatabaseUsers`,
   `KubernetesUsers/Groups`). However, these are _not_ tied to individual resources; they are global to the identity.

4. **Authorization (AccessChecker)**
   When the requester launches with a chosen principal, Teleport enforces in two stages:
   1. **Resource gate**: the target must appear in `AllowedResourceIDs` and pass RBAC label/condition checks.
   2. **Principal gate**: the chosen ARN / permission set / SSH login / DB user must be permitted by the identity's
      allowed principals.

### Current limitations

- **No per-resource constraint binding**: Certificates enumerate _resources_ (via `AllowedResourceIDs`) but do not tie
  resource-specific constraints (e.g., "these ARNs for this app") to those IDs. Allowed principals like `AWSRoleARNs`
  are global to the identity.
- **Resolution isn't persisted/enforced**: Hints (e.g., SSH login hints) can influence which roles are selected at
  validation time, but they aren't persisted in the identity and aren't enforced at authorization time. This means if a
  resolved role grants multiple principals, the requester gets them all once the resource is reachable, so the final
  access can be "over-granted" and exceed the requester's intent.

### Applicable resource kinds

The following resource kinds have user-selectable principals that can be scoped via constraints:

| Resource Kind       | Role Field(s)                                     | Principals                     |
| ------------------- | ------------------------------------------------- | ------------------------------ |
| AWS Console App     | `AWSRoleARNs`                                     | AWS IAM role ARNs              |
| SSH Node            | `Logins`                                          | SSH login usernames            |
| Windows Desktop     | `WindowsDesktopLogins`                            | Windows RDP logins             |
| Database            | `DatabaseUsers`, `DatabaseNames`, `DatabaseRoles` | DB users, schemas, roles       |
| Kubernetes Cluster  | `KubeUsers`, `KubeGroups`                         | K8s RBAC users/groups          |
| Azure App           | `AzureIdentities`                                 | Azure managed identities       |
| GCP App             | `GCPServiceAccounts`                              | GCP service accounts           |
| AWS Identity Center | `AccountAssignments`                              | Permission set + account pairs |

## UX

**Goal**: Let users pick constraints per-resource, see what's **granted** vs. **requestable**, and either launch
immediately or create a scoped Access Request. In the Web UI, this is a unified dropdown where the "connect"/"request"
button currently sits on resource cards. In `tsh`, constraints are specified inline on `--resource` (or as JSON) when
creating a request, and granted vs. requestable principals are surfaced by `tsh request search` and `tsh request
preview`.

**User stories**:

- **AWS Console (role elevation)**:
  Alice, a platform engineer with a standing read-only AWS role, needs elevated access to respond to an incident. In
  Teleport's Web UI, she searches for the AWS Console app and clicks its connect dropdown. There, she sees both her
  read-only role (available right away) and the other roles she is able to request. She selects the 'admin' ARN, submits
  the request, and after approval, is able to assume the request and launch the console with that elevated ARN.
- **AWS IAM IC (permission-set elevation)**:
  Ben operates in AWS IAM IC with a default `contributor` permission set but occasionally needs `BillingAdmin`. In
  Teleport's Web UI, he clicks the AWS IAM IC app's connect dropdown, selects `BillingAdmin` under the **requestable**
  section, submits the request, and once approved uses only that permission set without receiving extra IC permissions.
- **SSH node (login elevation)**:
  Diego can SSH to a service host as `deploy` but needs a temporary `web` login to perform maintenance. In Teleport's
  Web UI, he clicks the connect dropdown for the service node, and sees `deploy` under his **granted** logins, and `web`
  under the **requestable** section. He selects `web`, submits the request, and after approval, connects with just
  `web`.
- **SSH node via tsh**:
  Diego can also accomplish this via the CLI. He runs `tsh request search --kind node` to find requestable nodes, then
  `tsh request preview /cluster/node/web-1` to see which logins are granted vs. requestable. He
  then runs `tsh request create --resource '/cluster/node/web-1|logins=admin' --reason "maintenance"` to
  create the constrained request.
- **Database (role/user)**:
  Chandra has read-only access to a Postgres DB as `report_reader` and needs `migration_admin` for a one-off schema
  change. In Teleport's Web UI, she clicks the connect dropdown for the Postgres app, selects `migration_admin` under
  the **requestable** section, submits the request, and after approval, connects with only the `migration_admin` DB role
  for the session.

**Reviewer experience**: Reviewers see both **(a)** the requested constraints and **(b)** the resolved roles computed to
satisfy them.

## Design Overview

### ResourceID & Constraints

We add a `ResourceConstraints` message that carries domain-specific constraint details via a `oneof`, along with a
`ResourceAccessID` message that pairs a `ResourceID` with optional `ResourceConstraints`. A new
`RequestedResourceAccessIDs` field on `AccessRequestSpecV3` carries these when constraints are present.

> [!NOTE]
> The implementation uses `ResourceAccessID` and `ResourceConstraints` (not the `ConstrainedResourceID` /
> `ResourceConstraintDomain` names from the original draft). The `oneof` was chosen over a domain enum to use proto's
> type safety, avoiding a separate step to confirm the domain matches the detail type.

```protobuf
// ResourceConstraints is a domain-specific payload that narrows what principals
// or options are allowed on the associated ResourceID. Exactly one detail is set.
message ResourceConstraints {
  // version is the constraint format version; supported values are: "v1".
  string version = 1;

  oneof details {
    // aws_console scopes an AWS Console app to a subset of role ARNs.
    AWSConsoleResourceConstraints aws_console = 10;
    // ssh scopes an SSH node to a subset of logins.
    SSHResourceConstraints ssh = 11;
  }
}

message AWSConsoleResourceConstraints {
  repeated string role_arns = 1;
}

message SSHResourceConstraints {
  repeated string logins = 1;
}

// ResourceAccessID represents a ResourceID in an Access Request context,
// where additional information such as ResourceConstraints may be provided.
message ResourceAccessID {
  ResourceID id = 1;
  ResourceConstraints constraints = 2;
}

message ResourceAccessIDList {
  repeated ResourceAccessID resources = 1;
}
```

On `AccessRequestSpecV3`, a new field carries constrained resources:

```protobuf
message AccessRequestSpecV3 {
  // ...existing fields...

  // When present, RequestedResourceAccessIDs should be treated as authoritative
  // (ResourceIDs can be derived by mapping to ResourceAccessID.id).
  repeated ResourceAccessID RequestedResourceAccessIDs = 26;
}
```

### Serialization & Parsing

#### Encoding format

`ResourceAccessID`s with constraints are carried in the new `AllowedResourceAccessIDs` cert extension as JSON-encoded
text, marshaled with Go's standard `encoding/json`. The `ResourceConstraints` payload is the one exception: a protobuf
`oneof` does not round-trip through `encoding/json`, so `ResourceConstraints` defines custom `MarshalJSON`/`UnmarshalJSON`
methods backed by `jsonpb` (`OrigName: true`). These flatten the active variant to its proto field name. A serialized
entry is therefore:

```json
{
  "id": { "cluster": "main", "kind": "node", "name": "web-1" },
  "constraints": { "version": "v1", "ssh": { "logins": ["root", "admin"] } }
}
```

> [!NOTE]
> The original design called for deterministic proto3 binary encoding (`proto.MarshalOptions{Deterministic:true}`), but
> Go's `x509` package rejects certificates containing non-string ASN.1 types in name attributes; binary data (e.g.,
> OCTET STRING) causes encoding and parsing to fail with `"x509: invalid RDNSequence: invalid attribute value"`.
> See [golang/go#48371](https://github.com/golang/go/issues/48371).

#### Unconstrained resources

If a requested resource has no constraints, we serialize it as the slash-delimited `ResourceID` string under
`tlsca.Identity.AllowedResourceIDs`.

#### All-constrained case

If all requested resources are constrained, `AllowedResourceIDs` would otherwise be empty, which an older Auth/Proxy
(ignoring the new extension) reads through `AccessInfo` as "no resource-specific restrictions". To keep those components
fail-closed, a single sentinel `ResourceID` (`CreateSentinelResourceID()`, serialized as
`/__SENTINEL__/node/__SENTINEL__`) is added to `AllowedResourceIDs`. It matches no real resource, so an older component
denies access rather than treating the identity as unrestricted.

#### Parsing

1. Read `ResourceAccessIDList` JSON from the new extension and unmarshal. Unknown fields are ignored for forward
   compatibility.
2. If any entry is malformed or its constraint variant is unknown, it is omitted to avoid potential over-granting.
3. Parse legacy `tlsca.Identity.AllowedResourceIDs` as today; for consistency in code paths, "upgrade" each parsed
   `ResourceID` to a `ResourceAccessID` with nil `Constraints`.
4. Presence of the sentinel in `AllowedResourceIDs` has no effect for new agents; it is only consumed by older agents
   that ignore the new extension.

#### Limits & Safety

- **Size limits**:
  Enforce existing size limits for the encoded payload per cert. This helps keep cert sizes reasonable and avoid
  hard-to-debug transport issues we've hit in the past, namely, default gRPC message size limits.
  JSON encoding is slightly larger than binary proto, but well within this budget for typical request sizes.

  If exceeded, fail validation with a clear error message and guidance to reduce/split the request, similar to existing
  UX for long-term resource requests unable to be satisfied by a single Access List.

- **Safety**:
  - Deserialization errors from a `ResourceAccessID` cause request validation to fail.
  - Older binaries without knowledge of the new cert extension will ignore it, and only see the non-constrained
    `ResourceID`s in `AllowedResourceIDs`, preventing accidental over-granting.

### RBAC Semantics with Constraints

#### Role Definition

Greedy 'deny' behavior is preserved, as `ResourceConstraints` are purely additive. If a role in a user's RoleSet has
Deny rules blocking certain constraints (e.g., `role.spec.deny.logins`), those constraints are not presented as
requestable, even if another role in the RoleSet would allow requesting or assuming them.

#### Validation & Resolution

Resource-based Access Requests including constraints are validated using constraint-aware role matchers derived from
each `ResourceAccessID.Constraints` (e.g., ARN/SSH login matchers). `RequestValidator` extends the existing
`applicableSearchAsRoles`/`roleAllowsResource` flow by passing these matchers, so a role only qualifies if it allows the
requested resource _and_ the specified constraints. Resources are still pre-filtered by current access and requested
`ResourceAccessID`s are still deduplicated and size-capped, as they are currently. The resulting role list is the set of
requestable/search-as roles that satisfies all `(resource, constraints)` pairs (when multiple roles qualify, behavior
matches current semantics). If no roles qualify, existing failure behavior and errors apply.

#### Authorization

On approval/assumption, certs carry the exact `(resource, constraints)` information via
`tlsca.Identity.AllowedResourceAccessIDs` (in addition to the existing `AllowedResourceIDs` for backwards
compatibility). `AccessChecker` enforces those constraints via `WithConstraints`, a matcher transform that wraps
principal-bearing matchers (e.g., `loginMatcher`, `AWSRoleARNMatcher`) to additionally check that the principal is in
the constraint's allowed set. If a role grants multiple principals on a resource, only the requested principals are
allowed.

### Web/API

- The granted vs. requestable split is computed in Auth's `ListUnifiedResources`, not per-client in the proxy web
  handler. Auth already evaluates the caller's access against each listed resource both with their current roles
  (granted) and with their search-as-roles expansion (the requestable union), so the response carries both sets. Web,
  Connect, and `tsh` render that response rather than each recomputing the split.
- The split is carried on the listing response per principal kind. The granted set is a new field alongside the existing
  flat principal list, so a client that reads only the old field still works, while a constraint-aware client
  reconstructs the granted/requestable sets (requestable is the union minus the granted set). For each principal the UI
  renders a single dropdown with "Granted" and "Requestable" sections.
- Principals are populated for every constraint-applicable kind so the split is uniform across resource types (the same
  enrichment applied to nodes, desktops, and apps covers databases, Kubernetes, and the cloud apps).
- Feature
  advertisement ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md))
  gates constraint UI per resource. Proxy intersects `ComponentFeatures` from Auth, Proxy, and the resource's agent(s)
  and exposes a `supportedFeatureIds` field on each resource. Web UI only renders constraint dropdowns when
  `RESOURCE_CONSTRAINTS_V1` is present.

### TSH & TCTL

[RFD 0000's CLI UX guidance](https://github.com/gravitational/teleport/blob/master/rfd/0000-rfds.md#cli-ux) treats the
CLI as a primary interface for agents as well as humans. The design below keeps the human path short, makes
`--format json` the canonical path for automation, and avoids order-dependent flags and interactive prompts.

#### Specifying constraints on `tsh request create`

`tsh request create` accepts repeatable `--resource` flags. Each value is a slash-delimited `ResourceID` string (e.g.
`/cluster/node/web-1`); we extend it to accept three forms, where constraints attach to a single resource so flag order
does not matter:

1. A plain `ResourceID` (unchanged, unconstrained):

   ```
   --resource /cluster/node/web-1
   ```

2. Inline constraints, for convenience, appended to the resource string after a `|` as `key=v1,v2` pairs joined by `|`:

   ```
   tsh request create \
     --resource '/cluster/node/web-1|logins=root,admin' \
     --resource '/cluster/db/postgres|db_users=admin|db_names=production' \
     --reason "incident response"
   ```

3. A JSON `ResourceAccessID`, the canonical form for automation and agents and the fallback for any value the inline
   form cannot express. This is the shape serialized into the request and cert, and reuses the existing
   `ResourceAccessID` parsing and validation:

   ```
   --resource '{"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["root","admin"]}}}'
   ```

   (For larger or generated requests, the same JSON list may optionally be supplied via `--resource-file <path>` or
   stdin, avoiding shell-quoting many blobs.)

The CLI selects the form per value: a leading `{` is parsed as JSON; otherwise the value is a `ResourceID` string, split
into id + constraints if it carries an inline suffix (below).

The inline form is split from its constraints with an anchored-key rule. A `ResourceID` string treats its name as the
opaque trailing segment, and resource names are not restricted to a fixed character set: databases and Kubernetes
clusters reject `|` in their names,
but nodes and apps do not. So rather than assume `|` is absent from names, the parser splits on the first occurrence of
`|<key>=`, where `<key>` is a recognized constraint key from the table below. Everything before is parsed as a plain
`ResourceID`; everything after is the constraint suffix. A false split would require a resource name containing the
literal substring `|logins=` (or another key); the JSON form covers that case if it ever arises.

| Key                    | Constraint Kind      | Example                                                |
| ---------------------- | -------------------- | ------------------------------------------------------ |
| `logins`               | SSH logins           | `logins=root,admin`                                    |
| `role_arns`            | AWS Console ARNs     | `role_arns=arn:aws:iam::123:role/Admin`                |
| `db_users`             | Database users       | `db_users=postgres,admin`                              |
| `db_names`             | Database names       | `db_names=production`                                  |
| `db_roles`             | Database roles       | `db_roles=admin`                                       |
| `kube_users`           | Kubernetes users     | `kube_users=system:admin`                              |
| `kube_groups`          | Kubernetes groups    | `kube_groups=system:masters`                           |
| `azure_identities`     | Azure identities     | `azure_identities=/sub/.../identity`                   |
| `gcp_service_accounts` | GCP service accounts | `gcp_service_accounts=sa@proj.iam.gserviceaccount.com` |

Each key maps to a `ResourceConstraints` variant (`ssh` and `aws_console` first; the rest are added per kind under the
same `oneof`). The CLI rejects a key whose variant or agent support does not yet exist.

No supported principal value contains `=`, `,`, or `|` (ARNs and `system:`-prefixed names use `:` and `/`, never these
three), so the `key=v1,v2|key2=...` grammar is unambiguous on the value side; the only collision risk is the
resource-name side, handled above.

The CLI validates the following before sending the request:

- The key must be a recognized constraint kind.
- The key must apply to the resource kind of its `ResourceID` (e.g. `logins` only for `node`, `db_users` only for `db`);
  the kind is read from the `ResourceID`.
- Duplicate keys on one resource are merged.
- Empty values are rejected.

When constraints are present (inline or JSON), the CLI builds a `ResourceAccessID` with the matching
`ResourceConstraints` variant; otherwise the resource is sent as an unconstrained `ResourceAccessID` (current behavior).

#### Discovering principals via `tsh request search`

`tsh request search --kind <kind>` lists requestable resources. To choose constraints, a user (or agent) needs to know
which principals are granted vs. requestable per resource. As described in [Web/API](#webapi), Auth computes this split
in `ListUnifiedResources` and returns it directly, so `tsh`, Teleport Connect, and the Web UI render the same response
rather than each recomputing it.

Principals are surfaced at three levels of detail:

The default table stays compact. Rather than per-principal columns, which do not scale across kinds or to
dozens of ARNs, each resource gets a single summary cell, `<n> granted, <m> requestable`, with either side omitted when
zero:

```
$ tsh request search --kind node

Name   Hostname    Labels     Access
----   ---------   --------   ------------------------
web-1  web-1.dc1   env=prod   1 granted, 2 requestable
db-2   db-2.dc1    env=prod   3 granted

hint: use 'tsh request preview <resource-id>' to view granted & requestable principals
hint: to request access, run 'tsh request create --resource <resource-id> --reason <reason>'
```

`tsh request preview <resource-id>` is the human detail view: a per-resource listing of every principal grouped granted
vs. requestable, adapting to the resource kind. It takes a `ResourceID`, so it covers all kinds, and is separate from
`tsh request show`, which takes an access _request_ ID:

```
$ tsh request preview /cluster/node/web-1

Resource:  /cluster/node/web-1
Name:      web-1
Hostname:  web-1.dc1
Labels:    env=prod

Logins:
  deploy   granted
  root     requestable
  admin    requestable

hint: tsh request create --resource '/cluster/node/web-1|logins=root,admin' --reason "..."
```

Resources with multiple principal types (databases, for example) list each type under its own heading (Database Users,
Database Names), each split into granted and requestable.

For automation and agents, `tsh request search --format json` and `tsh request preview --format json` emit the complete
granted and requestable sets per principal kind, untruncated. This is the agent path: one call returns everything needed
to construct a constrained `tsh request create`, with no second round-trip and no interactive step.

#### Agentic use

The flows above follow [RFD 0000's CLI UX principles](https://github.com/gravitational/teleport/blob/master/rfd/0000-rfds.md#cli-ux):

- Structured output is the interface. `search` and `preview` support `--format json`, returning granted/requestable
  principals per resource so an agent enumerates options in one call, and `tsh request create --format json` returns the
  created request (id, state, resolved roles) so the agent can confirm the outcome.
- Flag order never matters and nothing blocks on a prompt, since constraints attach to a resource (inline or JSON). The
  JSON `--resource` form (and `--resource-file`) is the reliable input path for generated requests.
- The granted/requestable split is computed server-side and returned with the listing, so an agent does not chain
  `search` with a second principal-fetch call.

An end-to-end agent flow (discover, then request):

```
# 1. Enumerate requestable principals (one structured call)
tsh request search --kind node --format json

# 2. Create a constrained request (JSON resource form, structured result)
tsh request create \
  --resource '{"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["admin"]}}}' \
  --reason "automated remediation" --format json
```

#### Agent Skill

Per [RFD 0000's Agent Skills guidance](https://github.com/gravitational/teleport/blob/master/rfd/0000-rfds.md#agent-skills),
we will ship a reference Agent Skill covering the full flow: enumerate requestable principals, create a scoped request,
wait for approval, and assume. The skill documents the JSON `--resource` form and `--format json` output so an agent
need not rediscover the CLI grammar, and passes through validation errors so the agent can correct its input.

#### Display in `tsh request show`, `tsh request ls`, and `tctl`

`tsh request show` renders the constraints on each requested resource in the `resource_id (key=value,value)` format, for
every variant.

`tsh request ls` omits constraints from its compact table, which is fine for the overview, but `tsh request ls --format
json` includes full constraint information.

`tctl requests get` and `tctl requests ls` follow the same pattern: compact text by default, complete `ResourceAccessID`s
(constraints included) under `--format json`, so constrained requests are fully inspectable from automation.

## Compatibility

- Providing `ResourceAccessID`s with constraints is optional; existing clients and requests without constraints continue
  to work unchanged.
- `tsh request create --resource` accepts plain slash-delimited `ResourceID` strings unchanged; the inline (`|`) and
  JSON forms are additive and selected by the value's shape, so existing scripts keep working.
- RBAC semantics are additive. Constraint matchers are considered only when constraints are present on a
  `ResourceAccessID`.
- All cert paths that re-issue or re-encode an identity (web, app, and database sessions, and leaf-cluster routing)
  propagate the constraint extension, so constraints survive session and routing cert exchanges rather than being
  dropped at a hop.

### Mixed-version behavior

Enforcement is fail-closed across version skew. A component that does not understand the `AllowedResourceAccessIDs`
extension does not see the constrained resources in the legacy `AllowedResourceIDs` (only the sentinel from
[Serialization & Parsing](#serialization--parsing)), so it denies them rather than granting them unconstrained.

This is a deliberate security choice. Duplicating constrained resources into the legacy field so older components could
still serve them would let those components grant access without applying the constraint, making the feature unreliable
for least-privilege (see [Alternatives Considered](#alternatives-considered)).

- When a new client sends a constrained request to an old Auth, the unknown `RequestedResourceAccessIDs` field is
  dropped and the request degrades to unconstrained (the legacy `RequestedResourceIDs` is still honored). To keep a user
  from thinking they scoped a request that was not, the client gates on constraint support (feature advertisement / Auth
  version) and refuses to send constraints to an Auth that cannot enforce them.
- An old client against a new Auth works unchanged: it neither sends nor displays constraints.
- When a new Auth issues a constrained cert to an old Proxy or agent, the agent ignores the extension and sees only
  unconstrained `ResourceID`s, so it cannot over-grant. Constraint UI is withheld for that resource via feature
  advertisement, so the flow is not offered where it would dead-end in denial.
- When a new client or Proxy lists against an old Auth, `ListUnifiedResources` returns only the principal union, not the
  granted/requestable split, so clients fall back to the union (principals shown without the distinction) rather than
  failing.
- Across trusted clusters (root/leaf), a request may target leaf resources whose owning cluster runs a different
  version. Constraints are only offered when every component on the access path supports them, so feature advertisement
  ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md))
  intersects `ComponentFeatures` across the root Auth/Proxy and the leaf's Auth and agent(s), not just local components.
  A constrained cert reaching an older leaf is enforced by denial (above); the gating keeps the flow from being offered
  for leaf resources whose components are too old.
- Web UI adopts the per-principal "granted vs requestable" grouping incrementally by resource kind, gated by
  `supportedFeatureIds` from feature advertisement. The new per-principal fields are added alongside the existing flat
  ones, so older Web UIs keep working with newer proxies.

## Rollout & Gating

This feature is gated by component feature
advertisement ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md))
to ensure Web UI only surfaces constraint flows for resources whose Agent(s) support them.

This gating is strictly for UX enablement; server-side validation and enforcement remain fail-closed. We consider RFD
0230 a prerequisite for shipping constraint UI to avoid presenting actions that older components cannot fulfill.

For `tsh`/`tctl`, constraint support does not require feature advertisement gating. CLI users explicitly opt in by
providing inline or JSON constraints on `--resource`, and server-side validation will reject constraints that are not
supported. A clear error message should be returned if constraints are provided for a resource whose agent does not
support them.

## Scale

- Producing the split evaluates the caller's access against each listed resource twice, for the granted set and for the
  requestable union. This is the access-check work any granted-vs-requestable computation requires; keeping it in Auth
  avoids the Proxy and Auth each running a separate pass. It runs only when the listing requests requestable principals,
  and listing stays paginated.
- Constraint payloads are bounded by the existing cert and request size limits (see
  [Serialization & Parsing](#serialization--parsing)). JSON is larger than binary proto but within budget for typical
  requests.
- `tsh request preview` resolves a single `ResourceID` and adds no list-time cost.

## Audit Events

No new event types are required; constraints extend existing access-request events.

- `AccessRequestCreate` carries `RequestedResourceAccessIDs` (`[]ResourceAccessID`, constraints included) alongside the
  legacy `RequestedResourceIDs`, so the audit log records the requested `(resource, constraints)` pairs.
- The certificate-issuance event's embedded `Identity` records the enforced `AllowedResourceAccessIDs`, so the issued
  grant is auditable, not only the request.
- `AccessRequestResourceSearch` records `tsh request search`; it carries no constraint data, since the search does not
  select constraints.
- Events serialize constraints with a variant per kind plus an unknown-variant fallback, so future constraint kinds are
  preserved in the log rather than dropped.

## Observability

- The `*ResourceAccessIDs` fields above are enough to trace a constrained request end to end (create, approve, issued
  cert) from the audit log.
- A constraint that fails validation returns a clear error naming the offending key and resource; this surfaces in
  `tsh` / `tctl` output and Auth logs.
- No new Prometheus metrics for the initial release. Constrained-request volume can be derived from the audit events if
  needed.

## Test Plan

The following items are added to the [Test Plan](../.github/ISSUE_TEMPLATE/testplan.md), under its Access Requests
section:

- Creating a constrained request via each `--resource` form (plain, inline `|`, JSON), verifying the resolved roles and
  issued cert match the requested constraints.
- `tsh request preview` and `tsh request search` (text and `--format json`) showing correct granted vs. requestable
  principals per kind.
- Enforcement: assuming a constrained request grants only the requested principals, even when the resolved role grants
  more.
- Mixed-version behavior from [Compatibility](#compatibility): a constrained request is denied (not over-granted) on an
  older agent, and the CLI refuses to send constraints to an Auth that cannot enforce them.

These are end-to-end checks; the inline anchored-key parser and JSON `--resource` parsing (including resource names
containing `|` and malformed or unknown-variant entries) are covered by unit tests.

## Non-goals and Future Work

### Non-goals

- Role-based Access Requests are permanently out of scope, not deferred: constraints are per-resource, and a role-based
  request names no target resource, so there is nothing to scope.

### Out of scope for this draft

- Enforcement for kinds beyond AWS Console and SSH. The `ResourceConstraints` oneof currently contains `aws_console` and
  `ssh`; the other keys in the CLI table are part of the surface but are not yet enforceable.
- AWS Identity Center. IC access is already modeled as synthetic account-assignment resources that scope to a
  permission-set + account pair, so it reaches a similar result by another route. Bringing it under this framework needs
  deeper refactoring of that model, not a new constraint variant.
- Interactive CLI prompting and review-time constraint editing. The CLI takes constraints up front (inline or JSON), and
  a reviewer approves or denies the requested `(resource, constraints)` pairs as submitted.

### Planned

- Resource-kind coverage. Database (users/names/roles), Kubernetes (users/groups), Windows Desktop logins, Azure
  identities, and GCP service accounts, each adding a `ResourceConstraints` variant under the existing oneof with
  matching agent-side enforcement. AWS Identity Center follows once the account-assignment model is refactored.
- CLI and agent support. Broader `--format json` coverage, the reference Agent Skill described above, and possibly an
  interactive selection flow for humans.
- Teleport Connect. The same granted vs. requestable dropdown as the Web UI, reading the same listing response as Web
  and `tsh`.

## Alternatives Considered

- Duplicating constrained resources into the legacy `AllowedResourceIDs` field, so older components keep serving them,
  was decided against on security grounds. An older component would reach the resource without applying the constraint,
  so the requested principal scoping would silently not hold on that path, which makes the feature unsafe to rely on for
  least-privilege. The fail-closed alternative (constrained resources are absent from the legacy field, so older
  components deny them; see [Mixed-version behavior](#mixed-version-behavior)) was chosen instead.
- A dedicated principal-discovery RPC (e.g. `GetResourcePrincipals`) instead of extending `ListUnifiedResources` was
  rejected: it repeats the access-check work the resource listing already performs, adds a second round-trip, and leaves
  Web, Connect, and `tsh` on separate paths. Extending the existing listing keeps one path and one computation.
- Order-dependent `--constraint` flags, where a constraint applies to the preceding `--resource`, were rejected: flag
  order changing meaning is a CLI anti-pattern and is hard for agents to produce reliably. Constraints attach to the
  resource value instead (inline `|` or JSON).
- JSON as the only CLI constraint form was rejected: reliable for agents but tedious for humans. The inline `|` form
  covers the common case, with JSON as the canonical form for automation and the fallback for values the inline form
  cannot express.
- A domain enum instead of a proto oneof for `ResourceConstraints` was rejected: the oneof gives compile-time type
  safety and removes a separate step to match a domain to its detail payload (see
  [ResourceID & Constraints](#resourceid--constraints)).
- Deterministic binary proto encoding in the cert extension was rejected: Go's `x509` rejects non-string ASN.1 in name
  attributes, so binary fails to encode and parse. The extension carries JSON text instead (see
  [Serialization & Parsing](#serialization--parsing)).
