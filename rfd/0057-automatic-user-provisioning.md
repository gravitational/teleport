---
authors: Alex McGrath (alex.mcgrath@goteleport.com)
state: implemented (v10.0.0)
---

# RFD 57 - Automatic user and sudoers provisioning

## What

Automatically create non-existing users and optionally add them to
`sudoers` on Teleport nodes. Users will be removed after all sessions
have logged out.

## Why

Currently, when logging into an SSH node, the user must be
pre-created. Adding automatic user and `sudoer` provisioning would
make it so that any Teleport user would be able to login and have the
account created automatically without manual intervention.

## Details

The following are required for this feature:

- Ability to automatically provision a Linux user if it's not present
  on the node.
- Ability to automatically provision a Linux group if it's not present
  on the node.
- Ability to add the provisioned user to existing Linux groups defined
  in the user traits/role.
- Ability to add the provisioned user to sudoers.
- Clean up the provisioned user / sudoers changes upon logout (being
  careful not to remove pre-existing users).

### Config/Role Changes

Several new fields will need to be added to to the role `options` and
`allow` sections:

```yaml
kind: role
version: v5
metadata:
  name: example
spec:
  options:
    # Controls whether this role supports auto provisioning of users.
    create_host_user: true
  allow:
    # New field listing Linux groups to assign a provisioned user to.
    # Should support user and identity provider traits like other fields (e.g. "logins")
    host_groups: [ubuntu, "{{internal.groups}}", "{{external.xxx}}"]
    # host_sudoers is a list of entries to be included in a users sudoers file
    host_sudoers: ["{{internal.logins}} ALL=(ALL) ALL", ...]
```

An individual `ssh_service` can be configured disable auto user
creation with the below config:

```yaml
ssh_service:
    # when disabled, takes precedence over the role setting
    disable_create_host_user: true
```

### User creation

In order to create users `useradd` will be executed from teleport
after a user has tried to access a Teleport SSH node.

#### User Groups

When a user is created they will be added to the specified groups from
the `host_groups` field in the role. In addition the user will be
added to a special `teleport-system` group which can be used to
indicate that the user was created by teleport and that its safe for
it to be deleted. The groups will be created via `groupadd` at startup
if they do not already exist and users will be added to groups via
`usermod -aG <list of groups> <username>`

#### Valid user/group names

