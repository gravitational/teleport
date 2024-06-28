---
author: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 175 - Static Host Users

## Required Approvers

## What

Teleport nodes will be able to create host users statically, i.e. independently
of a Teleport user creating one when SSHing with the current host user creation.

## Why

users can be provisioned w/o them needing to log in beforehand

## Details

### Resource

Add a new resource to Teleport called `static_host_user`. This resource defines
a single Unix user, including groups, sudoers, uid, and gid, as well as labels
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

### UX

Admins will create a `static_host_user` resource:

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

### Security

We want to minimize the ability of Teleport users to mess with existing Unix users
via `static_host_user`s. To that end, all Unix users created from `static_host_user`s
will be in the `teleport-created` group (similar to the `teleport-system` group, which
we currently use to mark users that Teleport should clean up). Teleport will not
delete users without `teleport-created`, and new users will not override existing users
that are not in `teleport-created`.
