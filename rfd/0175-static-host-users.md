---
author: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 175 - Static Host Users

## Required Approvers

- Engineering: @rosstimothy && @lxea && @@espadolini

## What

Teleport nodes will be able to create host users statically, i.e. independently
of a Teleport user creating one when SSHing with the current host user creation.

## Why

Host users can be created and used (potentially by third-party services) without
a Teleport user needing to log in first.

## Details

### UX

To create a static host user, an admin will create a `static_host_user` resource:

```yaml
# foo-dev.yaml
kind: static_host_user
metadata:
    name: foo-dev
spec:
    login: foo
    node_labels:
        env: dev
```

Then create it with `tctl`:

```code
$ tctl create foo-dev.yaml
```

The user `foo` will eventually appear on nodes with label `env: dev` once the
`foo-dev` resource makes it through the cache.

To update an existing static host user, an admin will update update `foo-dev.yaml`,
then update the resource in Teleport with `tctl`:

```code
$ tctl create -f foo-dev.yaml
```

### Resource

We will add a new resource to Teleport called `static_host_user`. This resource defines
a single host user, including groups, sudoers entitlements, uid, and gid, as well as labels
to select specific nodes the user should be created on.

```yaml
kind: static_host_user
metadata:
    name: hostuser
spec:
    login: user1
    # groups and sudoers are identical to their role counterparts
    groups: [abc, def]
    sudoers: [
        # ...
    ]
    # same as from user traits
    uid: "1234"
    gid: "5678"
    # same as allow rules in roles
    node_labels:
        # ...
    node_labels_expression: # ...
```

```proto
message StaticHostUser {
    string kind = 1;
    string sub_kind = 2;
    string version = 3;
    teleport.header.v1.Metadata metadata = 4;

    StaticHostUserSpec spec = 5;
}

message StaticHostUserSpec {
    string login = 1;
    repeated string groups = 2;
    repeated string sudoers = 3;
    string uid = 4;
    string gid = 5;

    wrappers.LabelValues node_labels = 6;
    string node_labels_expression = 7;
}

service UsersService {
  rpc GetStaticHostUser(GetStaticHostUserRequest) returns (GetStaticHostUserResponse);
  rpc ListStaticHostUsers(ListStaticHostUsersRequest) returns (ListStaticHostUsersResponse);
  rpc CreateStaticHostUser(CreateStaticHostUserRequest) returns (CreateStaticHostUserResponse);
  rpc UpdateStaticHostUser(UpdateStaticHostUserRequest) returns (UpdateStaticHostUserResponse);
  rpc UpsertStaticHostUser(UpsertStaticHostUserRequest) returns (UpsertStaticHostUserResponse);
  rpc DeleteStaticHostUser(DeleteStaticHostUserRequest) returns (google.protobuf.Empty);
}
```

### Propagation

On startup, nodes will apply all available `static_host_user`s in the cache,
then watch the cache for new and updated users. Nodes will use the labels in the
`static_host_user`s to filter out those that don't apply to them, with the same
logic that currently determines access with roles. Updated `static_host_user`s
override the existing user. When a `static_host_user` is deleted, any host users
created by it are *not* deleted (same behavior as `keep` mode for current host
user creation).

Nodes that disable host user creation (by setting `ssh_service.disable_create_host_user`
to true in their config) will ignore `static_host_user`s entirely.

### Audit events

The `session.start` audit event will be extened to include a flag
indicating whether or not the host user for an SSH session was
created by Teleport (for both static and non-static host users).

### Product usage

The session start PostHog event will be extended to include the
same flag described in [Audit events](#audit-events).

### Security

CRUD operations on `static_host_user`s can be restricted with verbs
in allow/deny rules like any other resource.

We want to minimize the ability of Teleport users to mess with existing host users
via `static_host_user`s. To that end, all host users created from `static_host_user`s
will be in the `teleport-created` group (similar to the `teleport-system` group, which
we currently use to mark users that Teleport should clean up). Teleport will not
delete users not in `teleport-created`, and new users will not override existing users
that are not in `teleport-created`.

### Backward compatibility

Consider nodes that do not support static host users but are connected to an
auth server that does. These nodes will silently ignore static
host users. When these nodes are upgraded to a supporting
version, they will create static host users as normal.

### Test plan

Integration test for:
- nodes create/update nodes in response to `static_host_user` updates from the cache

Manual test for:
- create static host user with `tctl` and verify it's applied to nodes

### Future work

Extend server heartbeats to include static host users. This will allow Teleport
users to spot incorrect propagation of host users due to misconfiguration, nodes
that don't support them, etc.