The set of valid names that are valid on Linux varies between distros
and are generally more restrictive than the allowed usernames in
Teleport. This will require that names containing invalid characters
have those characters removed/replaced. Information on the valid
characters between Linux distros is available [here](https://systemd.io/USER_NAMES/).
The common core of valid characters is `^[a-z][a-z0-9-]{0,30}$`.

#### Adding and removing users from sudoers

Each user with entries in `host_sudoers` will have a file created in
`/etc/sudoers.d`, with one entry per line.

If a user is in multiple rules that specify `host_sudoers` they will
be all be concatenated together.

##### sudoers file syntax validation

If a system has `visudo` present, validation could be performed by
executing `visudo -c -f path/to/sudoersfile`, where if it fails to
validate, the user fails to have the shell start and the error is
reported.

##### sudoers security considerations

In order to stop users from being able to edit the sudoers file a
command allow list must be used, as or equivalent to below:

```
${USERNAME} ALL = (${USER TO RUN AS}) NOPASSWD: /bin/cmd1 args, /bin/cmd2 args
```

Should a user be given `root` access to all commands, they will be
able to modify any file, including sudoers files.


### User and group deletion

After all of a users sessions are logged out the created user and any
`sudoers` files that were created for that user will be deleted if
that user is also a member of the `teleport-system` group.

Users can not be deleted while they have running processes so each
time a session ends, an attempt to delete the user can happen, if it
succeeds the sudoers file can also be removed.

If it does not succeed a cleanup process will run every 5 minutes, that
will attempt to delete users if they no longer have running processes.
This clean up process will also ensure that users with running
sessions during a restart will be cleaned up appropriately.

Groups will not be cleaned up and will be created once and be reused
this is to avoid files created with specified groups will remain
accessible between sessions to users in those groups.


### Multiple matching roles

Automatic user provisioning will require that all roles matching a
node via `labels` have `create_host_user=true`


## Update: allow host users to not be deleted after session end

This will deprecate the current `create_host_user` option and replace
it with an option called `create_host_user_mode`. The new option will
have 3 possible settings:
- `off`: disables host user creation
- `drop`: deletes users after the session ends (current behaviour)
- `keep`: leaves host users after the session ends.

If the deprecated `create_host_user` option is specified and
`create_host_user_mode` is not, it will default to using `drop` when
it is true, and `off` when it is false.

If `create_host_user_mode` is set, its setting will always be used
instead of the `create_host_user` setting.

If multiple roles matching a node specify the `create_host_user_mode`
option with both `drop` and `keep`, teleport will default to `keep`

Once `create_host_user` is to be removed, a role migration will be
triggered and any roles including `create_host_user` will be migrated
to use `create_host_user_mode` setting it to `drop` if
`create_host_user` was set.

### Examples

#### Behaviour using the deprecated option will remain the same as it is currently:
```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

will be equivalent to:

```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users, drop them at session end.
    create_host_user_mode: drop
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

#### User will not be deleted at session end
```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users and for them to remain after session ends.
    create_host_user_mode: keep
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

#### Multiple roles specify `create_host_user_mode`:

Multiple roles specify `create_host_user_mode`, teleport will default to `keep`

```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users and for them to remain after session ends.
    create_host_user_mode: keep
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups-other
spec:
  options:
    # allow auto provisioning of users, drop them at session end.
    create_host_user_mode: drop
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

## Update: allow the UID and GID of the created user to be specified

This will require adding new traits to users -- `teleport.dev/uid` and
`teleport.dev/gid`. These will be settable manually or automatically
via an SSO provider attributes if one is setup.

Creating a user with a specific GID requires that a group with that
GID already exists, if it does not yet exist for the specified GID, a
group with that GID will be created with the same name as the user
logging in.

### Example of setting the uid/gid

Role configuration remains the same:
```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users.
    create_host_user_mode: drop
  allow:
    logins: [ "{{internal.username}}" ]
```

```yaml
kind: user
metadata:
  name: alex.mcgrath@goteleport.com
spec:
  created_by:
    connector:
      id: okta
      identity: user@okta.com
      type: saml
  roles:
  - editor
  - access
  - auditor
  saml_identities:
  - connector_id: okta
    username: ...
  traits:
    # new traits included will be used when specifying --gid and --uid in useradd
    teleport.dev/gid:
    - "1239"
    teleport.dev/uid:
    - "1239"
```

When set like this, the user created upon login will have the `--gid`
and `--uid` options specified when calling `useradd`

## UX Examples

### Teleport admin wants each user to have a dedicated host user defined by their Okta attributes
```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
  allow:
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

### Teleport admin wants to define which Linux groups each auto-created user will be added to

```yaml
kind: role
version: v5
metadata:
  name: auto-user-groups
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
  allow:
    # List of each group the user will be added to
    host_groups: [ubuntu, docker, ...]
    # username from external okta attribute
    logins: [ "{{external.username}}" ]
```

### Teleport admin wants to make each auto-created user a sudoer

```yaml
kind: role
version: v5
metadata:
  name: users-as-sudoers
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
  allow:
    # add users to the wheel group
    host_groups: [wheel]
    # make it so users in the wheel group will be able to execute sudoers commands without a password
    host_sudoers: ["%wheel ALL=(ALL) NOPASSWD: ALL"]
```

### Teleport admin wants to define particular commands user will be able to run as root
```yaml
kind: role
version: v5
metadata:
  name: specify-commands-as-sudoers
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
  allow:
    # make it so this specific user can execute `systemctl restart nginx.service `
    host_sudoers: ["{{internal.logins}} ALL = (root) NOPASSWD: /usr/bin/systemctl restart nginx.service"]
```

### Teleport admin wants to prohibit some nodes from auto-creating users

Include the below config for the Teleport node that should not allow
automatic user creation:

```yaml
ssh_service:
  enabled: "yes"
  # stops a specific node from auto-creating users
  disable_create_host_user: true
```

Nodes where `diable_create_host_user` is `false` will still be able to
have users be automatically created.

### Teleport user has multiple roles but not all of them enable `create_host_user`

In the situation where a user has roles as below, the user would not
be able to make use of automatically provisioning users as both roles
do not enable `create_host_user`.

```yaml
kind: role
version: v5
metadata:
  name: allow-access-and-auto-create
spec:
  options:
    # allow auto provisioning of users.
    create_host_user: true
    node_labels:
      - 'env': 'example'
```

```yaml
kind: role
version: v5
metadata:
  name: specify-commands-as-sudoers
spec:
  options:
    node_labels:
      - 'env': 'example'
```

## Update: tag `keep` users and reconcile groups for existing users

This will add a new teleport system group named `teleport-keep` which
will be assigned to host users created with `create_host_user_mode: keep`.
Adding a new group will make it possible to differentiate between
`insecure-drop` users, `keep` users, and users not managed by Teleport.
This makes it possible to apply any group changes made against the role to
previously created `keep` users without risk of modifying unmanaged users.

This requires a migration of existing `keep` users created with prior
versions of Teleport. In order to make this easier, we can support
automatic migration of users on login by assigning the `teleport-keep`
group directly in the role.

```yaml
kind: role
version: v5
spec:
  options:
    host_groups: [teleport-keep ubuntu ec2-user]
```

The `teleport-system` group, used to identify `insecure-drop` users, will
_not_ be directly assignable as this would effectively flag a host user for
deletion.
