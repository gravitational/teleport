---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 113 - On-demand database users

## Required approvers

Engineering: @smallinsky
Product: @klizhentas, @xinding33
Security: @reedloden, @jentfoo

## What

Proposes support for on-demand user creation for database access.

## Why

Today, we require all database users to exist in the database server beforehand.
This means that database access users have to either use a predefined set of
database users for all their Teleport users, or build extra automation to
provision database users separately when onboarding new users.

Adding support for on-demand database user provisioning, similar to [automatic
Linux user provisioning](https://github.com/gravitational/teleport/blob/master/rfd/0057-automatic-user-provisioning.md),
will allow database access users to not worry about preconfiguring each
individual user within the database and will simplify user on-/offboarding and
give ability to control permissions via IdP user traits.

## Scope

The RFD will cover self-hosted and RDS versions of PostgreSQL but the same
approach can then be extended to other protocols.

## High-level flow

Let's first define the high-level flow of how on-demand user provisioning
will work:

- Users will designate an existing user in their database as an "admin user"
  which will have permissions to create/delete other users and assign them
  privileges.
- When receiving a connection for the user that should be auto-created,
  Teleport database service will first connect to the database as the admin
  user and create it.
- Once the user completes the session, Teleport database service will connect
  as the admin user again and disable it if it doesn't have any other active
  sessions.

Now let's dive into details.

## Details

### Configuration changes

To support this functionality, Teleport database service needs to have access
to the database as a user that has permissions to create users and assign them
roles.

To avoid asking users to provide static credentials for the admin user,
Teleport will use the same authentication mechanism for it as for any other
user connecting to the database (i.e. X.509 for self-hosted, IAM for RDS, etc).
This means that the admin user will still need to be configured first to
allow Teleport to use it. The permissions that the admin user will need to hold
within the database will be defined below.

Database resource spec for both static config and dynamic resources will be
updated to allow users to specify which admin user Teleport should use for
auto-provisioning:

```yaml
kind: "db"
version: "v3"
metadata:
  name: "example"
spec:
  protocol: "postgres"
  uri: "localhost:5432"
  # The admin_user section is intentionally nested for future extensibility.
  admin_user:
    name: "postgres"
```

For auto-discovered databases, the admin user name will be taken from the
`teleport.dev/db-admin-user` tag.

### Role changes

The role resource will be updated to include additional options/fields:

- `options.create_db_user` will indicate whether Teleport should attempt to
  create a database user for the connecting Teleport user.
- `allow.db_roles` will contain database role names the created user should
  be granted within the database. Will support IdP trait templating.

```yaml
kind: "role"
version: "v5"
metadata:
  name: "example"
spec:
  options:
    create_db_user: true
  allow:
    db_roles: ["reader", "writer", "{{external.db_roles}}"]
```

The created user will have the same username as the Teleport user that's
establishing the connection.

Most commonly used databases (PostgreSQL, MySQL, MongoDB) support the concept
of roles as collection of privileges within the database that can be assigned
to a user so `db_roles` should extend well to other protocols we support too.

### Conditions for creating users

Teleport database service will attempt to auto-create the database user if
both of these are true:

- Teleport user's roleset includes a role that matches the database they're
  connecting to (by `db_labels`) and has `create_db_user: true` option set.
- The database they're connecting to has admin user defined.

### Creating users

PostgreSQL users created by Teleport will be members of `teleport-auto-user`
role. This is similar to Linux user provisioning which adds users to `teleport`
Linux group for tracking/bookkeeping purposes.

The `teleport-auto-user` role will be created by Teleport using admin user and
will not have any privileges:

```sql
create role teleport-auto-user
```

### Disabling users

PostgreSQL users created by Teleport **will not be deleted** by Teleport.

The main reason is dynamic users can have permissions to create database objects
(depending on their database role) and users that own any database objects can't
be deleted until the objects are deleted or their ownership is transferred to
another database user.

PostgreSQL provides commands for deleting objects owned by a user and changing
their owner but it has to be run in each database where user has objects which
would require some non-trivial database introspection. In addition, keeping
original ownership preserves the audit trail.

For these reasons, dynamic users instead will be **disabled instead of deleted**
after each session: stripped of all roles except `teleport-auto-user` and
updated with `nologin` trait so they can't connect.

### Stored procedures

To create and disable users Teleport will use 2 stored procedures:

- `teleport_create_user(username varchar, roles varchar[])`
- `teleport_disable_user(username varchar)`

Usage of stored procedures is chosen for the following reasons:

- Utility statements like create/grant do not support bound parameters so
  using procedure is safer that building queries in the code. See security
  section below for more details.
- Stored procedures are "all or nothing" so if any of the roles are invalid
  during user creation for example, the user won't be created and no extra
  cleanup will be required.
- Stored procedures make it easier to keep some of the control logic in the
  database for avoid races between agents, for example only disabling the user
  if it doesn't have any active sessions.

The procedures' source code is included below.

### Session flow

When receiving a database client connection, Teleport database service will
first connect to the database as the admin user, install both procedures
described above and call `teleport_create_user()` to provision a user and
assign it roles.

If the user already exists in the database, several scenarios are possible:

- The user doesn't have `teleport-auto-user` role. This means the user is not
  managed by Teleport. The connection will be aborted.
- The user has `teleport-auto-user` role and doesn't have active sessions. This
  means user was previously created by Teleport and isn't currently connected.
  User will be stripped of all roles except for `teleport-auto-user` to account
  for permissions potentially left from previous session (e.g. in case database
  service crashed) and reactivated/assigned appropriate roles.
- The user has `teleport-auto-user` role and has an active session. This means
  user was previously created by Teleport and is currently connected. User will
  not be modified and connection will proceed as normal.

After the database session completes, database service will connect again as
the admin user and call `teleport_disable_user()` to disable the user unless
it has other active sessions.

### Database user name

Provisioned database users will have the same usernames as connecting Teleport
users. Some databases impose limits on what a valid user name is, in which case
unsupported characters in the Teleport user name will be replaced.

For PostgreSQL specifically, acceptable username can include special characters
so no special replacement logic will be implemented until it becomes necessary
as any typical Teleport usernames are also valid PostgreSQL usernames, for
example: `alice.bob`, `ali$e`, `alice@example.com`.

### Stored procedures

```sql
create or replace procedure teleport_create_user(username varchar, roles varchar[])
language plpgsql
as $$
declare
    role_ varchar;
begin
    -- If the user already exists and was provisioned by Teleport, reactivate it,
    -- otherwise provision it.
    if exists (select * from pg_auth_members where
                roleid = (select oid from pg_roles where rolname = 'teleport-auto-user') and
                member = (select oid from pg_roles where rolname = username)
    ) then
        -- If the user has active connections, just use it.
        if exists (select usename from pg_stat_activity where usename = username) then
          return;
        end if;
        -- Otherwise reactivate the user, but first strip if of all roles to
        -- account for left-over situations.
        call teleport_disable_user(username);
        -- Utility statements like create/grant do not accept positional arguments
        -- so we have to use dynamic SQL. %I takes care of safe argument escaping.
        -- For RDS we will automatically add "rds_iam" to the roles.
        execute format('alter user %I with login', username);
    else
        execute format('create user %I in role teleport-auto-user', username);
    end if;
    -- Assign all roles to the created/activated user.
    foreach role_ in array roles
    loop
        execute format('grant %I to %I', role_, username);
    end loop;
end;$$;

create or replace procedure teleport_disable_user(username varchar)
language plpgsql
as $$
declare
    role_ varchar;
begin
    -- Only deactivate if the user doesn't have other active sessions.
    if exists (select usename from pg_stat_activity where usename = username) then
        raise exception 'User has active connections';
    else
      -- Revoke all role memberships except teleport-auto-user group.
      for role_ in select a.rolname from pg_roles a where
                    pg_has_role(username, a.oid, 'member') and
                    a.rolname not in (username, 'teleport-auto-user')
      loop
          execute format('revoke %I from %I', role_, username);
      end loop;
      -- Disable ability to login for the user.
      execute format('alter user %I with nologin', username);
    end if;
end;$$;
```

### Locking

Multiple create/disable procedures for the same user can run simultaneously e.g.
if sessions are being opened/established at the same time by user or GUI client.
To avoid races, Teleport database service will use `AcquireSemaphore` to make
sure only 1 procedure runs for a particular user:

https://github.com/gravitational/teleport/blob/v12.1.1/api/client/client.go#L953

For new sessions, the lock will be acquired before executing `teleport_create_user()`
procedure and released after the connection has been established. After session
ends, the lock will be acquired before executing `teleport_disable_user()` and
released after it's completed.

The lock's key will include cluster name and user name so only the connections
for the same user will be serialized.

## Security

### Admin user privileges

To support user creation in PostgreSQL, the admin user needs to have the
following privileges:

- `login` to establish database sessions.
- `createrole` to create/drop roles and grant/revoke role memberships.

```sql
create role teleportAdmin login createrole;
```

For RDS, the admin user will also need to have `rds_iam` role to allow IAM
authentication.

### SQL injections

Applications typically use prepared statements with bound parameters when
constructing queries and let the driver handle parameter validation and escaping.
Bound parameters work in regular DML queries but not in `CREATE`/`DROP`/`GRANT`/`REVOKE`
utility statements which Teleport will be executing.

To protect against SQL injection attacks, Teleport will employ two tactics.

First, the database service will validate that any usernames and role names are
valid PostgreSQL identifiers prior to trying to create them.

https://www.postgresql.org/docs/15/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS:

> SQL identifiers and key words must begin with a letter (a-z, but also letters
> with diacritical marks and non-Latin letters) or an underscore (_). Subsequent
> characters in an identifier or key word can be letters, underscores, digits
> (0-9), or dollar signs ($).

Second, the use of stored procedures allows bound parameters when invoking
them using the database driver. Example using `pgx` which Teleport uses:

```go
// Connect to the database.
conn, _ := pgx.ConnectConfig(ctx, config)

// Install the procedure.
_, _ = conn.Exec(ctx, "create or replace procedure ...")

// Call the procedure to create the user.
_, _ = conn.Exec(ctx, "call teleport_create_user($1, $2)", username, []string{roleA, roleB})
```

### Audit events

Teleport will generate `db.user.created` and `db.user.disabled` audit events
for each dynamic user. New events will include the user name and roles along
with the common audit event fields:

```json
{
    "name": "db.user.created",
    "db_name": "alice",
    "db_roles": ["reader", "writer"],
    "db_protocol": "postgres",
    ...
}
```

In addition, queries performed by the database service for installing stored
procedures and calling them will also be logged in the audit log as regular
`db.session.query` events.
