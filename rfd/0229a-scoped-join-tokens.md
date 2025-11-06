---
authors: Erik Tate (erik.tate@goteleport.com)
state: draft
---

# RFD 0229a - Scoped Join Tokens

## Required approvers

- Engineering: @fspmarshall && @rosstimothy
- Security: @rob-picard-teleport

## What

A new type of provisioning token with configurable usage limits that
automatically applies a scope and labels to provisioned resources.

## Why

The introduction of scoped access, as defined in the [Scopes RFD](./0229-scopes.md),
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
  automatically assigned to SSH nodes at provisioning time
- Support for defining a maximum number of uses a scoped token can support
- New `scoped` variants of the `tctl tokens *` family of sub commands

Considered out of scope for this RFD:

- Token types other than static tokens or join methods other than "token".
  These will almost certainly be implemented in the future but are not part
  of the immediately planned work.

### Scoped tokens

The recently added `ScopedToken` resource will be modified to include fields
necessary for assigning scopes, labels, and enforcing limited uses.
Additionally, fields missing from the existing `types.ProvisionTokenV2` will be
ported over to facilitate existing provisioning semantics.

```diff
index ed3f43847c1..813e50daf82 100644
--- a/api/proto/teleport/scopes/joining/v1/token.proto
+++ b/api/proto/teleport/scopes/joining/v1/token.proto
@@ -42,12 +42,33 @@ message ScopedToken {

   // Spec is the token specification.
   ScopedTokenSpec spec = 6;
+  // The status of the token
+  ScopedTokenStatus status = 7;
 }

 // ScopedTokenSpec is the specification of a scoped token.
 message ScopedTokenSpec {
-  // AssignedScope is the scope to which this token is assigned.
+  // The scope to which this token is assigned.
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
+  // The set of labels to be automatically assigned to any provisioned SSH node.
+  map<string, string> ssh_labels = 4;
+
+  // The number of resources that can be provisioned using this token.
+  int32 max_uses = 5;
+}
+
+// The status of a scoped token.
+message ScopedTokenStatus {
+  // The number of successful provisioning attempts made using this token.
+  int32 attempted_uses = 6;
 }
```

### Provisioning

Scoped resources will be provisioned by extending the existing `Join` RPC to
support scoped tokens. When the join method is `token` the auth server will
query both the existing `ProvisionTokenV2` and the new `ScopedToken` resources
using the provided token name. Conflicting token names between scoped and
unscoped tokens will result in an error and it will be up to the administrator
to resolve the ambiguity. Either by deleting one of the conflicting tokens or
creating a new token with a different name. This approach allows for backwards
compatible provisioning where agents can be scoped without any knowledge of
scopes themselves. The `assigned_scope` of the `ScopedToken` will be assigned
as the scope of the resulting resource. It will also be attached as metadata to
the resulting host certificate under the `AgentScope` field so that scope
related access controls can be performed against the agent's identity rather
than just restricting clients' access to the agent itself.

### Automatic labels for SSH nodes

Scoped tokens should also support automatic assignment of labels to any SSH
node provisioned. This document only proposes support for SSH nodes as solving
for all scoped node types introduces more significant complexity. The
`ssh_labels` field of a `ScopedToken` will be added encoded into the resulting
host certificate in much the same way as the `assigned_scope`. These labels
will be extracted while registering the inventory control stream and applied as
a new `token_labels` field stored in `ServerSpecV2`. Future heartbeats will
verify `token_labels` just as they do for static labels and command labels.

Merging token-assigned labels with static labels was also considered as a way
to more easily integrate with existing flows surrounding labels, but ultimately
decided against. Adding a new set of labels eliminates any ambiguity around
conflict resolution and allows for future security controls that select on
labels that cannot be maliciously escape.

In order to prevent inflating the weight of the resulting host certificates,
the `token_labels` will initially be limited to a total size of 2kb. This limit
will only be enforced when generating the host certificates in order to allow
future adjustments to sizing without requiring agent upgrades.

### Limited use tokens

The simplest way to implement token usage limits is to keep a counter of
successful provisioning attempts and fallback on the conditional update system
to maintain consistency. A "successful provisioning attempt" means that the
auth service generated host certificates, but it does not guarantee that the
resource successfully joined the cluster. Whenever a resource is provisioned
using a token, we increment the token's `attempted_uses` field with a
conditional update. If the `attempted_uses` reaches the `max_uses`, the token
will no longer be usable and should eventually be cleaned up. This is simple to
reason about but requires that the final decision about whether or not the
token can be used has to be deferred to the end of the provisioning process.
Otherwise it would be possible for many concurrent join attempts to exceed the
allowed `max_uses` for a token.

Configuring a token without defining `max_uses` would effectively disable usage
limits and fallback to the typical expiration behavior tokens have today.
Tokens that have exceeded their usage limits will remain in the backend, but
will not be usable. This would allow the posibility of extending `max_uses` to
effectively reenable a token that has been used up.

Individual auth service instances should guard incrementing the
`attempted_uses` field using a mutex in order to prevent excessive retries
within a single server due to conditional update failures. This will not
prevent conditional update failures from occurring across multiple auth service
instances.

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
$ tctl scoped tokens add --type=node --scope=/staging/west--assign-scope=/staging/west
```

```bash
# adding a scoped token that will assign provisioned resources to the
# /staging/west scope, automatically assign labels, and limit provisioned
# resources to 5
$ tctl scoped tokens add \
  --type=node \
  --scope=/staging/west \
  --assign-scope=/staging/west \
  # ssh_labels follows the same format as the common --labels flag
  --ssh-labels=env=staging,hello=world \
  --max-uses 5
```

```bash
# removing a scoped token by name
$ tctl scoped tokens rm <token-name>
```

```bash
# List all scoped tokens assigning provisioned resources to scopes subject to
# the target scope /staging (e.g. tokens assigning /staging/west and
# /staging/east will all be returned). Descendant matches are also the default
# behavior when filtering on scope
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

## Security considerations

Scoped tokens themselves will make use of scoped access controls. Meaning that
a scoped identity must have a scoped role assignment permitting the given
action taken against a scoped token resource.

Similar to scoped roles and scoped role assignments themselves, scoped tokens
should still be accessible to unscoped identities using the `editor` role.

# Test plan

- Scoped tokens can be created using `tctl scoped tokens add`
- Scoped tokens can be removed using `tctl scoped tokens rm`
- Scoped tokens can be listed using `tctl scoped tokens ls`
- Above commands are scope aware. Meaning adding or removing a token only
  succeed within accessible scopes and listing tokens only includes tokens
  from accessible scopes.
- SSH nodes joining with a scoped token are:
  - Assigned the correct scope
  - Assigned the token's `ssh_labels`
  - Assigned a certificate containing their assigned scope and token labels
- Scoped tokens are nout usable after their `--max-uses` limit has been reached
