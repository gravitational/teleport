---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: implemented
---

# RFD 0171 - Database session playback

## Required Approvals

* Engineering: (@r0mant || @smallinsky) && @greedy52
* Product: @klizhentas || @xinding33

## What

This RFD proposes enhancements to the database session recordings in Teleport by
expanding the current session recordings to include queries/command responses,
enabling complete session playback. Additionally, it integrates playback
functionality with existing tools (`tsh` and Web UI) by converting database
session recordings into a format compatible with the current SSH session player.

## Why

Currently, database session recordings in Teleport capture only the start/end,
query, and some protocol-specific (for example, PostgreSQL prepare statement)
events. However, these recordings do not include the server's responses, which
limits their usefulness for auditing and troubleshooting. By capturing the
queries/commands and their responses, we can provide a more comprehensive view
of database interactions. This enhancement will help organizations improve
security audits, and facilitate debugging and operational reviews.

## Details

The database access recording is already in place for every database protocol,
and is done in the same way as SSH recordings. The main difference is that all
events are also emitted as regular audit events, which means they're accessible
outside the recording files.

In contrast to SSH recordings, database access recordings do not have a 1-1
relation to the user's inputs. This means the recordings might contain
additional queries (executed by the clients) and client-side generated queries.
For example, if the user uses a GUI client, it might execute multiple queries
during the connection setup to grab scheme information. This information will be
present on the recordings even if the user didn’t directly produce it.  Another
example is when the user uses `psql` and performs one of the shortcuts, for
example, `\du`. Only the queries generated/executed by `psql` will be on the
recording, not the `\du` execution.

The start/end and query events are recorded. In addition to those events,
there are also protocol-specific events, for example `PostgresExecute` is
emitted on PostgreSQL when a prepared statement is executed.

The session recordings are available through the `tsh play` command, but they
can only be viewed as JSON:


```json
[
  {
      "ei": 0,
      "event": "db.session.start",
      "code": "TDB00I",
      "success": true,
      ...
  },
  {
      "ei": 1,
      "event": "db.session.postgres.statements.execute",
      "code": "TPG02I",
      "portal_name": "test",
      ...
  },
  {
      "ei": 2,
      "event": "db.session.query",
      "code": "TDB02I",
      "db_query": "SELECT 1;",
      "success": true,
      ...
  },
  {
      "ei": 3,
      "event": "db.session.end",
      "code": "TDB01I",
      ...
  }]
```

### Audit events

To have a complete recording with users input and server response, we'll
include the command result event:

```proto
// NOTE: This message already exists, we are just reusing as it is.
// Copied from: api/proto/teleport/legacy/types/events/events.proto
message Status {
  // Success indicates the success or failure of the operation
  bool Success = 1 [(gogoproto.jsontag) = "success"];

  // Error includes system error message for the failed attempt
  string Error = 2 [(gogoproto.jsontag) = "error,omitempty"];

  // UserMessage is a user-friendly message for successful or unsuccessful auth attempt
  string UserMessage = 3 [(gogoproto.jsontag) = "message,omitempty"];
}

// DatabaseSessionCommandResult represents the result of a user command. It is
// expected that for each user command/query there will be a corresponding
// result.
message DatabaseSessionCommandResult {
  // Metadata is a common event metadata.
  Metadata Metadata = 1;
  // User is a common user event metadata.
  UserMetadata User = 2;
  // SessionMetadata is a common event session metadata.
  SessionMetadata Session = 3;
  // Database contains database related metadata.
  DatabaseMetadata Database = 4;
  // Status of the execution.
  Status status = 5;
  // AffectedRecords represents the number of records that were affected by the
  // user query.
  uint64 AffectedRecords = 6;
}
```

#### Examples

* Audit event generated for a query that returns data (`SELECT id, name FROM events;`):

```json
{
  ...
  "status": {
    "success": true
  },
  "affected_records": 5
}
```

* Query that only returns a success message (`UPDATE events SET ...`):

```json
{
  ...
  "status": {
    "success": true
  },
  "affected_records": 3
}
```

* Query that returns an error (`SELECT err;`):

```json
{
  ...
  "status": {
    "success": false,
    "error": "ERROR: column \"err\" does not exist"
  },
  "affected_records": 0
}
```

### Recording options

