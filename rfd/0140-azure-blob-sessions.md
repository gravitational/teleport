---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0140 - Azure Blob Storage session storage

## Required Approvers

* Engineering: @fspmarshall && (@rosstimothy || @zmb3)
* Product: @xinding33 || @klizhentas || @russjones

## What

Enable users to leverage the Azure Blob Storage managed object storage service for Teleport session recordings.

## Why

To support deployments of Teleport in Azure with the same convenience and reliability that's currently available when using managed storage in AWS and GCP.

## Details

Azure Blob Storage offers similar functionality to S3, including all we need for `lib/events.MultipartUploader` in the strictest sense. In addition to that, it's possible to configure containers (the equivalent of a S3 bucket) to have a time-based immutability policy that prevents moving, renaming, overwriting or deleting a blob for a fixed amount of time after it's first written, which will allow us to not rely on versioning (like we do for S3 session storage).

Similarly to other cloud-based session storage, the configuration looks like this, specifying a storage account URL.

```yaml
teleport:
  storage:
    ...

    audit_sessions_uri: "azblob://<storage account name>.blob.core.windows.net"
```

It's possible to specify a client ID for a managed identity (to allow for different identities to be used for backend, events and sessions, as opposed to setting the `AZURE_CLIENT_ID` envvar and using the default credentials) by specifying a URL fragment of `#azure_client_id=11111111-2222-3333-4444-555555555555`. The `azblob` schema is only used by Teleport to identify the storage backend, and will actually result in a `https` URL. For testing against simulators (or other services with the same API surface) over `http`, the `azblob-http` schema is also supported (but we will leave it undocumented for end users).

Teleport will require access to two containers in the storage account, named `session`, which will store the completed session files, and `inprogress`, which will be used as temporary space to hold parts. The container names can be tweaked with query parameters in the fragment: `session_container=foo&inprogress_container=bar`.

To signify the creation of an upload, a random UUID is generated, and an empty blob is created in `inprogress` with name `upload/<session id>/<upload id>`; this will be used when listing unfinished uploads, by listing blobs whose name begins with `upload/`. Parts are uploaded in `inprogress` with name `part/<session id>/<upload id>/<part number>`, which will allow listing the parts for a specific upload. To complete an upload, parts will be used as blocks to compose the final session recording in the `session` container (in a blob that has the session ID as its name); this can be done efficiently with the "Put Block By URL" call.

Using a separate container for the final recordings lets the user configure an [immutability policy](https://docs.microsoft.com/en-us/azure/storage/blobs/immutable-time-based-retention-policy-overview) at the container level, which will prohibit any modification of completed blobs until a specified amount of time has passed or the block is removed; likewise, recordings older than a certain date can be configured to be automatically deleted via a [lifecycle management policy](https://docs.microsoft.com/en-us/azure/storage/blobs/lifecycle-management-overview).
