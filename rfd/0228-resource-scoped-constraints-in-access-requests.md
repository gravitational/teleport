| Authors                                  | State |
|------------------------------------------|-------|
| Maxim Dietz (maxim.dietz@goteleport.com) | draft |

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

Users frequently already have *some* access to a resource, but need to temporarily escalate for a task (e.g., switch to
an admin AWS role, use a higher-privileged DB role, or log in as a different SSH user). Today, making that escalation
via Access Requests can be opaque: granted vs. requestable options aren't presented together per resource, and roles
granting multiple principals aren't constrained to the requested principals on assumption. Presenting granted and
requestable options side-by-side and scoping authorization to the requested constraints will make escalation more
intuitive, explicit, and least-privilege by default.

## Background

### Terms

- **Principal (RFD-specific)**: Umbrella term for the resource-scoped identity a user selects when connecting (e.g., AWS
  role ARN, AWS IC permission set, database role/user, SSH login).
- **Constraint**: A per-resource selector that narrows what principals user wants to–or is allowed to–use on that
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
   `KubernetesUsers/Groups`). However, these are *not* tied to individual resources; they are global to the identity.
4. **Authorization (AccessChecker)**
   When the requester launches with a chosen principal, Teleport enforces in two stages:
    1. **Resource gate**: the target must appear in `AllowedResourceIDs` and pass RBAC label/condition checks.
    2. **Principal gate**: the chosen ARN / permission set / SSH login / DB user must be permitted by the identity's
       allowed principals.

### Current limitations

- **No per-resource constraint binding**: Certificates enumerate *resources* (via `AllowedResourceIDs`) but do not tie
  resource-specific constraints (e.g., "these ARNs for this app") to those IDs. Allowed principals like `AWSRoleARNs`
  are global to the identity.
- **Resolution isn't persisted/enforced**: Hints (e.g., SSH login hints) can influence which roles are selected at
  validation time, but they aren't persisted in the identity and aren't enforced at authorization time. This means if a
  resolved role grants multiple principals, the requester gets them all once the resource is reachable, so the final
  access can be "over-granted" and exceed the requester's intent.

### Applicable resource kinds

The following resource kinds have user-selectable principals that can be scoped via constraints:

| Resource Kind       | Role Field(s)                                     | Principals                     |
|---------------------|---------------------------------------------------|--------------------------------|
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
button currently sits on resource cards. In `tsh`, this is via `--constraint` flags on `tsh request create` and
principal columns on `tsh request search`.

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
  `tsh request search --inspect /cluster/node/web-1` to see which logins are granted vs. requestable. He
  then runs `tsh request create --resource /cluster/node/web-1 --constraint logins=admin --reason "maintenance"` to
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
`ResourceConstraintDomain` names from the original draft). The `oneof` approach was chosen over a domain enum to
> leverage proto's built-in type safety and avoid needing a separate validation step to ensure the domain matches the
> detail type.

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
text. Each `ResourceAccessID` is serialized using Go's standard `encoding/json` marshaling of the proto-generated types.

> [!NOTE]
> The original design called for deterministic proto3 binary encoding (`proto.MarshalOptions{Deterministic:true}`), but
> Go's `x509` package rejects certificates containing non-string ASN.1 types in name attributes; binary data (e.g.,
> OCTET STRING) causes encoding and parsing to fail with `"x509: invalid RDNSequence: invalid attribute value"`.
> See [golang/go#48371](https://github.com/golang/go/issues/48371).

#### Unconstrained resources

If a requested resource has no constraints, we serialize it as today's slash-delimited `ResourceID` string under
`tlsca.Identity.AllowedResourceIDs`.

#### All-constrained case

If all requested resources are constrained, we add a single sentinel value (e.g.,
"/placeholder/placeholder/placeholder") to `AllowedResourceIDs`, as `AccessChecker` in older Auths/Proxies (which will
ignore the new extension) treat an empty list of allowed resources provided via `AccessInfo` as meaning
"no resource-specific restrictions".

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
requested resource *and* the specified constraints. Resources are still pre-filtered by current access and requested
`ResourceAccessID`s are still deduplicated and size-capped, as they are currently. The resulting role list is the set of
requestable/search-as roles that satisfies all `(resource, constraints)` pairs (when multiple roles qualify, behavior
matches current semantics). If no roles qualify, existing failure behavior and errors apply.

#### Authorization

On approval/assumption, certs carry the exact `(resource, constraints)` information via
`tlsca.Identity.AllowedConstrainedResourceIDs` (in addition to the existing `AllowedResourceIDs` for backwards
compatibility). `AccessChecker` enforces those constraints via `WithConstraints`, a matcher transform that wraps
principal-bearing matchers (e.g., `loginMatcher`, `AWSRoleARNMatcher`) to additionally check that the principal is in
the constraint's allowed set. If a role grants multiple principals on a resource, only the requested principals are
allowed.

### Web/API

- Listing `UnifiedResources` surfaces per-principal status when "show requestable" is enabled. For each principal (ARN,
  SSH login, etc.), a `requiresRequest: true|false` marker is provided so the UI can render a single dropdown with "
  Granted" and "Requestable" sections.
- New fields are added alongside existing ones for backwards compatibility (e.g., `sshLoginDetails` alongside
  `sshLogins`). The Web UI uses the detailed field when present and falls back to the legacy field when talking to older
  Proxies.
- Feature
  advertisement ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md))
  gates constraint UI per resource. Proxy intersects `ComponentFeatures` from Auth, Proxy, and the resource's agent(s)
  and exposes a `supportedFeatureIds` field on each resource. Web UI only renders constraint dropdowns when
  `RESOURCE_CONSTRAINTS_V1` is present.

