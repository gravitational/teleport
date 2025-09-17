
| Authors                                  | State  |
|------------------------------------------|--------|
| Maxim Dietz (maxim.dietz@goteleport.com) | draft  |

# RFD 0228 - Resource-Scoped Constraints in Access Requests

## Required Approvers
- Engineering: (@r0mant && @smallinsky)

## Related
- https://github.com/gravitational/teleport/issues/58307
- https://github.com/gravitational/teleport/issues/59486
- https://github.com/gravitational/customer-sensitive-requests/issues/468

## What
This RFD proposes an extensible way to bind constraints to requested resources in resource-based Access Requests and have Teleport:
1. Resolve a minimal role set that satisfies those per-resource constraints, and
2. Scope authorization to the requested `(resource, constraints)` pairs.

Initial focus: AWS Console & AWS IC. The shape is generic and able to support SSH logins, database roles, and other future resource-specific constraints.

## Why
Users frequently already have *some* access to a resource, but need to temporarily escalate for a task (e.g., switch to an admin AWS role, use a higher-privileged DB role, or log in as a different SSH user). Today, making that escalation via Access Requests can be opaque: granted vs. requestable options aren’t presented together per resource, and roles granting multiple principals aren’t constrained to the requested principals on assumption. Presenting granted and requestable options side-by-side and scoping authorization to the requested constraints will make escalation more intuitive, explicit, and least-privilege by default.

## Background
### Terms
- **Principal (RFD-specific)**: Umbrella term for the resource-scoped identity a user selects when connecting (e.g., AWS role ARN, AWS IC permission set, database role/user, SSH login).
- **Constraint**: A per-resource selector that narrows what principals user wants to–or is allowed to–use on that resource.
- **Granted**: Immediately usable given the requester’s current login state (no request needed).
- **Requestable**: Not currently usable, but eligible to be approved via Access Request per policy.

### Types of Access Requests
- **Role-based**: The requester asks for one or more Teleport roles by name. If approved, those roles are added to the requester's cert for the session. These are out of scope; role-based requests don't identify a specific target resource, so per-resource constraints don't apply.
- **Resource-based**: The requester asks for access to one or more concrete resources by ID (cluster/kind/name; Kubernetes may also use `SubResourceName`). If approved, the cert lists the specific resources the requester may reach. This is the path we extend to carry per-resource constraints (e.g., "this ARN for this app", "these logins for this node").

### How resource-based requests work today
1. **Creation (Web UI/Proxy)**  
   The Web UI sends a request with `requested_resource_ids` (each a `ResourceID`), optionally with a reason and other metadata. If the request doesn't explicitly include roles, it's treated as a resource-based request.
2. **Validation & resolution (Auth server)**  
   Auth first builds a new `RequestValidator` with role expansion enabled. If no roles were provided, the validator computes a minimal set of `search_as_roles` that can access all requested resources (sometimes guided by an optional "SSH login hint"). The validator then deduplicates resources, enforces size limits, builds thresholds/annotations/suggested reviewers, and computes the session/request TTLs.
3. **Approval & issuance (Auth server)**  
   On approval/assumption, Auth issues a new TLS identity that includes:
     - the resolved Teleport roles (in `Groups`), and
     - the approved `AllowedResourceIDs` (slash-delimited string paths that name the specific resources).

   The identity also carries allowed principals derived from the roles (e.g., `AWSRoleARNs`, `DatabaseUsers`, `KubernetesUsers/Groups`). However, these are *not* tied to individual resources; they are global to the identity.
4. **Authorization (AccessChecker)**  
   When the requester launches with a chosen principal, Teleport enforces in two stages:
     1. **Resource gate**: the target must appear in `AllowedResourceIDs` and pass RBAC label/condition checks.
     2. **Principal gate**: the chosen ARN / permission set / SSH login / DB user must be permitted by the identity's allowed principals.

### Current limitations
- **No per-resource constraint binding**: Certificates enumerate *resources* (via `AllowedResourceIDs`) but do not tie resource-specific constraints (e.g., "these ARNs for this app") to those IDs. Allowed principals like `AWSRoleARNs` are global to the identity.
- **Resolution isn't persisted/enforced**: Hints (e.g., SSH login hints) can influence which roles are selected at validation time, but they aren't persisted in the identity and aren't enforced at authorization time. This means if a resolved role grants multiple principals, the requester gets them all once the resource is reachable, so the final access can be "over-granted" and exceed the requester's intent.

## UX
**Goal**: Let users pick constraints per resource, see what’s **granted** vs. **requestable**, and either launch immediately or create a scoped Access Request, using one unified dropdown where the "connect"/"request" button currently sits on resource cards.

