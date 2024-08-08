---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: draft
---

# RFD 0181 - PostgreSQL database access through Web UI

## Required Approvals

Engineering: (@r0mant || @smallinsky) && @greedy52

Product: @klizhentas || @xinding33

## What

This RFD proposes a feature that allows users to access PostgreSQL databases
through the Web UI as an alternative to using the CLI. The feature includes an
interactive shell for simplified query execution and database management.

## Why

While other protocols, such as SSH and Kubernetes Pod exec, already offer Web
UI access alongside CLI options, PostgreSQL users have been restricted to
terminal-based management. Providing a way to access databases through Web UI
will bring an alternative to CLI usage and cover scenarios where users don't
have access to a terminal or don't want to swap from Web UI to the terminal.

## Details

### UX

#### User story: Alice access database using Teleport for the first time.

Alice is new to Teleport, but she's experienced with PostgreSQL and has spent
a considerable time using `psql`, the PostgreSQL CLI.

Before her access, a system administrator had already enrolled a PostgreSQL
instance and created a set of users the Teleport users could use. Those
database users and the existent databases of the instance are listed on the
role assigned to Alice.

She first logs into Teleport's Web UI and then searches for the desired database
on the resources list. After locating it, she clicks on the "Connect" button.

![PostgreSQL instance resource card with connect button](assets/0181-connect-pg.png)

After clicking, a modal window with connection information is presented. In that
window, she needs to select which database and database user she'll be using.
Teleport already fills this information based on her permissions, so she doesn't
need to find this information somewhere else or ask someone. Also, this will
prevent her from inputting the information incorrectly and being unable to
connect.

![PostgreSQL connect modal](assets/0181-connect-dialog.png)

After selecting the required information, she's redirected to a new tab
containing an interactive shell, similar to `psql`, where she can type
her queries.

![PostgreSQL interactive shell](assets/0181-pg-shell.png)

After interacting with the database, she closes the tab, and her database
session ends.

##### Auto-user provisioning enabled

This is the same scenario, but the PostgreSQL instance was configured with user
provisioning enabled. This change implies which information Alice sees on the
connect modal. She doesn’t need to select the database user, as it will default
to her username. The select is then disabled, and a new now select will be
presented where she can select database roles attached to their user.

![PostgreSQL connect modal with database roles](assets/0181-connect-dialog-roles.png)

#### User story: Bob access database with per-session MFA enabled.

Bob is familiar with Teleport web UI and is experienced with PostgreSQL,
spending considerable time using `psql`, the PostgreSQL CLI.

The system administrators configured this Teleport cluster to use MFA per
session (`require_session_mfa: true`). They also have granted Bob permission to
connect to multiple PostgreSQL databases. The role he'll be using has a short
session TTL.

He logs into Teleport web UI, searches for the desired database, and clicks
"Connect". He’s presented with a pre-filled form containing his connection
information: database name, database roles, and database user. He confirms the
values and clicks "Connect" to start a new session. He's then presented with a
tab browser tab with the PostgreSQL interactive shell.

In the new tab, he's prompted with the MFA modal (the same as SSH
sessions). After completing the authentication, he starts performing multiple
queries using the interactive shell, but after some time, he exceeds his session
TTL, the connection is dropped, and he is redirected to the login page. His
database session is terminated.

#### PostgreSQL interactive terminal

##### Banner

At the beginning of each session, we'll include an informative banner that will consist of the following:
- Information about the interactive shell name and its version.
- Connected PostgreSQL version (this mimics the `psql`).
- Parameters used to connect to the instance, like the database and username.
- Short text describing how to get the supported commands and information through the help command (more details in the next section).

```shell
Teleport PostgreSQL interactive shell (v16.2.0-dev)
Connected to "pg" instance (16.2) as "alice" user (with roles "read-write").
Type "help" or \? for help.
```

##### Supported commands

In addition to executing queries, the interactive shell will implement some
backlash (\\) commands from `psql`. Users can fetch the list of supported
commands by calling the help command (`\?` or `help`):

```shell
postgres=> \?
General:
  \q          Terminate the session.
  \teleport   Show Teleport interactive shell information, such as execution limitations.

Informational:
  \d              List tables, views, and sequences.
  \d NAME         Describe table, view, sequence, or index.
  \dt [PATTERN]   List tables.

Connection/Session:
  \session   Display information about the current session, like user, roles, and database instance.
```

##### Limitations/Unpported commands

Teleport PostgreSQL interactive shell will not be a complete feature pair with
`psql`. Those limitations will be due to security measures or the shell's
simplicity. Given those limitations, messages will be shown to the users,
displaying a description and direction on executing the desired command
(if applicable).

Unsupported backslash (\\) commands from psql will only display a failure
message:

```shell
postgres=> \set a "hello"
Invalid command \set.
Try "help" or "\?" for the list of supported commands.

postgres=>
```

Other limitations, such as query size limit, will display a more complete message:

```shell
postgres=> INSERT INTO ... # Long query.
ERROR: Unable to execute query. Max query size limit (2048 characters) exceeded.
For long queries, execute it using `tsh db` commands.

postgres=>
```

