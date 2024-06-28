---
author: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 175 - Static Host Users

## Required Approvers

TODO

## What

Teleport nodes will be able to create host users statically, i.e. independently
of a Teleport user creating one when SSHing with the current host user creation.

## Why

TODO

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

To remove the resource and delete all host users associated with it, run:

```code
$ tctl rm host_user/foo-dev
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

### Propagation

On startup, nodes will apply all available `static_host_user`s in the cache,
then watch the cache for new and updated users. Nodes will use the labels in the
`static_host_user`s to filter out those that don't apply to them, with the same
logic that currently determines access with roles. Updated `static_host_user`s
override the existing user. Delete events from the cache will signal the node
to delete the created user.

To facilitate deletion, `static_host_user`s will be keyed under their login in
the backend, i.e. `hostUsers/<login>/<resource-name>`.

Nodes that disable host user creation (by setting `ssh_service.disable_create_host_user`
to true in their config) will ignore `static_host_user`s entirely.

### Security

We want to minimize the ability of Teleport users to mess with existing host users
via `static_host_user`s. To that end, all host users created from `static_host_user`s
will be in the `teleport-created` group (similar to the `teleport-system` group, which
we currently use to mark users that Teleport should clean up). Teleport will not
delete users not in `teleport-created`, and new users will not override existing users
that are not in `teleport-created`.

### Backward compatibility

Consider nodes that do not support static host users but are connected to an
auth server that does. These nodes will silently ignore static
host users.

### Future work

Extend server heartbeats to include static host users. This will allow Teleport users to spot incorrect propagation of host users
due to misconfiguration, nodes that don't support them, etc.
