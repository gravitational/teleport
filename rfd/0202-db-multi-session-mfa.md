---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

RFD 202 - Database Multi-session MFA

Required Approvers:
- Engineering: @r0mant && @codingllama

## What

Allows executing multiple database connections with a single MFA tap.

## Why

Teleport today supports per-session MFA for enhanced security. However, when a
user needs to run queries on multiple database hosts simultaneously, they have
to perform a tap for every connection.

A relaxed mode of per-session MFA will be introduced so that a MFA challenge is
still required for connecting to target databases but the MFA response can be
reused for a short period of time without the need to prompt the user again in
that period.

In addition to expanding MFA functionalities, a new `tsh` command will be
introduced to assist executing multiple database connections in a single command.

## Details

### UX
I would like to relax existing per-session MFA requirement and allow
multi-session MFA. The Teleport role that grants database access can be updated
as below:
```diff
kind: role
version: v7
metadata:
  name: example-role-with-mfa
spec:
  options:
    require_session_mfa: true
+    # Defaults to 'per-session'. Valid values are:
+    # - 'per-session': MFA is required for every session.
+    # - 'multi-session': Allows reuse of a MFA for multiple sessions. Currently only
+    #    supported for `tsh db exec` command with WebAuthn as the second factor.
+    requie_session_mfa_mode: "multi-session"
  allow:
    db_labels:
      'env': 'dev'
    db_users: ["mysql"]
```

I would like to execute a query on multiple databases:
```bash
$ tsh db exec --db-user mysql --exec-query "select @@hostname" mysql-db1 mysql-db2
MFA is required to execute database sessions
Tap any security key
Detected security key tap

[mysql-db1] @@hostname
[mysql-db1] mysql-db1-hostname
[mysql-db2] @@hostname
[mysql-db2] mysql-db2-hostname
```

I would like to search databases by labels, run the sql scripts in parallel, and
record the outputs to a directory:
```bash
$ tsh db exec --search-by-labels env=dev --db-user mysql --exec-query "source my_script.sql" --output-dir exec-logs --max-connections 3
Found 5 databases:
- mysql-db1
- mysql-db2
- mysql-db3
- mysql-db4
- mysql-db5

Tip: use --skip-confirm to skip this confirmation.
Do you want to continue? (Press any key to proceed or Ctrl+C to exit): <enter>

MFA is required to execute database sessions
Tap any security key
Detected security key tap

Executing command for 'mysql-db1'
Executing command for 'mysql-db2'
Executing command for 'mysql-db3'
Executing command for 'mysql-db4'
Executing command for 'mysql-db5'

$ ls exec-logs/
mysql-db1.output mysql-db2.output mysql-db3.output mysql-db4.output mysql-db5.output
```

### Multi-session MFA

TODO(greedy52) add a diagram.

A new role option is added to preserve existing behavior if not set:
```diff
kind: role
version: v7
spec:
  options:
    require_session_mfa: true
+    # Defaults to 'per-session'. Valid values are:
+    # - 'per-session': MFA is required for every session.
+    # - 'multi-session': Allows reuse of a MFA for multiple sessions. Currently only
+    #    supported for `tsh db exec` command with WebAuthn as the second factor.
+    requie_session_mfa_mode: "multi-session"
```

The multi-session MFA extends [RFD 155 Scoped Webauthn
Credentials](https://github.com/gravitational/teleport/blob/master/rfd/0155-scoped-webauthn-credentials.md)
with a new scope for executing database sessions:
```diff
// webauthn.proto
enum ChallengeScope {
...
    // Used for 'tsh db exec' and allows reuse. 
    SCOPE_DATABASE_MULTI_SESSION = 8;
}
```

Similar to `SCOPE_ADMIN_ACTION`, the new scope will allow reuse of the MFA
session data until it expires (5 minutes for WebAuthn).

The MFA response will be checked upon auth call of `GenerateUserCerts` where
user requests a TLS user cert with database route. New logic is added to
`GenerateUserCerts` where the new scope with reuse is allowed only if the role
set matching the requested database has `roleset.option.requie_session_mfa_mode`
option set to `multi-session`.

If MFA response is validated with existing non-reusable `SCOPE_SESSION`, the
action should be allowed regardless of `roleset.option.requie_session_mfa_mode`.

Here is a quick matrix:

| `session_mfa_mode` | MFA response scope | Access |
| ------------------ | ------------------ | ------ |
| `multi-session`    | `SCOPE_SESSION`    | allow  |
| `multi-session`    | `SCOPE_DATABASE_MULTI_SESSION` | allow  |
| `per-session`      | `SCOPE_SESSION`    | allow  |
| `per-session`      | `SCOPE_DATABASE_MULTI_SESSION` | denied  |

### The `tsh db exec` command

General flow of the command:
- Fetch databases (either specified directly or through search).
- Fetch roles and use access checker to pre-determine MFA requirement.
- For each database
  - Prompt MFA if necessary
  - Starts a local proxy in tunnel mode for this database
  - Craft a command and `os.exec`.

For MVP, only PostgreSQL and MySQL databases will be supported.

#### `tsh db exec` search
`tsh db exec` supports searching database by specifying one the following flags:
- `--search`: List of comma separated search keywords or phrases enclosed in quotations, e.g. `--search=foo,bar`
- `--search-by-labels`: List of comma separated labels to filter by labels, e.g. `key1=value1,key2=value2`
- `--search-by-query`: Query by predicate language enclosed in single quotes.

The command presents the search results then asks user to confirm before
proceeding. `--skip-confirm` can be used to skip the confirmation.

### Security

TODO

### Possible enhancements
#### `tsh db exec --exec-config`
To support a config file which allows specifying different flags like
`--db-user`, `--db-name`, `--exec-query` per target database.

#### `tsh db exec --exec-command`
To support custom command template like:
```
$ tsh db exec --exec-command "bash -c './myscript {{.DB_SERVICE}} {{.DB_LOCAL_PORT}}'
```
