## Firestore Events backend implementation for Teleport.

### Introduction

This package enables Teleport auth server to store secrets in 
[Firestore](https://cloud.google.com/firestore/docs/) on GCP.

WARNING: Using Firestore events involves recurring charge from GCP.

### Building

Firestore events is not enabled by default. To enable it you have to 
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
    audit_events_uri: 'firestore://events?projectID=gcp-proj-with-firestore-enabled&credentialsPath=/var/lib/teleport/gcs_creds'
```

Collections are automatically created by the Firestore APIs and the required indexes are created
by the event backend on first start, if they do not exist. 

### Full Properties

The full list of configurable properties for this backend are:

- host portion of URI is the Firestore collection used to persist stored events
- `credentialsPath` (string, path to GCP creds for Firestore, not-required)
- `projectID` (string, project ID, **required**)
- `purgeInterval` (time duration, poll interval to sweep expired documents, not-required, defaults to `once per minute`)
- `retryPeriod` (time duration, retry period for all background tasks, not-required, defaults to `10 seconds`)
- `disableExpiredDocumentPurge` (bool, disables expired document purging, not-required, defaults to `false`)
- `eventRetentionPeriod` (int, buffer size for watched events, not-required, defaults to `1024`)
- `endpoint` (string, firestore client endpoint, not-required, ex: `localhost:8618`)

### Firestore Client Authentication Options

There are three authentication/authorization modes available;

1. With no `credentialsPath` and no `endpoint` defined, the Firestore clients will use
Google Application Default Credentials for authentication. This only works in cases
where Teleport is installed on GCE instances and have service accounts with IAM role/profile
associations authorizing that GCE instance to use Firestore.  
2. With `endpoint` defined, Firestore will create clients no auth, GRPC in-secure, clients pointed
at the specified endpoint. **This is only used for tests, see `Tests` section below.**
3. With `credentialsPath` defined, Firestore will create clients authenticating against
live systems with the Service Account bound to the JSON key file referenced in the option.  

### Implementation Details

Firestore Document IDs must be unique, cannot start with periods, and cannot contain forward
slashes. In order to support more straight forward fetching but work within the requirements
of Firestore, Document IDs are the concatenation of the session ID (a UUID) and event type joined with a dash `-`,
ex: `13498a42-69a8-4fa2-b39d-b0c49e346713-user.login`.

Expired event purging should be enabled on as few instances as possible to reduce query costs,
though there's no harm in having every instance query and purge. Purging is enabled based on
the `purgeExpiredDocuments` property, which defaults to true. Purging is done based on the
configurable `eventRetentionPeriod` property, which defaults to a year. Add this property to
the URI to change the retention period.

Two composite indexes are required for this implementation:

1. `EventNamespace` ascending, then on `CreatedAt` ascending
2. `SessionID` ascending, then on `EventIndex` ascending

Composite indexes should be limited to the specific collection set in the
configuration (in the aforementioned example is `events`).

### Tests

Tests must execute one of two ways:

1. With `gcloud` installed in test infrastructure and the `firestore` emulator enabled
and running to a dynamic port a pre-defined port used in the config.
Ex: `gcloud beta emulators firestore start --host-port=localhost:8618`. This is where the Firestore config
parameter `endpoint` is used.
2. With a service account pointed a test GCP project and or test collections.

### Get Help

This backend has been contributed by https://github.com/joshdurbin