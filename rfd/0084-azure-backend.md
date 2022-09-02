---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0084 - Azure backend storage

## Required Approvers
* Engineering: TBD
* Security: TBD
* Product: TBD

## What

Enable users to leverage Azure-native services as a Teleport backend.
We will accomplish this by:
- adding support for Azure IAM authentication to the existing Postgres backend
- adding audit log capabilities to the `sqlbk`/Postgres backend
- adding a session uploader that uses Azure Blob Storage (with the same or similar IAM authentication)

## Why

To support deployments of Teleport in Azure with the same convenience and reliability that's currently available when using managed storage in AWS and GCP.

## Details

### Auth backend

Even though our backend is ultimately just a key-value store, the managed NoSQL database offering in Azure, called Cosmos DB, is not sufficient for our purpose, because our cache propagation model relies on all auth servers having access to a stream of events that replicates all the changes applied to the backend. Cosmos DB has a change feed, but it doesn't contain every change to values and it doesn't have deletions - as such, it's not usable as the backing store for our auth backend (a "full fidelity" change feed has been [in private preview since 2020](https://azure.microsoft.com/en-us/updates/change-feed-with-full-database-operations-for-azure-cosmos-db/)).

As using Postgres as the backend storage for Teleport has been possible (in Preview) since v9.0.4, and Azure offers a managed Postgres service, we will be adding support for Azure AD authentication to the Postgres backend - that's the only change needed to support Azure Postgres in Teleport. As of August 2022, only the "Single Server" flavor of Azure Postgres supports AD authentication and it's limited to Postgres 11, which will be [deprecated in November 2023](https://docs.microsoft.com/en-us/azure/postgresql/single-server/concepts-version-policy#major-version-retirement-policy), but it seems likely that AD authentication will also be supported in the "Flexible Server" offering by then.

### Configuration

This are some options on how the configuration side might look like.

The original SQL Backend RFD imagined something like:

```yaml
teleport:
  storage:
    type: postgres
    addr: pgservername.postgres.database.azure.com
    database: dbname_defaulting_to_teleport
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
    azure:
      username: pgusername@pgservername
```

Alternatively, we could provide additional aliases for the Postgres backend:

```yaml
teleport:
  storage:
    type: azurepostgres
    addr: pgservername.postgres.database.azure.com
    username: pgusername@pgservername
    database: dbname_defaulting_to_teleport
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
```

Or we could add some "authentication type" field:

```yaml
teleport:
  storage:
    type: postgres
    addr: pgservername.postgres.database.azure.com
    username: pgusername@pgservername
    database: dbname_defaulting_to_teleport
    authentication: azure
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
```

Other options such as checking for the `.postgres.database.azure.com` suffix in the hostname seem a bit too fragile.

Authentication will happen according to the default behavior of [the Azure Go SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity), which will try to use credentials from envvars, from the IDMS accessible from Azure VMs, and from the command-line `az` tool - it would be straightforward to add some tunables to the configuration file if some specific need arose in the future.

## Audit log

The current two cloud storage options (DynamoDB on AWS and Firestore on GCP) can store both the auth backend and the events in the audit log. To match the same convenience, we'll implement audit log storage for Postgres - an alternative would be to store the audit log in Cosmos DB, but that would require that our users set up both a Postgres server and a Cosmos DB storage account. As a bonus, this option will also be usable outside of Azure as a HA-capable audit log to use with etcd or Postgres as a backend.

The database schema will be composed of just an `audit` table:
```pgsql
CREATE TABLE audit (
    count BIGSERIAL PRIMARY KEY,
    time TIMESTAMP NOT NULL,
    event TEXT NOT NULL,
    sid UUID,
    ei BIGINT NOT NULL,
    data JSONB,
    UNIQUE (sid, ei)
);
CREATE INDEX ON audit (time, event, sid, ei);
```

We'll have to make sure to leave the SQL backend schema and the SQL audit log schema compatible, so that they can use the same database and credentials, for ease of setup.

Configuration will be something like the following:

```yaml
teleport:
  storage:
    ...

    audit_events_uri: ['postgres://host:123/database?sslmode=verify-full&sslrootcert=cafile&sslcert=certfile&sslkey=keyfile']
```

IAM authentication in such case would require some bespoke parameter in the query or fragment of the URI:

```yaml
teleport:
  storage:
    ...

    audit_events_uri: ['postgres://host:123/database?sslmode=verify-full&sslrootcert=cafile&user=pgusername%40pgservername#auth=azure']
```

Additional security could be provided by only granting `INSERT` (and `SELECT`) privileges to the Postgres user on the table that stores audit events, but not `UPDATE` or `DELETE` privileges; such a setup couldn't be automatically managed by Teleport, and thus would require detailed documentation and some care.

## Session storage

Azure Blob Storage offers similar functionality to S3, including all we need for `lib/events.MultipartUploader` in the strictest sense. In addition to that, it's possible to configure containers (the equivalent of a S3 bucket) to have a time-based immutability policy that prevents moving, renaming, overwriting or deleting a blob for a fixed amount of time after it's first written, which seems to be exactly what we're currently using versioning for.

Similarly to other cloud-based session storage, it would be configured like this, specifying a storage account URL, replacing the `https` schema with `azblob`; an (undocumented?) alternate `azblob-http` schema will be provided to use an endpoint without encryption, for local development.

```yaml
teleport:
  storage:
    ...

    audit_sessions_uri: "azblob://accountname.blob.core.windows.net"
```

Other configuration options, if required, will be passed in as query parameters in the URI, like the other session storage backends.

Teleport will require access to two containers in the storage account, named `session`, which will store the completed session files, and `inprogress`, which will be used as temporary space to hold parts.

To signify the creation of an upload, a random UUID is generated, and an empty blob is created in `inprogress` with name `upload/<session id>/<upload id>`; this will be used when listing unfinished uploads, by listing blobs whose name begins with `upload/`. Parts are uploaded in `inprogress` with name `part/<session id>/<upload id>/<part number>`, which will allow listing the parts for a specific upload. To complete an upload, parts will be used as blocks to compose the final session recording in the `session` container (in a blob that has the session ID as its name); this can be done efficiently with the "Put Block By URL" call.

Using a separate container for the final recordings lets the user configure an [immutability policy](https://docs.microsoft.com/en-us/azure/storage/blobs/immutable-time-based-retention-policy-overview) at the container level, which will prohibit any modification of completed blobs until a specified amount of time has passed or the block is removed; likewise, recordings older than a certain date can be configured to be automatically deleted via a [lifecycle management policy](https://docs.microsoft.com/en-us/azure/storage/blobs/lifecycle-management-overview).
