---
authors: Erik Tate (erik.tate@goteleport.com)
state: draft
---

# RFD XXXX - Scoped Join Tokens

## Required approvers

- Engineering: @fspmarshall && @rosstimothy

## What

A new type of provisioning token with configurable usage limits that
automatically applies a scope and labels to provisioned resources.

## Why

The introduction of scoped access, as defined in the [Scopes RFD](./XXXX-scopes.md),
requires that resources are assigned a scope at provisioning time, which
implies some kind of extension to our provisioning capabilities. Scopes also
introduce an entirely new access control paradigm to Teleport. Due to the
complexity involved with modifying all of our existing mechanisms to support
scopes, we have chosen to implement them as parallel flows with minimal direct
overlap of our existing flows. Which is why this RFD proposes implementing a
new resource type and API for managing scoped join tokens.

This also provides a good opportunity and clean break for extending what's
possible with provisioning tokens. As such this document also proposes the
addition of automatically assigned labels and configurable usage limits for
scoped tokens.

## Overview

### Planned scope

The scope of work proposed by this RFD is:

- A `ScopedToken` resource type and API that is itself guarded by the new
  scoped access rules
- Support for static tokens and provisioning using the "token" join method.
- Support for configuring an `assigned_scope` on a scoped token which will
  in turn be assigned to new resources at provisioning time
- Support for configuring a scoped token with a set of labels that are
  automatically assigned to a resource at provisioning time
- Support for defining a maximum number of resources a scoped token can
  provision
- New `scoped` variants of the `tctl tokens *` family of sub commands

Considered out of scope for this RFD:

- Token types other than static tokens or join methods other than "token".
  These will almost certainly be implemented in the future but are not part
  of the immediately planned work.

### Scoped Tokens

The recently added `ScopedToken` resource will be modified to include fields
necessary for assigning scopes, labels, and enforcing limited uses.
Additionally, fields missing from the existing `types.ProvisionTokenV2` will be
ported over to facilitate existing provisioning semantics.

```proto
--- a/api/proto/teleport/scopes/joining/v1/token.proto
+++ b/api/proto/teleport/scopes/joining/v1/token.proto
@@ -16,7 +16,9 @@ syntax = "proto3";

 package teleport.scopes.joining.v1;

+import "google/protobuf/timestamp.proto";
 import "teleport/header/v1/metadata.proto";
+import "teleport/legacy/types/types.proto";

 option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1;joiningv1";

@@ -46,8 +48,29 @@ message ScopedToken {

 // ScopedTokenSpec is the specification of a scoped token.
 message ScopedTokenSpec {
-  // AssignedScope is the scope to which this token is assigned.
+  // The scope to which resources provisioned using this token are assigned.
   string assigned_scope = 1;

-  // TODO(fspmarshall): port relevant token features to scoped tokens.
+  // The list of roles associated with the token. They will be converted
+  // to metadata in the SSH and X509 certificates issued to the user of the
+  // token.
+  repeated string roles = 2;
+
+  // The joining method required in order to use this token.
+  // Supported joining methods for scoped tokens only include 'token'.
+  string join_method = 3;
+
+  // The list of rules applied to resources using this token.
+  // At least one rule must match the resource in order to join.
+  repeated types.TokenRule allow = 4;
+
+  // The set of labels to be automatically assigned to any provisioned resource.
+  map<string, string> assigned_labels = 5;
+
+  // The number of resources that can still be provisioned using this token.
+  int32 remaining_uses = 6;
 }
```

### Provisioning

Scoped resources will be provisioned by extending the existing `Join` RPC to
support scoped tokens. When the join method is `token` the auth server will
query both the existing `ProvisionTokenV2` and the new `ScopedToken` resources
using the provided token name. If a `ScopedToken` is found then it will take
precedence over any `ProvisionTokenV2` that might share the same name. This
approach allows for backwards compatible provisioning where agents can be
scoped without any knowledge of scopes themselves. The `assigned_scope` of the
`ScopedToken` will be assigned as the scope of the resulting resource. It will
also be attached as metadata to the resulting host certificate under the
`AgentScope` field so that scope related access controls can be performed
against the agent's identity rather than just restricting clients' access to
the agent itself.

### Automatic Labels

