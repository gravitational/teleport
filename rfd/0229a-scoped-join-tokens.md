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
  scoped access rules.
- Support for provisioning using the "token" join method.
- Support for static and ephemeral tokens.
- Support for configuring an `assigned_scope` on a scoped token which will
  in turn be assigned to joining nodes at provisioning time.
- Support for configuring a scoped token with a set of labels that are
  automatically assigned to SSH nodes at provisioning time.
- Support for single use (oneshot) tokens.
- New `scoped` variants of the `tctl tokens *` family of sub commands.

Considered out of scope for this RFD:

- Token types other than static and ephemeral tokens or join methods other than
  "token". Other join methods will almost certainly be implemented in the
  future but are not part of the immediately planned work.

### Scoped tokens

The recently added `ScopedToken` resource will be modified to include fields
necessary for assigning scopes, labels, and enforcing single use. Additionally,
fields missing from the existing `types.ProvisionTokenV2` will be ported over
to facilitate existing provisioning semantics.

```diff
--- a/api/proto/teleport/scopes/joining/v1/token.proto
+++ b/api/proto/teleport/scopes/joining/v1/token.proto
@@ -16,6 +16,7 @@ syntax = "proto3";

 package teleport.scopes.joining.v1;

+import "google/protobuf/timestamp.proto";
 import "teleport/header/v1/metadata.proto";

 option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1;joiningv1";
@@ -42,12 +43,48 @@ message ScopedToken {

   // Spec is the token specification.
   ScopedTokenSpec spec = 6;
+
+  // The status of the token.
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
+  // The usage mode of the token. Can be "oneshot" or "unlimited". Oneshot
+  // tokens can only be used to provision a single resource. Unlimited tokens
+  // can be be used to provision any number of resources until it expires.
+  string mode = 4;
+
+  // Immutable labels that should be encoded into the certificate of an
+  // SSH node provisioned using this token.
+  map<string, string> ssh_labels = 5;
+}
+
+// The usage status of a oneshot token.
+message OneshotStatus {
+  // The timestamp representing when a oneshot token was successfully used for
+  // provisioning.
+  google.protobuf.Timestamp used_at = 1;
+  // The timestamp representing when a oneshot token should no longer be available
+  // for idempotent retries.
+  google.protobuf.Timestamp reusable_until = 2;
+  // The public key of the host that used the token.
+  bytes used_by_public_key = 3;
+}
+
+// The status of a scoped token.
+message ScopedTokenStatus {
+  // The usage status of the token.
+  oneof usage {
+    // The usage status of a oneshot token.
+    OneshotStatus oneshot = 1;
+  }
+  // The secret that must be provided along with the token's name in order to
+  // be used.
+  string secret = 2;
```

The `usage` field of the scoped token status is a `oneof` in order to more
easily support other usage modes that may be added in the future (e.g. max
usage limits).

The `secret` field represents a departure from the existing token flow in that
the token's name is no longer the secret. This is similar to how the bounded
keypair join method manages the registration secret. The `token` join method
will require specifying both the token's name and secret when using scoped
tokens.

### Provisioning

Scoped resources will be provisioned by extending the existing `Join` RPC to
support scoped tokens. When the join method is `token` the auth server will
query both the existing `ProvisionTokenV2` and the new `ScopedToken` resources
using the provided token name. Conflicting token names between scoped and
unscoped tokens will result in an error and it will be up to the administrator
to resolve the ambiguity. Either by deleting one of the conflicting tokens or
creating a new token with a different name.

A new `token_secret` field will be included in the `TokenInit` message when
joining with a scoped token. This secret must match the secret found in the
retrieved token in order to proceed.

The `assigned_scope` of the `ScopedToken` will be assigned as the scope of the
resulting node. It will also be attached as metadata to the node's host
certificates under the `AgentScope` field so that scope related access controls
can be performed against the agent's identity rather than just restricting
clients' access to the agent itself.

### Automatic labels for SSH nodes

Scoped tokens should also support automatic assignment of immutable labels to
any provisioned Teleport SSH agents. This document only proposes support for
Teleport SSH agents as applying them to other scoped Teleport services (i.e.
Application, Database, etc.) introduces significant complexity. The
`ssh_labels` field of a `ScopedToken` will be encoded into the resulting host
certificates in much the same way as the `assigned_scope`. These labels will be
extracted while registering the inventory control stream and applied as a new
`token_labels` field stored in `ServerSpecV2`. Future heartbeats will verify
`token_labels` just as they do for static labels and command labels.

