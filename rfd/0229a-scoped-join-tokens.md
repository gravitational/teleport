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
- Support for static and ephemeral tokens.
- Support for provisioning hosts using all join methods other than
  `bound_keypair`. This includes ephemeral and static tokens as well as
  delegated join methods like `iam`, `github`, and `tpm`.
- Support for configuring an `assigned_scope` on a scoped token which will
  in turn be assigned to joining nodes at provisioning time.
- Support for configuring a scoped token with a set of labels that are
  automatically assigned to SSH nodes at provisioning time.
- Support for single use tokens.
- New `scoped` variants of the `tctl tokens *` family of sub commands.

Considered out of scope for this RFD:

- Scoped joining for machine and workload identity. This means the
  `bound_keypair` join method will not be initially supported.

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
+  string join_method = 3;
+
+  // The usage mode of the token. Can be "single_use" or "unlimited". Single use
+  // tokens can only be used to provision a single resource. Unlimited tokens
+  // can be be used to provision any number of resources until it expires.
+  string mode = 4;
+
+  // Immutable labels that should be applied to any resulting resources
+  // provisioned using this token.
+  ImmutableLabels immutable_labels = 5;
+}
+
+// A set of configurations for immutable labels.
+message ImmutableLabels {
+    // Labels that should be applied to SSH nodes.
+    map<string, string> ssh = 1;
+  }
+}
+
+// The host certificate parameters that should be cached and leveraged for
+// token reuse
+message HostCertParams {
+  // The host ID generated for the host.
+  string host_id = 1;
+  // The host's name.
+  string node_name = 2;
+  // The system role of the host
+  string role = 3;
+  // The additional principals to include
+  repeated string additional_principals = 4;
+  // The scope to assign the host to.
+  string assigned_scope = 5;
+}
+
+// The usage status of a single use token.
+message SingleUseStatus {
+  // The timestamp representing when a single use token was successfully used for
+  // provisioning.
+  google.protobuf.Timestamp used_at = 1;
+  // The timestamp representing when a single use token should no longer be available
+  // for idempotent retries.
+  google.protobuf.Timestamp reusable_until = 2;
+  // The fingerprint of the public key provided by the host that used the token.
+  bytes used_by_fingerprint = 3;
+  // The relevant host parameters provided while initially using a single use token.
+  // Attempts to reuse the token will apply these same parameters while regenerating
+  // certificates.
+  bytes host_cert_params = 4;
+}
+
+// The status of a scoped token.
+message ScopedTokenStatus {
+  // The usage status of the token.
+  oneof usage {
+    // The usage status of a single use token.
+    SingleUseStatus single_use = 1;
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
unscoped tokens will result in an error message describing the collision and an
immediate join failure. It will be up to the administrator to resolve the
ambiguity. Either by deleting one of the conflicting tokens or creating a new
token with a different name. If ambiguity were introduced mid-join by creating
a token with a duplicate name, it would have no effect as the join process will
retain the initially resolved token for the duration of the flow. It's worth
noting that the node agent will continue join attempts after collisions just as
it does today for other join failures. Meaning that if the ambiguity is
resolved the node may join successfully without intervention.

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
Application, Database, etc.) introduces significant complexity. However the
proposed message structure allows for easy extension into more agent types.
The `immutable_labels` field of a `ScopedToken` will be deterministically
sorted, hashed, and encoded into the resulting host certificates in much the
same way as the `assigned_scope`. These labels will be returned in the result
of a successful join, stored in proc state, and included while registering the
inventory control stream. The call to register the control stream will also
verify that the labels' hashed representation matches the certificate. A new
`immutable_labels` field will be added to `ServerSpecV2` and future heartbeats
will verify `immutable_labels` just as they do for static and command labels.

Merging token-assigned labels with static labels was also considered as a way
to more easily integrate with existing flows surrounding labels, but ultimately
decided against. Adding a new set of labels eliminates any ambiguity around
conflict resolution and allows for future security controls that select on
labels that cannot be maliciously escaped. That said, token labels will be
combined with static labels in contexts that call for it (e.g.
`ServerV2.GetAllLabels()`). In these cases, the token labels will take ultimate
precedence and cannot be overridden by either static or dynamic labels.

Storing the hash on the certificate instead of the labels prevents exposing
a host's labels in an effectively public way and allows for the labels to be
arbitrarily sized without inflating the certificate weight.

### Single use tokens

Single use tokens provide a simple way of enforcing that a given token can only
be used to provision a single node. A single use token can be created by
setting the `mode` field of a scoped token's spec to `single_use`. This will be
implemented as first-come-first-served by recording the fingerprint of the
first joining host's public key in the `status.used_by_fingerprint` field of
the scoped token used. Once a token has recorded a fingerprint it will no
longer be usable by any other hosts. This could also be controlled by a
boolean, but using the fingerprint allows for reuse of the single use token by
the same host as long as it provides the same public key. This is convenient in
cases where the host fails to join after credentials have been issued due to
some transient error receiving or storing those credentials. Some of the fields
relevant to certificate generation are also included in the
`status.host_cert_params` field. When a single use token is reused, these
values should override any values already present in the join attempt when
regenerating the certificate.

In order to prevent indefinite reuse of a token by a given host, the
`reusable_until` field will be set with an expiration timestamp representing 30
minutes after the public key has been recorded. After this time, the token will
no longer be usable at all.

By default, a new scoped token should be created with a `mode` of `unlimited`
unless another mode is explicitly specified.

#### Clock skew

When checking whether or not a single-use scoped token can be reused, the
Teleport Auth service will allow for up to 5 minutes of clock skew. It is also
recommended that all Teleport Auth Service instances leverage NTP to ensure
that clock skew is minimized. Any skew in individual SSH node clocks is not
relevant when evaluating token expirations.

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
at token creation. When a name is not explicitly provided, a UUIDv4 will be
used to more easily differentiate the token's name from the secret.

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
# adding a single use scoped token that will assign a provisioned resource to the
# /staging/west scope and automatically assign labels
$ tctl scoped tokens add \
  --type=node \
  --scope=/staging/west \
  --assign-scope=/staging/west \
  # --ssh-labels follows the same format as the common --labels flag
  --ssh-labels=env=staging,hello=world \
  --mode single_use
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

## Audit events

Interacting with scoped tokens should generate the following audit events:

- `scoped_token.created`: When a scoped token is created (e.g. with
  `tctl scoped tokens add`).
- `scoped_token.deleted`: When a scoped token is deleted (e.g. with
  `tctl scoped tokens rm`).
- `scoped_token.used`: When a scoped token is used to generate host
  certificates.
- `scoped_token.use_failed`: When a resource fails to join while using a
  scoped token.

These will contain metadata similar to the existing `join_token.created` event
with a few additions specific to scoped tokens.

```diff
--- a/api/proto/teleport/legacy/types/events/events.proto
+++ b/api/proto/teleport/legacy/types/events/events.proto
@@ -2472,6 +2472,47 @@ message ProvisionTokenCreate {
   ];
 }

+// The event emitted when a scoped token is created.
+message ScopedTokenCreate {
+  Metadata metadata = 1;
+  ResourceMetadata resource = 2;
+  UserMetadata user = 3;
+  repeated string roles = 4;
+  string join_method = 5;
+  string usage_mode = 6;
+  string scope = 7;
+  string assigned_scope = 8;
+}
+
+// The event emitted when a scoped token is deleted.
+message ScopedTokenDelete {
+  Metadata metadata = 1;
+  ResourceMetadata resource = 2;
+  UserMetadata user = 3;
+}
+
+// The event emitted when a scoped token is used to provision a resource.
+message ScopedTokenUse {
+  Metadata metadata = 1;
+  ResourceMetadata resource = 2;
+  repeated string roles = 4;
+  string join_method = 3;
+  string usage_mode = 6;
+  string scope = 7;
+  string assigned_scope = 8;
+}
+
+// The event emitted when a scoped token fails to provision a resource.
+message ScopedTokenFailed {
+  Metadata metadata = 1;
+  ResourceMetadata resource = 2;
+  repeated string roles = 4;
+  string join_method = 3;
+  string usage_mode = 6;
+  string scope = 7;
+  string assigned_scope = 8;
+}
+
```

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
- Single use scoped tokens are not usable more than once.