Similar to assigning the provisioned resource scope from the `ScopedToken`
used, the same token's `assigned_labels` field will be applied to the resulting
resource directly after provisioning. Because static labels exist exclusively
as configuration within the agent's `teleport.yaml` file, scoped token
provisioning will use the `server_info` resource to apply
[resource-based labels](https://goteleport.com/docs/zero-trust-access/rbac-get-started/labels/#apply-resource-based-labels)
upon creation. This applies labels at runtime using the inventory control
stream and does not require any new resources or RPCs. Labels will be created
at the end of `RegisterUsingToken()` before returning the host certificate for a
successfully provisioned resource.

### Limited Use Tokens

The simplest way to implement token usage limits is to allow for eventual
consistency without strong guarantees on usage limits. Whenever a resource is
provisioned using a token, we decrement the token's `remaining_uses` field with
a conditional update and delete the token once that number reaches zero. This
is simple to reason about but makes the tradeoff that multiple resources being
provisioned at the same time could exceed the number of uses before the token
was deleted.

Because joining happens over a single bidirectional streaming RPC, we can
easily wrap the simple approach with a distributed lock keyed on the token's
name. A reasonable TTL configured for the lock would account for auth server
crashes, but we would otherwise have complete visibility into whether or not
provisioning was successful and could release the lock appropriately. This has
the reverse tradeoff in that we can have strong guarantees about usage limits
but only one resource can be provisioned at any given time.

Provided that concurrent provisioning of resources using the same token is not
a requirement, this seems to balance implementation simplicity without
sacrificing hard limits.

To facilitate this, we would augment the `ScopedTokenService` API to include a
method with the following signature:

```go
func WithScopedToken(ctx context.Context, tokenName string, scopedFn func (ctx context.Context, token *ScopedToken) (bool, error)) error {
}
```

This function would acquire a distributed lock on `scoped_token/<token-name>`
and fetch the `ScopedToken` from the backend. It would then ensure that there
are still uses remaining before passing the fetched token to the `scopedFn`
callback. The bool returned by `scopedFn` represents whether or not a resource
was successfully provisioned. When provisioning is successul, `WithScopedToken`
will either decrement the remaining uses or delete the `ScopedToken` if there
are no uses remaining. Finally, it will release the held lock.

### `tctl` subcommands

In order to provision and manage scoped tokens, we will extend `tctl` with
`scoped` variants of the existing `tokens` sub commands. The UX of these new
sub-commands will be nearly identical to working with unscoped provisioning
tokens. The only concrete differences are the addition of some new flags
specific to scoped tokens and the fact that these operations themselves are
subject to scoped access controls. Meaning that you will not be able to create
a token for a scope that is orthogonal or ancestor to the scope of your own
access credentials.

Below are examples for each of the sub commands that will be implemented as
part of this RFD.

```bash
# adding a scoped token that will assign provisioned resources to the
# /staging/west scope
$ tctl scoped tokens add --type=node --scope=/staging/west
```

```bash
# adding a scoped token that will assign provisioned resources to the
# /staging/west scope, automatically assign labels, and limit provisioned
# resources to 5
$ tctl scoped tokens add \
  --type node \
  --scope /staging/west \
  --labels env=staging,hello=world \
  --max-uses 5
```

```bash
# removing a scoped token by name
$ tctl scoped tokens rm <token-name>
```

```bash
# List all scoped tokens assigning provisioned resources to descendants of the
# target scope /staging (e.g. tokens assigning /staging/west and /staging/east
# will all be returned). Descendant matches are also the default behavior when
# filtering on scope
$ tctl scoped tokens ls --scope=/staging
```

```bash
# List all scoped tokens assigning provisioned resources to ancestors of the
# target scope /staging/east (e.g. tokens assigning /staging/east and /staging
# will all be returned, but /staging/west will not be)
$ tctl scoped tokens ls --scope=/staging/east --mode=ancestor
```

### Agent Changes

As mentioned earlier in this RFD, scoped tokens are meant to be backwards
compatible for Teleport agents. Which means the existing mechanisms for
joining, such as `join_params` in `teleport.yaml` or the
`--token` flag of `teleport start`, will continue to work in the same way. The
issued host certificate will be ammended with an `AgentScope` field which will
be used by the Teleport Auth Service during access control decisions.

## Modifying provisioned resource scope

Modification of the scope assigned to a resource will not be permitted
initially. Reprovisioning the resource using a new scoped token configured with
the correct scope will be the only way to "move" a resource into another scope.

# Test plan

- Scoped tokens can be created using `tctl scoped tokens add`
- Scoped tokens can be removed using `tctl scoped tokens rm`
- Scoped tokens can be filtered by `assigned_scope` using
  `tctl scoped tokens ls --scope <scope>`
- Resource joining with a scoped token are:
  - Assigned the correct scope
  - Assigned the token's `assigned_labels` as dynamic resource labels
  - Assigned a certificate containing their assigned scope
- Scoped tokens are automatically removed after provisioning `--max-uses`
  resources
