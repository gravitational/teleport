| Authors                                  | State |
|------------------------------------------|-------|
| Maxim Dietz (maxim.dietz@goteleport.com) | draft |

# RFD 0228 - Requesting Specific Logins for Resources

## Required Approvers
- Engineering: (@r0mant && @smallinsky)

## Related
- See [#58307](https://github.com/gravitational/teleport/issues/58307)

## What
Allow a user to request specific logins for a resource (e.g., an AWS role ARN, an IAM IC permission set, a DB user, or an SSH login), and have the Access Request:
1.  map those selections to the minimal set of roles needed, and
2.  scope the approved session to only the requested resource + the selected logins, even when the backing roles are wildcarded.

This applies first to **AWS Console** and **AWS IC Account** apps, but is designed to generalize to other resource kinds (DBs, Nodes, etc).

## Why
Users often already have access to a resource but need to escalate privileges (e.g., assume a higher-privilege AWS role) for tasks. Doing that currently through Access Requests is unintuitive; the Web UI does not clearly separate what you can already use from what you can request.

By listing granted vs requestable logins side-by-side per resource in a single dropdown, users can either connect immediately or select additional logins to request.

## Proposed Changes

1. **Extend `ResourceID` type to carry requested logins**:
   Access Requests can include multiple logins for the same resource. If omitted, behavior is unchanged.
   ```go
   type ResourceID struct {
     Kind            string            `json:"kind"`
     Name            string            `json:"name"`
     ClusterName     string            `json:"clusterName"`
     SubResourceName string            `json:"subResourceName,omitempty"`
     // New: list of requested logins for this resource
     RequestedLogins []RequestedLogin  `json:"requestedLogins,omitempty"`
   }

   // RequestedLogin expresses a specific login identity for a resource.
   type RequestedLogin struct {
     // Domain is the namespace: "aws_role_arn", "aws_ic_permission_set", "ssh_login", "db_user", ...
     Domain LoginDomain `json:"domain"`
     // Value is a concrete identifier: role ARN, permission-set identifier,
     // unix login, database user, etc.
     Value  string `json:"value"`
   }
   ```
   ```grpc
   enum LoginDomain {
     LOGIN_DOMAIN_UNSPECIFIED           = 0;
     LOGIN_DOMAIN_AWS_ROLE_ARN          = 1;
     LOGIN_DOMAIN_AWS_IC_PERMISSION_SET = 2;
     LOGIN_DOMAIN_SSH_LOGIN             = 3;
     // ...
   }

   message RequestedLogin {
     LoginDomain Domain = 1;
     string Value = 2;
   }

   message ResourceID {
     // 1..4 are existing fields
     string ClusterName = 1 [(gogoproto.jsontag) = "cluster"];
     string Kind        = 2 [(gogoproto.jsontag) = "kind"];
     string Name        = 3 [(gogoproto.jsontag) = "name"];
     string SubResourceName = 4 [(gogoproto.jsontag) = "sub_resource,omitempty"];
     // New: list of requested logins for this resource
     repeated RequestedLogin RequestedLogins = 5 [(gogoproto.jsontag) = "requested_logins,omitempty"];
   }
   ```
2. **Surface per-login availability in Unified Resources**:
   When the Web UI requests w/ both "include logins" and "include requestable," each resource includes its login options, both:
    -   granted: usable now (connect)
    -   requestable: eligible to request (checkboxes)

   There is currently no standard for how logins are returned and consumed by the Web UI (AWS Console apps populate `AwsRoles`, AWS IAM IC apps `PermissionSets`). Unifying this, we should extend the `UnifiedResource` / `EnrichedResource` types to carry more information about logins, e.g.,
   ```go
   type EnrichedResource struct {
       ResourceWithLabels
       Logins          []string // Currently, a list of strings is used.
       RequiresRequest bool
       LoginsWithDetails []LoginWithDetails // Unsure on the best naming here.
   }

   type LoginWithDetails struct {
       RequestedLogin
       // Display is a human-readable display name for front-end use.
       Display string `json:"display,omitempty"`
       // RequiresRequest notes whether this login is currently granted, or is requestable
       RequiresRequest bool `json:"requiresRequest"`
   }
   ```
3. **Role resolution honors `RequestedLogins`**:
   For resource-based requests with `RequestedLogins`, `validateAccessRequestForUser` should resolve only roles that allow this resource with these logins. Wildcard roles are fine; the session is still narrowed.
4. **Scope reflects selected logins**:
   Approved identities are scoped via `AllowedResourceIDs`. We additionally record the chosen logins, so a broad role effectively applies only to the requested `(resource, login)` pairs.

## Backwards Compatibility
- New fields are optional; existing APIs and clients continue to work.
- If `requested_logins` is empty, requests behave as they do currently.
- If `logins_with_details` is not specified, front-end falls back to current behavior.

## Open Questions
-   **Limits:** Reasonable cap on `RequestedLogins` per resource (e.g., 32).
-   **Kubernetes:** How to harmonize current kube SubResources model with this model over time (/do we want to)?


