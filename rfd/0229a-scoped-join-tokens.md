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
- Support for single use (oneshot) tokens
- New `scoped` variants of the `tctl tokens *` family of sub commands

Considered out of scope for this RFD:

- Token types other than static tokens or join methods other than "token".
  These will almost certainly be implemented in the future but are not part
  of the immediately planned work.

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
+}
+
+// The usage status of a oneshot token.
+message OneshotStatus {
+  // The timestamp representing when a oneshot token was successfully used for
+  // provisioning.
+  google.protobuf.Timestamp used_at = 1;
+  // The timestamp representing when a oneshot token should no longer be avaialable
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
```

The `usage` field of the scoped token status is a `oneof` in order to more
easily support other usage modes that may be added in the future (e.g. max
usage limits).

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

### Oneshot tokens

Single use, or oneshot, tokens provide a simple way of enforcing that a given
token can only be used to provision a single resource. A oneshot token can
be created by setting the `mode` field of a scoped token's spec to `oneshot`.
This will be implemented as first-come-first-served by recording the first
joining host's public key in the `status.used_by_public_key` field of the
scoped token used. Once a token has recorded a public key it will no longer be
usable by any other hosts. This could also be controlled by a simple boolean,
but using the public key allows for reuse of the oneshot token by the same
host. This is useful in cases where the host fails to join after credentials
have been issued due to some transient error receiving or storing those
credentials.

In order to prevent tokens from being indefinitely reusable by a given public
key, an optional final message should be added to the joining flow:

```protobuf
--- a/api/proto/teleport/join/v1/joinservice.proto
+++ b/api/proto/teleport/join/v1/joinservice.proto
@@ -436,6 +436,7 @@ message JoinRequest {
     OracleInit oracle_init = 9;
     TPMInit tpm_init = 10;
     AzureInit azure_init = 11;
+    Confirm confirm = 12;
   }
 }

@@ -501,6 +502,10 @@ message BotResult {
   optional BoundKeypairResult bound_keypair_result = 2;
 }

+// The final message sent from the client to the cluster signaling that credentials have been
+// successfully received.
+message Confirm {}
+
 // JoinResponse is the message type sent from the server to the joining client.
 message JoinResponse {
   oneof payload {
@@ -515,6 +520,10 @@ message JoinResponse {
     // the cluster when the join flow is successful.
     // For the token join method, it is sent immediately in response to the ClientInit request.
     Result result = 3;
+    // Confirm is the final message sent from the client back to the cluster after a successful join.
+    // It signals to the cluster that the client received their credentials and the token used can
+    // be consumed.
+    Confirm confirm = 4;
   }
 }
```

This new message will signal to the auth service that the host received and
stored their credentials. At which point, the scoped token's `used_at` field
should be updated with the current timestamp and future attempts to reuse the
token will fail with a token exhausted error. This is added to the end of the
join bidi stream in order to leverage auth's knowledge of which token was used
to initiate a join attempt. Otherwise the token name would need to be encoded
into the host certificate to prevent any sort of malicious consumption of other
tokens. As an additional measure, a token should only be reusable for a limited
time after the first public key has been recorded. Once this time has elapsed,
the token will be considered exhausted even when using the same public key. The
`reusable_until` field will be set with the current timestamp at the same time
as `used_by_public_key` in support of this.

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

Below are examples for each of the sub commands that will be implemented as
part of this RFD.

```bash
# adding a scoped token that will assign provisioned resources to the
# /staging/west scope
$ tctl scoped tokens add --type=node --scope=/staging/west--assign-scope=/staging/west
```

```bash
# adding a oneshot scoped token that will assign a provisioned resource to the
# /staging/west scope, automatically assign labels
$ tctl scoped tokens add \
  --type=node \
  --scope=/staging/west \
  --assign-scope=/staging/west \
  # ssh_labels follows the same format as the common --labels flag
  --ssh-labels=env=staging,hello=world \
  --mode oneshot
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
be used by the Teleport Auth Service during access control decisions. Because
joining can still be successful without issuing the new confirmation message,
oneshot tokens will also be usable by agents on previous versions. With the
caveat that token reuse will be possible until the `reusable_until` timestamp
has been reached.

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
- Oneshot Scoped tokens are not usable more than once.