### Implementation

#### Web UI

All the new interactions will be done on the Web UI. First, the database
resource returned by the API will include new fields:

```go
// Defined at lib/web/ui/server.go
type Database struct {
  // omitted

  // DatabaseRoles is the list of allowed database roles that the user can
  // select.
  DatabaseRoles []string `json:"database_roles"`
  // SupportsWebUISession is a flag to indicate the database supports Web UI
  // sessions.
  SupportsWebUISession bool `json:"support_webui_session"`
}
```

These new fields are used to:
- Change the "Connect" button behavior to present a connect modal when web UI
  is supported.
- Present the available database roles when applicable.

In addition, a new page will be added to handle the database session
console/terminal (`/web/cluster/:clusterId/console/db/:sid`). This page will be
very similar to the SSH session console, except for the file transfer/upload
icons bar, which will be removed.

#### Proxy

A new web handler will be added to handle the interactive session socket
connection: `/webapi/sites/:site/databases/term`. We cannot provide the session
parameters since web socket connections are started as GET requests. Instead,
when the web socket connection is established, the front end will forward a
message containing the session parameters. This is the same flow used by
Kubernetes exec sessions.

The following struct represents the session request:

```go
// DatabaseSessionRequest describes a request to create a web-based terminal
// database session.
type DatabaseSessionRequest struct {
  // DatabaseResourceID is the database resource ID the user will be connected.
  DatabaseResourceID string `json:"db_resource_id"`
  // DatabaseName is the database name the session will use.
  DatabaseName string `json:"db_name"`
  // DatabaseUser is the database user used on the session.
  DatabaseUser string `json:"db_user"`
  // DatabaseRoles are ratabase roles that will be attached to the user when connecting to the database.
  DatabaseRoles []string `json:"db_roles"`
}
```

After the connection is established and the first message is received, the proxy
will generate a user certificate containing `RouteToDatabase` using one of
`PerformMFACeremony` or `GenerateUserCerts`.

Note: The MFA flows will follow the existing functions (used by Kubernetes
execs, desktops, and SSH sessions).

Once the certificates are generated, the proxy will connect to the target
database server. This flow resembles what the database proxy does: Choose a
database server among a list of available servers to forward the connection.
After the connection is established, a new instances of the Teleport's
PostgreSQL REPL is created, and the session starts.

#### PostgreSQL REPL

The proxy will include a PostgreSQL Read-Eval-Print Loop (REPL) that interacts
with the databases served by database servers. The REPL will read the user input
per line and decide whether or not to execute the command (as the REPL will
support multi-line commands). The rule follows:
- If the line starts with a backslash command, execute it.
- Send the line to the target database if it terminates in a semicolon (`;`).

If the line read doesn't meet the above criteria, it is added to an internal
buffer and is accumulated until it is ready to be executed.

The REPL will interact with the WebSocket connection using the existent
`terminal` (`lib/web/terminal/terminal.go`) structs.

##### Backslash commands

Backslash commands are executed at the proxy, and results are forwarded to the
client's websocket. Depending on the command executed, there is no interaction
with the database server, and the command won't be included in the session
recording.

There are two categories of backslash commands: those that don't require
database interaction (for example, printing the help menu) and those that expand
into SQL queries forwarded into the database.

For the latter, we'll keep a list of pre-written queries based on the queries
used at `psql`.

##### Command/Queries execution

To execute commands and queries into the target PostgreSQL, the REPL will use a
`pgx.Conn` instead of manually generating PostgreSQL protocol messages. This
will simplify the protocol interaction and cover most use cases.

The `pgx.Conn` will be started during the REPL initialization process.

##### Result formatting

Once the query is executed, the interactive shell will parse command tag results
and decide how to render it:
- For `SELECT` commands: Generate an ASCII table using the query results with
  the same format as `psql`. The field information (table head) will be fetched
  using the result field descriptors.
- For other commands: Display the command tag as it is received.

```shell
postgres=# SELECT * FROM events;

id | name
---+-------
 1 | start 
 2 | end 
(2 rows)

postgres=# INSERT INTO events (name) VALUES ('end');
INSERT 0 1
```

Errors will be shown with `ERROR:` prefix.

```shell
postgres=> SELECT err;
ERROR: column "err" does not exist

```

### Security

#### Resource exhaustion attacks

Users could intentionally send large, complex queries to consume excessive
memory or processing power, potentially leading to service degradation or
outages.

The interactive shell will limit the command/query size regarding character
count. It includes the shell internal buffer (used for multi-line commands) and
the line reader.

In addition, the command results might include large sets. `pgx` will handle
this when reading a query's `Rows` results. The interactive shell will iterate
over each row, avoiding loading all at once in memory.

#### Obfuscating malicious activities

Regarding users trying to bypass Teleport's audit system, the same risks and
solutions from CLI access will apply to the newly introduced Web UI access.

This is because the database server still handles all the auditing capabilities,
and the Web UI interactive shell acts as a regular database client.