**User stories**:
- **AWS Console (role elevation)**:  
  Alice, a platform engineer with a standing read-only AWS role, needs elevated access to respond to an incident.  In Teleport's Web UI, she searches for the AWS Console app and clicks its connect dropdown. There, she sees both her read-only role (available right away) and the other roles she is able to request. She selects the 'admin' ARN, submits the request, and after approval, is able to assume the request and launch the console with that elevated ARN.
- **AWS IAM IC (permission-set elevation)**:  
  Ben operates in AWS IAM IC with a default `contributor` permission set but occasionally needs `BillingAdmin`. In Teleport's Web UI, he clicks the AWS IAM IC app's connect dropdown, selects `BillingAdmin` under the **requestable** section, submits the request, and once approved uses only that permission set without receiving extra IC permissions.
- **SSH node (login + optional sudo)**:  
  Diego can SSH to a service host as `deploy` but needs a temporary `admin` login (and possibly `sudo`) to perform maintenance. In Teleport's Web UI, he clicks the connect dropdown for the service node, and sees `deploy` under his **granted** roles, and `admin` under the **requestable** section. He selects `admin`, checks "allow sudo", submits the request, and after approval, connects with just `admin`.
- **Database (role/user)**:  
  Chandra has read-only access to a Postgres DB as `report_reader` and needs `migration_admin` for a one-off schema change. In Teleport's Web UI, she clicks the connect dropdown for the Postgres app, selects `migration_admin` under the **requestable** section, submits the request, and after approval, connects with only the `migration_admin` DB role for the session.

**Reviewer experience**: Reviewers see both **(a)** the requested constraints and **b)** the resolved roles computed to satisfy them.

## Design Overview
### ResourceID & Constraints
We add an optional, typed `Constraints` field to`ResourceID`. The content is domain-specific and enumerated.

```grpc
message ResourceID {
  // 1..4 are existing fields.
  string Cluster                  = 1 [(gogoproto.jsontag) = "cluster"];
  string Kind                     = 2 [(gogoproto.jsontag) = "kind"];
  string Name                     = 3 [(gogoproto.jsontag) = "name"];
  string SubResource              = 4 [(gogoproto.jsontag) = "sub_resource,omitempty"];
  // New: Optional constraints for this resource.
  ResourceConstraints Constraints = 5 [(gogoproto.jsontag) = "constraints,omitempty"];
}

enum ResourceConstraintDomain {
  CONSTRAINT_DOMAIN_UNSPECIFIED         = 0;
  CONSTRAINT_DOMAIN_AWS_CONSOLE         = 1;
  CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER = 2;
  CONSTRAINT_DOMAIN_SSH                 = 3;
  CONSTRAINT_DOMAIN_DATABASE            = 4;
}

message ResourceConstraints {
  ResourceConstraintDomain Domain = 1 [(gogoproto.jsontag) = "domain"];
  string Version = 2 [(gogoproto.jsontag) = "version"];

  oneof details {
    AWSConsoleConstraints AWSConsole = 10 [(gogoproto.jsontag) = "aws_console"];
    AWSIdentityCenterConstraints AWSIC = 11 [(gogoproto.jsontag) = "aws_ic"];

    // Examples for future expansion:
    SshConstraints SSH = 12 [(gogoproto.jsontag) = "ssh"];
    DbConstraints  DB  = 13 [(gogoproto.jsontag) = "db"];
  }
}

message AWSConsoleConstraints {
    repeated string RoleARNs = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.jsontag) = "role_arns"
    ];
}

message AWSIdentityCenterConstraints {
    repeated IdentityCenterAccountAssignment AccountAssignments = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.jsontag) = "account_assignments"
    ];
}

message SSHConstraints {
  repeated string logins = 1 [(gogoproto.jsontag) = "logins"];
}

message DBConstraints {
  repeated string database_users = 1 [(gogoproto.jsontag) = "roles"];
}
```

### Serialization of `ResourceConstraints`
To bind constraints to resources in certs, we replace the current slash-delimited `ResourceID` path string with a JSON blob that encodes both the `ResourceID` and its `ResourceConstraints`, when `ResourceConstraints` are present.

#### Format Examples
- **AWS Console ARN constraint**
  ```json
  {"cluster":"example.com","kind":"app","name":"aws-console","constraints":{"domain":1,"version":"v1","details":{"aws_console":{"role_arns":["arn:aws:iam::123456789012:role/Admin"]}}}}
  ```
- **SSH logins constraint**
  ```json
  {"cluster":"example.com","kind":"node","name":"bbbb56211-7b54-4f9e-bee9-b68ea156be5f","constraints":{"domain":3,"version":"v1","details":{"ssh":{"logins":["admin","web"]}}}}
  ```