### TSH & TCTL

#### Specifying constraints on `tsh request create`

Today, `tsh request create` accepts `--resource` flags with ResourceID strings (e.g., `/cluster/node/web-1`). To support
constraints, we add an optional `--constraint` flag that applies to the immediately preceding `--resource`:

```
tsh request create \
  --resource /cluster/node/web-1 --constraint logins=root,admin \
  --resource /cluster/app/aws-console --constraint role_arns=arn:aws:iam::123:role/Admin \
  --reason "incident response"
```

The `--constraint` flag uses a `key=value` format where the key identifies the constraint kind and the value is a
comma-separated list of principals. The flag stacks; multiple `--constraint` flags can follow a single `--resource` to
specify compound constraints. For example, databases may need both a user and a database name:

```
tsh request create \
  --resource /cluster/db/postgres --constraint db_users=admin --constraint db_names=production \
  --reason "schema migration"
```

Each `--constraint` applies to the immediately preceding `--resource`. If a `--constraint` appears before any
`--resource`, the CLI returns an error.

| Key                    | Constraint Kind      | Example                                                |
|------------------------|----------------------|--------------------------------------------------------|
| `logins`               | SSH logins           | `logins=root,admin`                                    |
| `role_arns`            | AWS Console ARNs     | `role_arns=arn:aws:iam::123:role/Admin`                |
| `db_users`             | Database users       | `db_users=postgres,admin`                              |
| `db_names`             | Database names       | `db_names=production`                                  |
| `db_roles`             | Database roles       | `db_roles=admin`                                       |
| `kube_users`           | Kubernetes users     | `kube_users=system:admin`                              |
| `kube_groups`          | Kubernetes groups    | `kube_groups=system:masters`                           |
| `azure_identities`     | Azure identities     | `azure_identities=/sub/.../identity`                   |
| `gcp_service_accounts` | GCP service accounts | `gcp_service_accounts=sa@proj.iam.gserviceaccount.com` |

**Validation**: The CLI validates `--constraint` inputs before submitting the request:

- The `key` must be a recognized constraint kind from the table above.
- The `key` must be applicable to the resource kind of the preceding `--resource` (e.g., `logins` is only valid for
  `node` resources, `db_users` only for `db` resources). The CLI can determine the resource kind from the `ResourceID`
  string.
- Duplicate keys for the same resource are merged (e.g., two `--constraint logins=...` for the same `--resource`).
- Empty values are rejected.

When `--constraint` is provided, the CLI constructs a `ResourceAccessID` with the appropriate `ResourceConstraints`
variant populated. When `--constraint` is omitted for a `--resource`, the resource is added as an unconstrained
`ResourceAccessID` (i.e., current behavior).

#### Discovering principals via `tsh request search`

`tsh request search --kind <kind>` currently displays resources with their names, labels, and ResourceID strings. To
support constraints, it needs to also display which principals are granted vs. requestable for each resource.

This requires principal information that is currently only available via Auth (requires calling `AccessChecker` with
`AccessInfo` containing a `RoleSet` of the user's current roles, to determine which roles granting principals are
currently requestable). Two potential approaches here:

**Option A: New `GetResourcePrincipals` RPC**

Add a new Auth RPC that, given a list of ResourceIDs, returns the granted and requestable principals for each:

```protobuf
message GetResourcePrincipalsRequest {
  repeated ResourceID resource_ids = 1;
}

message ResourcePrincipals {
  ResourceID resource_id = 1;
  repeated Principal granted = 2;
  repeated Principal requestable = 3;
}

message Principal {
  string kind = 1;   // "login", "role_arn", "db_user", etc.
  string value = 2;
}

message GetResourcePrincipalsResponse {
  repeated ResourcePrincipals resources = 1;
}
```

