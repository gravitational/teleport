---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

# RFD 152 - Database Automatic User Provisioning for MongoDB

## Required approvers

Engineering: @r0mant || @smallinsky
Product: @klizhentas || @xinding33
Security: @reedloden || @jentfoo

## What

This RFD discusses on how to expand Database Automatic Provisioning feature for
MongoDB.

## Why

Automatic User Provisioning has been implemented for several SQL databases,
including PostgreSQL and MySQL, with the basic design described in [RFD
113](https://github.com/gravitational/teleport/blob/master/rfd/0113-automatic-database-users.md).

Adding support for database user provisioning to MongoDB presents unique
challenges due to differences in architecture compared to traditional SQL
databases.

This RFD aims to identify the challenges and provide solutions to address them.

Since the differences in architecture between MongoDB Atlas and self-hosted
MongoDB are significant, they will be discussed separately in this RFD.

## MongoDB Atlas Details

Automatic User Provisioning will NOT be supported for MongoDB Atlas with the
reasons discussed below. This RFD should be updated if better solutions are
found in future iterations.

[Database
Users](https://www.mongodb.com/docs/atlas/security-add-mongodb-users/) and
[Custom Database
Roles](https://www.mongodb.com/docs/atlas/security-add-mongodb-roles/) for
MongoDB Atlas are managed at a Atlas project level.

As a consequence, database users and roles are NOT modifiable through
in-database connections. Instead, one can authenticate with Atlas using the
Atlas SDK and use APIs to manage these database users and roles. Multiple
deployment jobs will be created to update the MongoDB clusters in this project,
upon successful APIs calls.

In my personal testing on an Atlas project with a single MongoDB cluster, it
takes 10~20 seconds for the deployment job to refresh the database user in the
target MongoDB cluster.

With the current design of Automatic User Provisioning, the database user must
be updated with new role assignments for each new connection then the roles
should be revoked once the connection is done. However, waiting for 10+ seconds
for provisioning the database user each connection will result in a very bad
user experience (also client may just time out).

## Self-hosted MongoDB Details

The overall flow and logic will follow the previous [RFD
113](https://github.com/gravitational/teleport/blob/master/rfd/0113-automatic-database-users.md).
Differences will be outlined in the sections below.

### The admin user connection

The Database Service will connect as an admin user in order to manage database
users. The admin user requires a role on `admin` database with the following
privileges:
```json
{
  "createRole": "teleport-admin-role",
  "privileges": [
    { "resource": { "cluster": true }, "actions": [ "inprog" ] },
    { "resource": { "db": "", "collection": "" }, "actions": [ "grantRole", "revokeRole" ] }, 
    { "resource": { "db": "$external", "collection": "" }, "actions": [ "createUser", "updateUser", "dropUser", "viewUser", "setAuthenticationRestriction", "changeCustomData"] }
  ],
  "roles": []
}
```
Where:
- `inprog` action is required to run `currentOp` for searching active
  connections.
- `grantRole` and `revokeRole` actions are required to manage roles on all
  databases.
- User related actions are limited on database `$external` database as X.509
  users only exist on `$external`.

The admin user must be created on `$external` database with X.509 authentication:
```json
{
    "createUser": "CN=teleport-admin",
    "roles": [ {"role": "teleport-admin-role", "db": "admin"} ]
}
```

As the implementation of other databases, the name of admin user is
defined in the database spec `admin_user.name`.

However, there is NO concept of "Stored Procedures" in MongoDB. The user
provisioning logic will be carried through multiple `runCommand` calls
implemented in Go. Multiple parallel database sessions will not race thanks to
[semaphore
locking](https://github.com/gravitational/teleport/blob/master/rfd/0113-automatic-database-users.md#locking).

### Roles

Unlike MySQL or PostgreSQL where roles are scoped to the entire database
instance/cluster, a role in MongoDB is scoped to a specific database.

For example, a custom role `myCustomRole` can be created on database `db1` with
specific privileges on `db1`, and another role `myCustomRole` can be created on
database `db2` with specific privileges on `db2`. They will be considered two
different roles.

Most built-in roles are available on all databases, while some built-in roles
like `readAnyDatabase` can only be applied on the `admin` database.

Therefore, when specifying database roles to assign for the user, it must be in
the format of `<role-name>@<db-name>` to fully identify the role:

```yaml
kind: "role"
version: "v6"
metadata:
  name: "example"
spec:
  options:
    create_db_user_mode: keep
  allow:
    db_names:
    - "db1"
    - "db2"
    - "db3"
    db_roles: 
    - "readAnyDatabase@admin"
    - "readWrite@db2"
    - "myCustomRole@db3"
```

Teleport assigns the roles specified in `db_roles` to the auto-provisioned user
and the MongoDB cluster restricts in-database access based on the assigned
roles. On top of that, Teleport enforces that the user can only access
databases listed in `db_names`.

For example, in the above sample role, even though the user is assigned role
`readAnyDatabase@admin`, Teleport will block access to databases not in the
`db_names`.

Of course, the user has the option to use `*` for `db_names` to solely rely on
MongoDB's role management to restrict access.

### User accounts

New users with name `CN=<teleport-username>` will be created on `$external`
database to use X.509 authentication.

All auto-provisioned users will have the following `customData` to indicate the
user is managed by Teleport:
```json
{
  "createUser": "CN=<teleport-username>",
  "customData": {
    "teleport-auto-user": true
  },
  "roles": [
    { "role": "read", "db": "db1" }
  ]
}
```

MongoDB admins can easily find all Teleport-managed users by running this command
on `$external`:
```json
{ "usersInfo": 1, "filter": { "customData.teleport-auto-user": true } }
```

There is no built-in way to lock an user account in MongoDB, for deactivation
purpose. However, MongoDB has builtin
[`authenticationRestrictions`](https://www.mongodb.com/docs/manual/reference/method/db.createUser/#authentication-restrictions)
that restricts logins by `clientSource` or `serverAddress`, which is always
checked when a database user is being authenticated.

For example, the following `authenticationRestrictions` can be
applied to the user account, in addition to stripping the roles:

```json
{
  "updateUser": "CN=<teleport-username>",
  "roles": [],
  "authenticationRestrictions": [
    { "clientSource": ["0.0.0.0"] }
  ]
}
```

Limiting the `clientSource` effectively locks out the user from logging in.

When re-activating the user, `clientSource` will be set to `["0.0.0.0/0"]`.

Also note that `customData` is not modified during `updateUser` commands to
preserve any `customData` added by the users.

### Finding active connections

Command `currentOp` is used to find active connections for a specific user:
```json
{
  "currentOp": true,
  "$ownOps": false,
  "$all": true,
  "effectiveUsers": {
    "$elemMatch": {
      "user": "CN=<teleport-username>",
      "db": "$external"
    }
  }
}
```

## UX

Database roles must be specified in format of `<role-name>@<db-name>`. See
"Roles" section above for more details.

## Security

### Admin user privileges
The admin user requires the following privileges:
```json
{
  "privileges": [
    { "resource": { "cluster": true }, "actions": [ "inprog" ] },
    { "resource": { "db": "", "collection": "" }, "actions": [ "grantRole", "revokeRole" ] }, 
    { "resource": { "db": "$external", "collection": "" }, "actions": [ "createUser", "updateUser", "dropUser", "viewUser", "setAuthenticationRestriction", "changeCustomData"] }
  ]
}
```

Note that there are implications of allowing `grantRole` on all databases: the
admin user can technically assign any privileges to itself.

We should document this fact in our official documentation guide, and encourage
users to limit the `grantRole` to specific databases, when possible. For
example, if only roles on `db1` and `db2` will be assigned to auto-provisioned
users, the privileges can be limited to:
```
{
  "privileges": [
    { "resource": { "cluster": true }, "actions": [ "inprog" ] },
    { "resource": { "db": "", "collection": "" }, "actions": [ "revokeRole"] }, 
    { "resource": { "db": "db1", "collection": "" }, "actions": [ "grantRole"] }, 
    { "resource": { "db": "db2", "collection": "" }, "actions": [ "grantRole"] }, 
    { "resource": { "db": "$external", "collection": "" }, "actions": [ "createUser", "updateUser", "dropUser", "viewUser", "setAuthenticationRestriction", "changeCustomData"] }
  ]
}
```

The admin user still requires `revokeRoles` on all databases in order to remove
roles during the `updateUser` call (see
[required-access](https://www.mongodb.com/docs/manual/reference/method/db.updateUser/#required-access)).

### Locking database user when deactivated

Auth restrictions `clientSource: ["0.0.0.0"]` is used to lock an user account
when deactivated.

## Performance

### Concurrent connections

As stated in [Issue
#10950](https://github.com/gravitational/teleport/issues/10950), MongoDB
clients usually spawn multiple connections to the server resulting multiple
parallel database sessions on the Database Service. The number of connections
can be limited using a smaller `maxPoolSize` (default 100) in the connection
string, but Teleport does not have full control on this as it's specified from
the MongoDB clients (e.g GUI clients via `tsh proxy kube`). And even when the
`maxPoolSize` is set to 1, it's observed that `mongosh` will still keep three
connections open at the same time.

To speed things up, the admin user connection will be kept open and reused for
up to a minute, per MongoDB database per Admin user per Database Service.

### Minimizing number of `runCommand` calls

Since there is no stored procedures, the Database Service has to make multiple
`runCommand` calls to setup the database session.

To minimize number of roundtrips:
- `teleport-auto-user` is set as `customData` of an user. This avoids the
  attempt to create the `teleport-auto-user` role at the beginning of each
  session.
- Use a single `getUser` command to check:
    1. Whether the database user exists.
    1. Whether the database user is managed by Teleport.
    1. What roles currently assigned to the database user.
- Use a single `updateUser` command to update both `roles` and
  `authRestrictions`. The downside using `updateUser` to update roles is the
  admin user must have `revokeRole` privilege on all databases, whereas
  `revokeRolesFromUser` only requires `revokeRole` privilege on those specific
  databases.