Merging token-assigned labels with static labels was also considered as a way
to more easily integrate with existing flows surrounding labels, but ultimately
decided against. Adding a new set of labels eliminates any ambiguity around
conflict resolution and allows for future security controls that select on
labels that cannot be maliciously escaped.

In order to prevent inflating the weight of the resulting host certificates,
the `token_labels` will initially be limited to a total size of 2kb. This limit
will only be enforced when generating the host certificates in order to allow
future adjustments to sizing without requiring agent upgrades.

### Oneshot tokens

Single use, or oneshot, tokens provide a simple way of enforcing that a given
token can only be used to provision a single resource. A oneshot token can
be created by setting the `mode` field of a scoped token's spec to `oneshot`.
This will be implemented as first-come-first-served by recording the first
joining host's public key in the `status.used_by_public_key` field of the
scoped token used. Once a token has recorded a public key it will no longer be
usable by any other hosts. This could also be controlled by a boolean, but
using the public key allows for reuse of the oneshot token by the same host as
long as it provides the same public key. This is useful in cases where the host
fails to join after credentials have been issued due to some transient error
receiving or storing those credentials.

In order to prevent indefinite reuse of a token by a given host, the
`reusable_until` field will be set with an expiration timestamp representing 30
minutes after the public key has been recorded. After this time, the token will
no longer be usable at all.

By default, a new scoped token should be created with a `mode` of `unlimited`
unless another mode is explicitly specified.

### `tctl` subcommands

In order to provision and manage scoped tokens, we will extend `tctl` with
`scoped` variants of the existing `tokens` sub commands. The UX of these new
sub-commands will be nearly identical to working with unscoped provisioning
tokens. The only concrete differences are the addition of some new flags
specific to scoped tokens and the fact that these operations themselves are
subject to scoped access controls. Meaning that you will not be able to create
a token for a scope that is orthogonal or ancestor to the scope of your own
access credentials.

The scoped variant of `tctl token add` will also replace the optional `--value`
flag with `--name` to better represent what the flag influences. The `--value`
override will not be implemented as the secret will be automatically generated
at token creation.

Below are examples for each of the sub commands that will be implemented as
part of this RFD.

```bash
# adding a scoped token that will assign provisioned resources to the
# /staging/west scope
$ tctl scoped tokens add --type=node --scope=/staging/west --assign-scope=/staging/west
```

```bash
# adding a scoped token with an explicit name
$ tctl scoped tokens add \
  --type=node \
  --scope=/staging/west \
  --assign-scope=/staging/west \
  --name=foo
```

```bash
# adding a oneshot scoped token that will assign a provisioned resource to the
# /staging/west scope and automatically assign labels
$ tctl scoped tokens add \
  --type=node \
  --scope=/staging/west \
  --assign-scope=/staging/west \
  # --ssh-labels follows the same format as the common --labels flag
  --ssh-labels=env=staging,hello=world \
  --mode oneshot
```

```bash
# removing a scoped token by name
$ tctl scoped tokens rm <token-name>
```

```bash
# List all scoped tokens visible to the scope of the current credentials
$ tctl scoped tokens ls
```

### Config Changes

Static scoped tokens will be enabled by adding a `scoped_tokens` block to the
`auth_service` section of the config file. The typical token strings will be
broken into their constituent parts with the addition of the `scope` and
`secret` fields.

For example, the configuration below will create two tokens `foo` and `bar`
that can both be used to provision nodes. The `foo` token will be unscoped and
`bar` will be scoped to `/staging`.

```yaml
auth_service:
  tokens:
    - node:foo
  scoped_tokens:
    - name: bar
      roles: [node]
      scope: /staging
      secret: asdf1234
```

A `token_secret` field will also be added to the `join_params` block of the
`teleport` section in order to provide the secret and the name when using
scoped tokens. This field will function exactly like the `token_name` field for
unscoped tokens in that it can source its value from a file. The `auth_token`
field will not be supported for scoped tokens.

```yaml
teleport:
  join_params:
    method: token
    token_name: bar
    token_secret: asdf1234
```

Finally, a `--token-secret` flag will be added to the `teleport start` command
in order to maintain usage parity with unscoped tokens.

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
  - Assigned the token's `--ssh-labels`
  - Assigned a certificate containing their assigned scope and token labels
- Oneshot Scoped tokens are not usable more than once.