Database session recording options will be defined per role. This configuration
will be under the `options.record_session.db` role option. It will support the
following values:
- `on`: Enables session recording.
- `off`: Disables session recording. Audit events are kept unchanged (start,
  end, and query events emitted).

The value will be set to `on` by default, keeping the current behavior.

```yaml
kind: role
version: v5
metadata:
  name: alice
spec:
  allow:
    ...
  options:
    record_session:
      db: on|off
```

### Player

To play the session recording, we will rely on the existing player used by SSH
sessions. This means we don't need to introduce any new player on `tsh` or the
Web UI. We're going to convert database recording events into `SessionPrint`
events, which can be rendered by the players.

This conversion will be agnostic to the database protocol. This will make it
easier to keep the session recording play consistent across protocols. However,
protocol-specific messages will still require special handling, otherwise the
player won't be able to present them.

It will present the session recording events in different text formats:

- Status (result): Show the user message. If none is present, show "SUCCESS" in
  case of success or "ERROR" in case of error.
- The player will define if there is need to display the number of affected
  records. Protocol-specific variations can be added for this. For example, if
  the query was a `SELECT` the player might display it as "Returned X rows"
  after the status.

Example of player visualization:

```code
mydatabase=# SELECT id, name FROM events;
SUCCESS
(3 rows returned)

mydatabase=# INSERT INTO events (name) VALUES ('session.query');
SUCCESS
(1 row inserted)

mydatabase=# SELECT with_error;
ERROR: column "with_error" does not exist (SQLSTATE 42703)

mydatabase=# ALTER SYSTEM SET client_min_messages = 'notice';
SUCCESS

mydatabase=# SELECT greet('hello');
SUCCESS
(1 row)

mydatabase=# call greet_procedure('hello');
SUCCESS

mydatabase=# call error_procedure('hello');
ERROR: procedure error_procedure() does not exist (SQLSTATE 42883)

(session end)
```

#### Legacy session recordings

Older sessions records will not contain the results, however, the player will
still be able to play them. For this, the player will only present the queries
executed, following the same format described earlier.

Example:

```code
mydatabase=# SELECT id, name FROM events;

mydatabase=# INSERT INTO events (name) VALUES ('session.query');

mydatabase=# SELECT with_error;

mydatabase=# ALTER SYSTEM SET client_min_messages = 'notice';

(session end)
```

#### Web UI

Since the database session recording is going to be translated into the same
format as SSH sessions, it can be reproduced on the Web UI without requiring a
new player.

Given that the player (which contains the translator) is executed on the proxy
side, only the `SessionPrint` events are sent to the browser. The Web UI will be
required to verify if the proxy can replay the session based on the database
protocol before showing the “Play” button, avoiding redirecting users to the
player with sessions that won't be able to be reproduced.

### References

Section to consolidate a few references used while making the definitions on the
RFD.

#### Database CLIs rendering examples

```bash
$ sqlcmd
1> INSERT INTO events (name) VALUES ('event');
2> GO
(1 rows affected)

1> SELECT * FROM events
2> GO
id name
-- ----
 1 start
 2 end

(2 rows affected)
1> EXIT

$ psql
psql (16.2 (Homebrew))
SSL connection GCM_SHA256, (protocol: TLSv1.3, cipher: TLS_AES_128_ compression: off)
Type "help" for help.

my-database=# INSERT INTO events (name) VALUES ('end');
INSERT 0 1

my-database=# SELECT * FROM events;
id | name
---+-------
 1 | start 
 2 | end 
(2 rows)

my-database=# exit

$ mysql
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 9
Server version: 8.4.0 MySQL Community Server - GPL

Copyright (c) 2000, 2024, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> INSERT INTO events (name) VALUES ('end');
Query OK, 1 row affected (0.00 sec)

mysql> SELECT * FROM events;
+----+-------+
| id | name  |
+----+-------+
|  1 | start |
|  2 | end   |
+----+-------+
2 rows in set (0.00 sec)

$ redis-cli
127.0.0.1:6379> SET hey 1
OK
127.0.0.1:6379> HSET myhash a 1 b 2 c 3
(integer) 3
127.0.0.1:6379> HGETALL myhash
1) "a"
2) "1"
3) "b"
4) "2"
5) "c"
6) "3"
127.0.0.1:6379> HGET myhash a
"1"
127.0.0.1:6379> GET hey
"1"
127.0.0.1:6379> exit
```
