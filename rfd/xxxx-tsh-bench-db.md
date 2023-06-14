---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: draft
---

# RFD xxxx - tsh bench db

## Required Approvals

* Engineering: @smallinsky && @r0mant
* Product: @xinding33 || @klizhentas
* Security: @reedloden

## What
Introduce a new subcommand, `tsh bench db`, which will work similarly to
`tsh bench` for SSH and Kubernetes benchmarking but for database access.

## Why
The team is working towards understanding performance issues with database
access. This built-in tool will be used to generate database access load and
produce some client-side metrics that will be used to understand overall
performance better.

In addition, the command will also have a mode of direct running the load to the
databases (without Teleport proxying them). The team can use this to compare
Teleport performance with direct database access.

## Prior art
The `tsh` already has commands for performing benchmarks. Those rely on the
structure defined at `lib/benchmark/benchmark.go`. It defines a small framework
to define benchmarks. By following it, we can rely on the tools already used on
the SSH and Kubernetes benchmarks, such as reports.

To help readers better understand how the database benchmark will be organized,
here is a summary of it:
* Benchmark suites: Implements a function that generates workload functions.
  The suite is called before the benchmark starts and returns a workload function. It has access to the Teleport Client.
* Workload function: Functions that will be repeatedly executed to grab data.
  Since the suite generates it, it can use information from the suite function.
  For example, on the SSH benchmark, the servers list used to select the server
  where the command will be executed is fetched on the suite function, and the
  workload function only connects to it.

## Details
The database flow areas will be split into isolated suites, allowing developers
to test and validate them individually.

The suites will use an interface (`DatabaseClient`) to interact with databases.
Decoupling it from the suites will increase reusability and also make it quicker
to enable more database protocols to the benchmark tests.

This interface is returned after connecting to the database. Each protocol will
define its connection flow respecting the `ConnectFunc`.

```go
// DatabaseConnectionConfig contains all information necessary to establish a
// new database connection.
type DatabaseConnectionConfig struct {
	// Protocol database protocol.
	Protocol string
	// URI direct database connection URI.
	URI string
	// Username database username the connection should use.
	Username string
	// Database database name where the connection should point to.
	Database string
	// ProxyAddress Teleport database proxy address.
	ProxyAddress string
	// TLSConfig TLS configuration containing Teleport CA and database
	// certificates.
	TLSConfig *tls.Config
}

// ConnectFunc is a function that establishes a database connection and returns
// the DatabaseClient.
type ConnectFunc func(ctx context.Context, config *DatabaseConnectionConfig) (DatabaseClient, error)

// DatabaseClient represents a database connection.
type DatabaseClient interface {
	// Ping runs a command on the database to ensure the connection is alive.
	Ping(ctx context.Context) error
	// Close closes the connection.
	Close(ctx context.Context) error
	// Setup creates the necessary struct to create and load records.
	Setup(ctx context.Context) error
	// Teardown drops anything the test has created.
	Teardown(ctx context.Context) error
	// CreateRecord creates a new record on the database.
	CreateRecord(ctx context.Context) error
	// LoadRecord loads a record from the database.
	LoadRecord(ctx context.Context) error
}
```

#### Connection establishment
It covers authorization capabilities and also initial connections. Although the
other suites will also cover this, we can have more options, like providing
different users and having the ability to run it across different database
protocols. It will not rely on any table or struct on the connected database, as
it will only perform a ping to ensure the connection is healthy. For databases
without support to ping, a simple query will be sent. For example, the suite
will send a `SELECT 1;` query on PostgreSQL databases.

This suite can also validate changes on the access checker without running large
amounts of queries.

```code
$ tsh bench db connect --db-user=postgres --db-name=postgres postgres-dev
```

#### Import/Export data
It is a typical flow for users to import data to their databases. This suite is
focused on running multiple queries within the same database connection (much
like what an import tool would do). To do so, some databases will require an
additional step executed before the suite starts. For example, on PostgreSQL,
the necessary tables must be created. This setup won't be performed inside the
workload function.

The exporting suite will load data outside the workload function. This way, the
measuring will only include read queries.

##### Data generation
The database clients will have access to a set of functions that can be used to
generate random data. Those functions will cover the following types: `[]byte`,
`string`,  `bool`, `int64`, and `float64`. Some of those functions, such as
generating `[]byte` and `string`, will make it possible to define their length.
The clients will be responsible for determining their structure (if required,
creating them on the `Setup` call) and filling with data on `CreateRecord`.

### Direct database connections
To directly connect to the databases, the suites must accept the credentials
that will be used. To avoid adding multiple flags to cover the scenarios, we're
heading into having a single `--uri` flag, which will receive the database URI
with all options included. This way, users can choose the authentication method
that will be used.

```code
$ tsh bench db connect --uri postgres://hello@localhost:5432
```

By providing this flag, the `tsh` will connect directly to the database without
using any Teleport certificate.

## Security

### Bypass MFA or access checker
The command will use the same flow used by `tsh db login` to issue certificates,
including support to MFA. The main difference is that the issue credentials
won't be persisted on disk.

### Malicious URI on the direct database connection
Users may try to send malicious URI when connecting directly to databases.
Teleport doesn't validate the input provided; it is passed directly to the
database client responsible for validating it. Client execution is entirely
done locally. Nothing is sent to the Teleport server.

## Future work

### Run benchmark on multiple databases
As in the first versions, benchmarks are executed in the specified database.
Providing a predicate query enables users to use different databases similar
to the `all` host that can be used on the SSH benchmark.

```code
$ tsh bench db connect --query 'resource.spec.protocol == "postgres"'
```
