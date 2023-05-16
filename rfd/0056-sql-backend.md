---
authors: Jim Bishopp (jim@goteleport.com)
state: deprecated
---

# RFD 56 - SQL Backend


## What

A Teleport Backend is a pluggable interface for storing core cluster state.
This RFD proposes the addition of a new backend supporting SQL database platforms.
The initial supported platforms for the SQL Backend will be PostgreSQL and
CockroachDB. Support will include self-hosted PostgreSQL/CockroachDB or
hosting the database on a cloud provider platform (AWS/GCP).


## Why

Supporting a SQL Backend reduces onboarding time for Teleport customers by
allowing them to use existing infrastructure.


## Scope

This RFD focuses on implementing a SQL Backend for PostgreSQL and CockroachDB
where the database is either self-hosted or cloud-hosted. The implementation's
design will be extensible to allow future work that supports other SQL databases
such as MySQL.

Cloud-hosted configurations will support AWS RDS/Aurora/Redshift and GCP Cloud SQL.

The implementation will support connecting to a single endpoint. Failover to
a secondary endpoint will not be supported but may be considered in a future
proposal.


## Authentication

Self-hosted configurations will require mutual TLS authentication using
user-generated client certificates. The user's CA, client certificate, and
client key must be added to the Teleport configuration file. Users must ensure
the provided CA is trusted by the host where the Teleport authentication server
is running.

AWS and GCP cloud-hosted configurations require IAM. Teleport uses the
default credential provider for both AWS and GCP to authenticate using IAM.

Mutual TLS authentication is optionally supported for GCP Cloud SQL.
Paths to a client certificate and key must be added to the Teleport
configuration file to enable mTLS.


## UX

Teleport users must first configure the instance and database where Teleport will
store its data. A new database instance and user must be created. The new user
should be granted ownership of the new database and have the ability to login.
And cloud-hosted configurations must configure and enable IAM.

Once the database instance and user are created, Teleport users must enable the
SQL Backend by configuring the storage section in the Teleport configuration
file. Setting the storage type to either `postgres` or `cockroachdb` enables the
SQL Backend. Additional configurations may apply depending on whether the
configuration is for self-hosted or cloud-hosted environments.

```yaml
teleport:
  storage:
    # Type of storage backend (postgres or cockroachdb).
    type: postgres

    # Database connection address.
    addr: "postgres.example.com:5432"

    # Optional database name that Teleport will use to store its data.
    # Defaults to "teleport". The database must not be shared.
    database: "teleport"

    # TLS validation and mutual authentication.
    tls:
      # Path to the CA file that Teleport will use to verify TLS connections.
      ca_file: /var/lib/teleport/backend.ca.crt

      # Paths to the client certificate and key Teleport will use for mutual
      # TLS authentication. Required for self-hosted configurations. Optional
      # for GCP Cloud SQL. Not supported for AWS.
      client_cert_file: /var/lib/teleport/backend-client.crt
      client_key_file: /var/lib/teleport/backend-client.key

    # AWS specific configuration, only required for RDS/Aurora/Redshift.
    aws:
      # Region the database is deployed in.
      region: "us-east-1"

      # Redshift specific configuration (postgres only).
      redshift:
        # Redshift cluster identifier.
        cluster_id: "redshift-cluster-1"

    # GCP specific configuration, only required for Cloud SQL.
    gcp:
      # GCP project ID.
      project_id: "xxx-1234"
      # Cloud SQL instance ID.
      instance_id: "example"
```

## Implementation Details

Users can set the Teleport storage type to either `postgres` or `cockroachdb`.
The initial implementation will use the same PostgreSQL driver for both settings.

Future iterations of Teleport will be able to easily create a unique `cockroachdb`
driver implementation if CockroachDB specific functionality is desired, such as
using hash-sharded indexes, which are not available in PostgreSQL.

### Transaction Isolation

One difference between CockroachDB and PostgreSQL is how they handle transaction
isolation. CockroachDB [always uses `SERIALIZABLE` isolation][1], which is the
strongest of the [four transaction isolation levels][2].

The `SERIALIZABLE` isolation level guarantees that even though transactions may
execute in parallel, the result is the same as if they had executed one at a time,
without any concurrency.

The SQL Backend will enforce `SERIALIZABLE` transaction isolation for all database
platforms. The trade-off is data consistency versus slower execution and the need
to retry transactions when a conflict occurs. Teleport's Backend interface assumes
consistency. E.g. the `Create` function requires the new record does not exist, and
`CompareAndSwap` requires the previous record has a specific value. Using
`SERIALIZABLE` is the desirable isolation level for the majority of Teleport Backend
functionality.

[1]: https://www.cockroachlabs.com/docs/stable/demo-serializable.html
[2]: https://en.wikipedia.org/wiki/Isolation_(database_systems)#Isolation_levels