This keeps the `tsh request search` flow two-step (list resources, then fetch principals) but provides a clean,
dedicated API. The RPC would reuse the same `AccessChecker` logic the web handler uses to compute granted vs.
requestable sets.

**Option B: Extend `ListResources` response**

Add per-resource principal metadata to the existing `ListResources` / `ListUnifiedResources` response, similar to how
the web handler already computes `requiresRequest` per principal when `IncludeRequestable` is set. This avoids a second
RPC call but couples principal enumeration to resource listing.

> [!NOTE]
> Both options require Auth to evaluate `AccessChecker` for the calling user's RoleSet against each resource, which is
> the same work the web handler already does. This choice is primarily about API surface area. Option A is more explicit
> and avoids bloating the general-purpose `ListResources` response; Option B avoids the extra round-trip.

#### Discovering principals via `tsh request search --inspect`

The `tsh request search` table view remains unchanged; it lists requestable resources with their names, labels, and
ResourceID strings. Adding per-constraint-type columns (e.g., "Logins (Granted)", "Logins (Requestable)") to the table
wouldn't scale well across resource kinds and would make the output unwieldy.

Instead, we add an `--inspect <resource-id>` flag that shows detailed principal information for a specific resource from
the search results:

```
$ tsh request search --inspect /cluster/node/web-1

Resource:    /cluster/node/web-1
Name:        web-1
Hostname:    web-1.dc1
Labels:      env=prod

Principals:
  Logins:
    deploy         granted
    root           requestable
    admin          requestable

hint: tsh request create --resource /cluster/node/web-1 --constraint logins=root,admin --reason "..."
```

For resource kinds with multiple principal types (e.g., databases), all applicable principal types are shown:

```
$ tsh request search --inspect /cluster/db/postgres

Resource:    /cluster/db/postgres
Name:        postgres
Labels:      env=prod

Principals:
  Database Users:
    report_reader    granted
    admin            requestable
  Database Names:
    analytics        granted
    production       requestable

hint: tsh request create --resource /cluster/db/postgres --constraint db_users=admin --constraint db_names=production --reason "..."
```

The `--format json` output of `tsh request search` (without `--inspect`) should also include principal information for
each resource, enabling scripted workflows.

The hint at the bottom of `tsh request search` (without `--inspect`) should mention the `--inspect` flag:

```
hint: use 'tsh request search --inspect <resource-id>' to view granted & requestable principals
```

#### Display in `tsh request show` and `tsh request ls`

`tsh request show` already displays constraints via `FormatResourceAccessID()`. This function should be extended to
handle all constraint variants (currently only `aws_console` is handled). The format `resource_id (key=value,value)` is
preserved.

`tsh request ls` currently discards constraints for brevity (via `RiskyExtractResourceIDs()`). This is acceptable for
the compact table view, but `tsh request ls --format json` should include full constraint information.

`tctl requests get` and `tctl requests ls` should follow the same pattern.

## Compatibility

- Providing `ResourceAccessID`s with constraints is optional; existing clients and requests without constraints continue
  to work unchanged.
- RBAC semantics are additive. Constraint matchers (`WithConstraints`) are considered only when `ResourceConstraints`
  are present on a `ResourceAccessID`.
- Web UI adopts the per-principal "granted vs requestable" grouping incrementally by resource kind, gated by
  `supportedFeatureIds` from component feature
  advertisement ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md)).
- New web handler fields (e.g., `sshLoginDetails`) are added alongside existing fields (e.g., `sshLogins`) so older Web
  UIs continue to work with newer proxies.
- Older Agents ignore the new cert extension and only see non-constrained `ResourceID`s (if any) in
  `AllowedResourceIDs`, preventing accidental over-granting.

## Rollout & Gating

This feature is gated by component feature
advertisement ([RFD 0230](https://github.com/gravitational/teleport/blob/master/rfd/0230-component-feature-advertisement.md))
to ensure Web UI only surfaces constraint flows for resources whose Agent(s) support therm.

This gating is strictly for UX enablement; server-side validation and enforcement remain fail-closed. We consider RFD
0230 a prerequisite for shipping constraint UI to avoid presenting actions that older components cannot fulfill.

For `tsh`/`tctl`, constraint support does not require feature advertisement gating. CLI users explicitly opt in by
providing `--constraint` flags, and server-side validation will reject constraints that are not supported. A clear error
message should be returned if constraints are provided for a resource whose agent does not support them.
