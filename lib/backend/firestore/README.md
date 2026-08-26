## Firestore backend implementation for Teleport.

### Introduction

This package enables Teleport auth server to store secrets in 
[Firestore](https://cloud.google.com/firestore/docs/) on GCP.

WARNING: Using Firestore involves recurring charge from GCP.

### Building

Firestore backend is not enabled by default. To enable it you have to 
compile Teleport with `firestore` build flag.

To build Teleport with Firestore enabled, run:

```
ADDFLAGS='-tags firestore' make teleport
```

### Quick Start

There are currently two Firestore mode options for any given GCP Project; `Native mode` and
`Datastore Mode`. This storage backend uses Real-time updates to keep individual auth instances
in sync and requires Firestore configured in `Native mode`.  

Add this storage configuration in `teleport` section of the config file (by default it's `/etc/teleport.yaml`):

```yaml
teleport:
  storage:
    type: firestore
    collection_name: cluster-data
    credentials_path: /var/lib/teleport/firestore_creds
    project_id: gcp-proj-with-firestore-enabled
```

Collections are automatically created by the Firestore APIs and the required indexes are created
by the backend on first start, if they do not exist.

### Encryption

Lifted from [Google docs](https://cloud.google.com/firestore/docs/server-side-encryption); Cloud Firestore automatically
encrypts all data before it is written to disk. There is no setup or configuration required and no need to modify the
way you access the service. The data is automatically and transparently decrypted when read by an authorized user.

With server-side encryption, Google manages the cryptographic keys on your behalf using the same hardened key management
systems that we use for our own encrypted data, including strict key access controls and auditing. Each Cloud Firestore
object's data and metadata is encrypted under the 256-bit Advanced Encryption Standard, and each encryption key is itself
encrypted with a regularly rotated set of master keys.

### Full Properties

The full list of configurable properties for this backend are:

- `credentials_path` (string, path to GCP creds for Firestore, not-required)
- `project_id` (string, project ID, **required**)
- `collection_name` (string, collection for cluster information, **required**)
- `purge_expired_documents_poll_interval` (time duration, poll interval to sweep expired documents, not-required, defaults to `once per minute`)
- `retry_period` (time duration, retry period for all background tasks, not-required, defaults to `10 seconds`)
- `disable_expired_document_purge` (bool, disables expired document purging, not-required, defaults to `false`)
- `buffer_size` (int, buffer size for watched events, not-required, defaults to `1024`)
- `endpoint` (string, firestore client endpoint, not-required, ex: `localhost:8618`)
- `limit_watch_query` (bool, forces the watcher to start querying only for records from current time forward, not-required, defaults to `false`)

### Firestore Client Authentication Options

There are three authentication/authorization modes available;

1. With no `credentialsPath` and no `endpoint` defined, the Firestore clients will use
Google Application Default Credentials for authentication. This only works in cases
where Teleport is installed on GCE instances and have service accounts with IAM role/profile
associations authorizing that GCE instance to use Firestore.  
2. With `endpoint` defined, Firestore will create clients no auth, gRPC in-secure, clients pointed
at the specified endpoint. **This is only used for tests, see `Tests` section below.**
3. With `credentialsPath` defined, Firestore will create clients authenticating against
live systems with the Service Account bound to the JSON key file referenced in the option.

### Implementation Details

Firestore Document IDs must be unique, cannot start with periods, and cannot contain forward
slashes. In order to support more straight forward fetching but work within the requirements
of Firestore, Document IDs are SHA1 hashed from the records key.

Realtime updates are consumed across the cluster via Firestore's [ability](https://cloud.google.com/firestore/docs/query-data/listen)
to watch for document updates.

One composite indexes is required for this implementation:

1. `key` ascending, then on `expires` ascending

Composite indexes should be limited to the specific collection set in the
configuration (in the aforementioned example is `cluster-data`).

### Tests

Tests must execute one of two ways:

1. With `gcloud` installed in test infrastructure and the `firestore` emulator enabled
and running to a dynamic port a pre-defined port used in the config.
Ex: `gcloud beta emulators firestore start --host-port=localhost:8618`. This is where the Firestore config
parameter `endpoint` is used.
2. With a service account pointed a test GCP project and or test collections.  

### Get Help

This backend has been contributed by https://github.com/joshdurbin