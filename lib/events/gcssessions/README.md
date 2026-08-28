## GCS Storage Implementation for Teleport

### Introduction

This package enables Teleport auth server to store session recordings in 
[GCS](https://cloud.google.com/storage/docs/) on GCP.

WARNING: Using GCS involves recurring charge from GCP.

### Building

GCS session storage is not enabled by default. To enable it you have to 
compile Teleport with `gcs` build flag.

To build Teleport with GCS enabled, run:

```
ADDFLAGS='-tags gcs' make teleport
```

### Quick Start

Configuration options are passed to the GCS handler via a URI/URL. The following is a sample
configuration in `teleport` section of the config file (by default it's `/etc/teleport.yaml`):

```yaml
teleport:
  storage:
    audit_sessions_uri: 'gs://teleport-session-storage-2?projectID=gcp-proj&credentialsPath=/var/lib/teleport/gcs_creds'
```

### Full Properties

The full list of configurable properties for this backend are:

- host portion of URI is the GCS bucket used to persist session recordings
- `credentialsPath` (string, path to GCP creds for Firestore, not-required)
- `projectID` (string, project ID, **required**)
- `endpoint` (string, GCS client endpoint, not-required, ex: `localhost:8618`)
- `path` (string, the path inside the GCS bucket to use as storage root, not-required)
- `keyName` (string, the user-defined GCP KMS key name to use for encryption, not-required)

### GCS Client Authentication Options

There are three authentication/authorization modes available;

1. With no `credentialsPath` and no `endpoint` defined, the GCS client will use
Google Application Default Credentials for authentication. This only works in cases
where Teleport is installed on GCE instances and have service accounts with IAM role/profile
associations authorizing that GCE instance to use Firestore.  
2. With `endpoint` defined, GCS will create a client with no auth and clients pointed
at the specified endpoint. **This is only used for tests, see `Tests` section below.**
3. With `credentialsPath` defined, Firestore will create clients authenticating against
live systems with the Service Account bound to the JSON key file referenced in the option.

### Get Help

This backend has been contributed by https://github.com/joshdurbin