#### Encoding
- JSON must be compacted (no whitespace), UTF-8, with stable field names and ordering.
- Fields with zero values (e.g., empty arrays, nil pointers) must be omitted to minimize size.
- Arrays within `ResourceConstraints` must be deduped and sorted lexicographically before encoding to ensure deterministic paths.
- If no constraints are present for a resource, the path string remains as it is today.

#### Parsing
1. If the string begins with a `{`, it is JSON and should be unmarshalled into `ResourceID`.
2. Otherwise, if the string begins with a `/`, it is a legacy slash-delimited string path without constraints and is parsed as it is today.

#### Limits & Safety
- **Size limits**:  
  We enforce a sane max encoded length, both per-`ResourceID` string, and for the total set of `AllowedResourceIDs` in a single cert/request. This helps keep cert sizes reasonable and avoid hard-to-debug transport issues we've hit in the past, namely, default gRPC message size limits.

  - Per-`ResourceID`: <=2KB (2048 bytes)
  - Per-request (all `AllowedResourceIDs`s combined): <=32KB (32768 bytes)

  These limits should provide a comfortable range for most use cases (e.g., a max 512-2048 chars per `ResourceID`, and up to 16 `ResourceID`s of max size in a single cert/request).
- **Safety**:  
  - JSON parsing errors, or malformed/unknown constraints, cause request validation to fail to avoid over-granting.
  - Older binaries that don't understand the JSON string path will fail to parse the resource ID (as they first check for a leading `/` character), preventing over-granting.

> **Note**: `SubResourceName` is incompatible with constraints.
> 
> Today, Kubernetes `ResourceIDs` may include `SubResourceName` (e.g., "namespace/pod"), which is serialized as an additional path segment after `<name>` (e.g., `/<cluster>/kube/<name>/<subResourceName>`). This field will not be supported when `Constraints` are present, as:
> - Kubernetes resources are not in scope for constraints, at least initially.
> - `SubResourceName` should eventually be migrated to a more structured, extensible format. `ResourceID.Constraints` could be a good home for that in the future.

### RBAC Semantics with Constraints
#### Validation & Resolution
Resource-based Access Requests including constraints are validated using constraint-aware role matchers derived from each `ResourceID.Constraints` (e.g., ARN/permission set/SSH login matchers). `RequestValidator` extends the existing `applicableSearchAsRoles`/`roleAllowsResource` flow by passing these matchers, so a role only qualifies if it allows the requested resource *and* the specified constraints. Resources are still pre-filtered by current access and  requested `ResourceIDs` are still deduplicated and size-capped, as they are currently. The resulting role list is the set of requestable/search-as roles that satisfies all `(resource, constraints)` pairs (when multiple roles qualify, behavior matches current semantics). If no roles qualify, existing failure behavior and errors apply.

#### Authorization
On approval/assumption, certs carry the exact `(resource, constraints)` pairs via `tlsca.Identity.AllowedResourceIDs` (each entry including `ResourceID.Constraints`). `AccessChecker` enforces those constraints: if a role grants multiple principals on a resource, approving a request for a narrower set (e.g., a single ARN, specific SSH logins, or IC permission sets) must only allow those requested items. By reading `ResourceID.Constraints` from `AllowedResourceIDs`, `AccessChecker` scopes evaluation accordingly and denies any not-requested principals even if the resolved role(s) in the cert would otherwise permit them.

### Web/API (high-level)
- Listing `UnifiedResources` surfaces per-constraint status when "show requestable" is enabled. For each item (ARN, permission set, SSH login), a `requiresRequest: true|false` marker is provided so the UI can render a single menu with "Granted" and "Requestable" sections.
- Resource Access Requests that include `ResourceID.Constraints` present both requested constraints and resolved roles to reviewers.

## Compatibility
- The `constraints` field is optional; existing clients and requests without constraints continue to work unchanged.
- UI can adopt the per-constraint "granted vs requestable" grouping incrementally by resource type.
- RBAC semantics are additive. Constraint matchers are considered only when constraints are present in a request.
- Serialized `ResourceID`s with constraints will not be parsable by older versions. This is an intentional fail-closed design to avoid potential over-granting if constraints are ignored.
  - For now, to keep non-constraint-bearing `ResourceID`s compatible with older agents, we retain the existing slash-delimited string format for those, and only use the JSON format when constraints are present.

## Next Steps
- Finalize UI designs and UX specifics (layout, grouping visuals, request checkout/review details).
- `tsh request` commands and Teleport Connect flows.
- More exhaustive resource/constraint coverage beyond the initial AWS Console/AWS IC focus (SSH/DB/...).
