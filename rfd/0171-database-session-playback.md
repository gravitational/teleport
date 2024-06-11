---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: draft
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
// DatabaseFieldType enum for database data fields data types.
enum DatabaseDataFieldType {
  UNSPECIFIED = 0;
  STRING = 1;
  NUMBER = 2;
  DATE = 3;
  BINARY = 4;
}

// DatabaseDataFieldMetadata contains metadata for database data fields.
message DatabaseDataFieldMetadata {
  // Name is the field name.
  string Name = 1;
  // Type is the field type. Since the values are stored inside the event as
  // strings, this type can convert the row values back into their original
  // type. This information can be used to format the row values.
  DatabaseDataFieldType Type = 2;
}

// DatabaseDataRow represents a single data row, containing a list field values.
message DatabaseDataRow {
  // Values the row field values.
  // Value is the row field values. The values aren not expected to be the
  // direct database type but their string representation, making it possible to
  // display them without parsing. For example, binary data can be encoded into
  // base64.
  repeated string Values = 1;
}

// DatabaseResultData database results data, containing fields metadata and also
// the data itself.
message DatabaseResultData {
  // FieldMetadata list of fields metadata.
  repeated DatabaseDataFieldMetadata Fields = 1;
  // Rows list of the field values.
  repeated DatabaseDataRow Rows = 2;
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

  oneof Result {
    // Status of the query execution. It is used when the command doesn't return
    // any data.
    Status status = 5;
    // Data contains the fields and data of the query execution.
    DatabaseResultData Data = 6;
  }
}
```

#### Examples

* Audit event generated for a query that returns data (`SELET id, name FROM events;`):

```json
{
  ...
  "data": {
    "fields": [
      {"name": "id", "type": "INTEGER"},
      {"name": "name", "type": "STRING"}
    ],
    "rows": [
      ["1", "session.start"],
      ["2", "session.end"]
    ]
  }
}
```

* Query that only returns a success message (`UPDATE events SET ...`):

```json
{
  ...
  "status": {
    "success": true,
    "message": "3 rows affected"
  }
}
```

* Query that returns an error (`SELECT err;`):

```json
{
  ...
  "status": {
    "success": false,
    "error": "ERROR: column \"err\" does not exist"
  }
}
```

### Recording options

Database session recording options will be defined per role. This configuration
will be under the `options.record_session.db` role option. It will support the
following values:
- `on`: Enables session recording, but only queries are recorded. This will
  mimic the current behavior.
- `full`: Enables session recording. Queries and responses are recorded.
- `off`: Disables session recording. Audit events are kept unchanged (start,
  end, and query events emitted).
- `recording_only`: Enables recording, but events will not be emitted, they
  will only be present on recordings. This mode will be the same format as SSH
  sessions, where the commands and their results are only present on the
  recording.

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
      db: on|full|recording_only|off
```

### Player

To play the session recording, we will rely on the existing player used by SSH
sessions. This means we don't need to introduce any new player on `tsh` or the
Web UI. We're going to convert database recording events into `SessionPrint`
events, which can be rendered by the players.

This conversion will be agnostic to the database protocol. This will make it
easier to keep the session recording play consistent across protocols.

It will present the session recording events in different text formats:

- User queries will be presented as received with additional information about
  the database name.
- Data with fields metadata: Format the data into an ASCII table, where each
  field is a column.
- Data without field metadata: Print each row as a single line.
- Status (result) without data: Show the user message. If none is present, show
  "OK" in case of success or "ERROR" in case of error.

Example of player visualization:

```code
mydatabse=# SELECT id, name FROM events;
| id |      name     |
+----+---------------+
|  1 | session.start |
|  2 | session.end   |

mydatabase=# INSERT INTO events (name) VALUES ('session.query');

1 row affected

mydatabase=# SELECT with_error;
ERROR: column "with_error" does not exist

mydatabase=# ALTER SYSTEM SET client_min_messages = 'notice';
OK

(session end)
```

### Performance considerations

Recording queries and responses will increase storage requirements and
potentially impact CPU and memory usage. To reduce these, the following measures
will be taken:

- The recording mode will be similar to `node` (async), where the recording is
  always done on the node side, persisted into the local filesystem, and
  uploaded to the auth server after the session ends.
- Server response events will be generated and recorded in a separate goroutine,
  avoiding delayed message delivery to the clients.
- Record a predefined maximum number of rows per result. If this limit is
  exceeded, the rows will be truncated. In addition to this limit, the record
  will also ensure the Protobuf max message size is not exceeded.

### Security

Depending on the configured recording mode, the proposed database session
recording additions will capture and store data from users' databases. This
introduces considerations around data sensitivity and the potential exposure
of confidential information within the session recording files.

Like SSH session recordings, where users might execute commands revealing
sensitive information, database session recordings can also contain sensitive
data.

Administrators can disable or configure the extent of session recording to
address these concerns. By changing the recording options, organizations can
tailor the recording behavior to their security policies and compliance
requirements. Additionally, system administrators can configure stricter auditor
roles to restrict access to sensitive information within the session recordings,
ensuring only authorized personnel can view them.

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
