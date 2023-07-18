---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0138 - Postgres backend storage

## Required Approvers

* Engineering: @fspmarshall && (@rosstimothy || @zmb3)
* Product: @xinding33 || @klizhentas || @russjones

## What

This RFD proposes a backend implementation and an audit log implementation using [PostgreSQL](https://www.postgresql.org/) as the underlying storage mechanism.

## Why

Currently, users looking to self-host Teleport in a High Availability (HA) deployment outside of AWS (where they could use DynamoDB) or GCP (where they could use Firestore) must be willing and capable to operate [etcd](https://etcd.io/), as that's the only HA-supporting, self-runnable backend currently supported by Teleport. In addition, they must handle shipping log files from all their auth servers into some long-term storage facility or forgo the use of the Teleport audit log, as there is no HA-capable audit log supported by Teleport that's self-hostable outside of said cloud environments.

With the Postgres backend and audit log, users with infrastructure on Azure will be able to use the managed [Azure Database for PostgreSQL](https://azure.microsoft.com/en-us/products/postgresql/) service, with [Managed Identities](https://learn.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/) allowing secretless authentication between Teleport and the database, and users running infrastructure on bare metal will have an option other than etcd for a HA deployment of Teleport, including a working audit log.

## Details

### Non-goals

We don't intend for this backend and audit log to be extensible to completely different RDBMS (such as MariaDB or Microsoft SQL Server); support for other Postgres-compatible databases (such as CockroachDB) should be possible in the future, but is not in scope for this RFD.

### General architecture of the backend

The Teleport backend abstraction consists of a key-value store (where both key and value are opaque bytestrings), with a per-item optional Time-To-Live (TTL), and a stream of changes to the data. As Postgres provides a way to fetch changes to the data over time through [logical decoding](https://www.postgresql.org/docs/current/logicaldecoding.html) of the Write-Ahead Log, we can tailor the database schema to the exact needs of the backend operations.

As such, the schema consists of a single `kv` table, defined as such:
```pgsql
CREATE TABLE kv (
    key bytea NOT NULL,
    value bytea NOT NULL,
    expires timestamptz,
    revision uuid NOT NULL,
    CONSTRAINT kv_pkey PRIMARY KEY (key)
);
CREATE INDEX kv_expires_idx ON kv (expires) WHERE expires IS NOT NULL;
```

Each item in the backend is represented by a row in the `kv` table, with a `NULL` value for `expires` representing no expiration. To efficiently delete expired items, we keep a (partial) index on the expiration time. The `revision` column contains the revision for the upcoming opportunistic locking, and shall be set to a new unique value by the application code for every write (unfortunately, it's not a constraint that's expressible in SQL).

With this very simple schema, all backend operations can be expressed in a single SQL statement, making all operations take a single round-trip between the Teleport auth server and the database once the prepared statement cache is warm.

As we intend to use logical decoding, we want to avoid transactions that are too big (in terms of rows changed); the only operations that affect more than one row are `DeleteRange` (which, in actual operation, is only ever used to delete a handful of rows at a time, for things like deleting a user's properties, password and MFA devices at once) and the deletion of expired items, which we'll have to do ourselves. Since logical decoding starts having performance issues when reaching the _thousands_ of rows changed at once, we only need to be careful when doing expired item deletion. Since we have no requirement that the deletion happens atomically (or in any specific order), we will take care to apply the operation across multiple transactions, limiting the amount of rows affected in each - such a mass expiration can happen, for instance, with the heartbeats of a large amount of nodes that have lost connection to the Teleport cluster.

Each auth server running the backend will periodically attempt to delete expired items - at an interval tentatively set to 30 seconds - deleting small batches of expired items in different transactions until no more items can be deleted.

To generate an event stream we'll connect to the database with a regular connection, create a temporary logical replication slot, then poll changes on it via the [`pg_logical_slot_get_changes`](https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-REPLICATION) function. As the builtin `pgoutput` logical decoding plugin cannot be used without errors via the SQL interface except in very recent versions of Postgres (11.20, 12.15, 13.11, 14.8 or 15.3, released in May 2023), we use the third-party but very popular [`wal2json`](https://github.com/eulerto/wal2json) plugin, which is available by default on the managed Postgres offerings in Azure, AWS and GCP, and is installable from the Debian/Ubuntu official packages or from the Postgres APT and YUM repos.

The `pg_logical_slot_get_changes` function (with the appropriate `wal2json` options) will return one entry per change, which maps nicely to the backend event model; updates and inserts will be rendered as `OpPut` events, deletes as `OpDelete` events.

### General architecture of the audit log

The database schema for the audit log will be composed of just an `events` table with the appropriate indexes for searching:

```pgsql
CREATE TABLE events (
    event_time timestamptz NOT NULL,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    session_id uuid NOT NULL,
    event_data json NOT NULL,
    creation_time timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_pkey PRIMARY KEY (event_time, event_id)
);
CREATE INDEX events_creation_time_idx ON events USING brin (creation_time);
CREATE INDEX events_search_session_events_idx ON events (session_id, event_time, event_id)
    WHERE session_id != '00000000-0000-0000-0000-000000000000';
```

While the database schema is compatible with the one for the backend, we are going to strongly discourage running the backend and the audit log in the same database (using two different databases in the same instance of Postgres is ok), because logical decoding is per-database, not per-table, and keeping both in the same database will require extra work to filter out the changes to the audit log - especially when sifting through the transactions deleting events that are past the retention period.

The `event_id` column is a UUID chosen at random at insert time, unrelated to the event data, only used to disambiguate events for the purpose of pagination; `creation_time`, likewise, is only used for deleting events after some retention period (defaulting to one year).

With minor modifications, this schema can support range based partitioning by `event_time`, creating partitions by hand or using `pg_partman`. The initial implementation will not use partitioning, but setting up the table layout for it will allow for a much easier migration if and when we end up supporting it.

### Requirements and recommendations

Any supported version of Postgres (11 to 15) should work, with `wal2json` 2.1 or later, although for better results, Postgres 13 or higher is recommended (as logical decoding had some internal improvements).

Because the db user that Teleport connects as needs `REPLICATION` permissions, which effectively grants read access to the entire database cluster, the backend should not be used in a database cluster that also contains data that the Teleport auth server should not have access to (it's fine to use a single cluster for the backend and the audit log storage, for instance).

Because of how temporary replication slots (and prepared statements) are tied to individual connections, the use of external connection poolers (such as `pgbouncer`) is not supported.

For self-managed Postgres setups we recommend the use of [certificate authentication](https://www.postgresql.org/docs/current/auth-cert.html); in Azure we support and recommend the use of Azure AD authentication over the use of hardcoded passwords.

Teleport will attempt to create the databases specified in the configuration for itself, and set them up with the correct schema (as well as apply any future migrations, if needed). It's possible to create the databases manually and not give Teleport permissions to create databases, and technically it's even possible to manually set up the database schemas (to avoid granting Teleport the ability to create tables, for instance), but we recommend giving Teleport its own database for the backend - and since Teleport will need read-write access to all the data in it anyway, we also recommend letting Teleport manage the schema automatically.

It's possible to only grant `INSERT` and `SELECT` privileges to the Teleport db user on the table that stores audit events, but not UPDATE or DELETE privileges; such a setup couldn't be automatically managed by Teleport, however, and thus will require detailed documentation on how to set up the schema, and some care when upgrading - Teleport will refuse to start if the schema version is outdated and can't be fixed by Teleport itself.

For reliability, it's recommended to run a standby server with failover and synchronous replication (by setting `synchronous_standby_names`). The details on how to do this vary depending on the way Postgres is deployed; for instance, on Azure it's sufficient to enable ["High availability" mode](https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-high-availability).

### UX

The backend and audit log are configured in the `storage` section of the `teleport.yaml` config file:

```yaml
teleport:
  storage:
    type: postgresql
    conn_string: host=testdb.postgres.database.azure.com port=5432 sslmode=verify-full user=teleport_auth dbname=teleport_backend
    auth_mode: azure
    azure_client_id: ""

    change_feed_poll_interval: "1s"
    change_feed_batch_size: 10000

    disable_expiry: false
    expiry_interval: "30s"
    expiry_batch_size: 1000

    audit_events_uri:
      - postgresql://teleport_auth@testdb.postgres.database.azure.com:5432/teleport_events?sslmode=verify-full#auth_mode=azure&disable_cleanup=false&cleanup_interval=1h&retention_period=8766h
```

The `conn_string` used by the backend is a [`libpq`-compatible Postgres connection string](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING), and can be specified in both keyword-value format and as a URI. The audit log URI can only be specified as a URI. In addition to the standard Postgres options, the options supported by [`pgxpool`](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#ParseConfig) are also supported - `pool_max_conns` being the most useful. Non-Postgres parameters for the audit log are passed as query parameters in the fragment of the URI.

The `auth_mode: azure` field for the backend and the `auth_mode=azure` query parameter enable authentication to the database via Azure AD. The optional `azure_client_id` field (for both the backend and the audit log) override the default azure credential selection and the selection in the `AZURE_CLIENT_ID` envvar, so it's possible to select two different managed identities for backend and audit log. A blank (or unset) `auth_mode` will use the default Postgres authentication methods.

The `change_feed_*` options regulate the internal behavior of the change feed polling, the `expiry_*` and `disable_expiry` regulate the behavior of the TTL deletions. We're not expecting users to have to tweak these options (except maybe `disable_expiry` if TTL deletion is going to be run externally with `pg_cron` or similar methods), they're just exposed for performance tuning.

The `disable_cleanup`, `cleanup_interval` and `retention_period` options to the audit log are optional, and can be set and changed to support manual cleanup of old events, or to change the data retention period for the audit log. The default `retention_period` is one year (365.25 days).

### Rejected alternatives and future work

Early tests with an in-database change feed mechanism (using a global, transactional counter to guarantee correct ordering of events in an `events` table) seemed to show that the performance was not up to the task, failing to sustain idle cluster operations for more than 3500 connected agents. It's possible that using some sharded counter would improve the performance of such an approach, but the added complexity in polling for such events makes it a less appealing option than just using logical decoding together with very simple operations on the data.

Postgres is not the only storage mechanism that matches "key-value store with TTL and change feed", "can be self-hosted" and "has a managed solution in Azure" - a combination of Scylla and Azure Cosmos DB for Apache Cassandra would've also fit the bill, but early feedback from some users showed that it's far more common to have in-house competencies to operate Postgres than to operate Scylla, so the choice fell on Postgres.
