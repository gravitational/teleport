---
authors: Alexander Klizhentas (sasha@gravitational.com)
state: discussion
---

# RFD 2 - Session Streaming

## What

Design and API of streaming and storing structured session events.

## Why

Existing API and design for sending and storing session events has several
issues.

In pre 4.3 implementation events were buffered on disk on proxies or nodes.
This required encryption at rest, and allowed attackers to tamper
with event data. Session recording was uploaded as a single tarball,
auth server had to unpack the tarball in memory to validate it's contents,
causing OOM and other performance issues. Events were not structured, and often
clients were omitting and sending wrong fields not validated by the server.

## Details

### Structured Events

Events have been refactored from unstructured to structured definitions generated
from protobuf spec.

Each event embeds common required metadata:

```protobuf
// Metadata is a common event metadata
message Metadata {
    // Index is a monotonically incremented index in the event sequence
    int64 Index = 1;

    // Type is the event type
    string Type = 2;

    // ID is a unique event identifier
    string ID = 3;

    // Code is a unique event code
    string Code = 4;

    // Time is event time
    google.protobuf.Timestamp Time = 5;
}
```

This metadata is accompanied by common event methods:

```go
// GetType returns event type
func (m *Metadata) GetType() string {
	return m.Type
}

// SetType sets unique type
func (m *Metadata) SetType(etype string) {
	m.Type = etype
}
```

That allow every event to have a common interface:

```go
// AuditEvent represents audit event
type AuditEvent interface {
	// ProtoMarshaler implements efficient
	// protobuf marshaling methods
	ProtoMarshaler

	// GetID returns unique event ID
	GetID() string
	// SetID sets unique event ID
	SetID(id string)
```

**Session events**

Session events embed session metadata:

```
// SesssionMetadata is a common session event metadata
message SessionMetadata {
    // SessionID is a unique UUID of the session.
    string SessionID = 1;
}
```

And implement extended interfaces:

```go
// ServerMetadataGetter represents interface
// that provides information about it's server id
type ServerMetadataGetter interface {
	// GetServerID returns event server ID
	GetServerID() string

	// GetServerNamespace returns event server namespace
	GetServerNamespace() string
}
```

This approach allows common event interface to be converted to other
event classes without casting to specific type:

```go
	getter, ok := in.(events.SessionMetadataGetter)
	if ok && getter.GetSessionID() != "" {
		sessionID = getter.GetSessionID()
	} else {
```

**Other event types**

Other event types, such as events dealing with connections embed other  metadata,
for example connection metadata events:

```go
// Connection contains connection info
message ConnectionMetadata {
    // LocalAddr is a target address on the host
    string LocalAddr = 1 ;

    // RemoteAddr is a client (user's) address
    string RemoteAddr = 2;

    // Protocol specifies protocol that was captured
    string Protocol = 3;
}
```

### Streams

Streams are continuous sequence of events associated with a session.

```go
// Stream used to create continuous ordered sequence of events
// associated with a session.
type Stream interface {
	// Status returns channel receiving updates about stream status
	// last event index that was uploaded and upload ID
	Status() <-chan StreamStatus
....
}
```

Streamer is an interface for clients to send session events to the auth
server as a continuous sequence of events:

```go
// Streamer creates and resumes event streams for session IDs
type Streamer interface {
	// CreateAuditStream creates event stream
	CreateAuditStream(context.Context, session.ID) (Stream, error)
	// ResumeAuditStream resumes the stream for session upload that
	// has not been completed yet.
	ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (Stream, error)
}
```

Clients can resume streams that were interrupted using upload ID.


Clients can use stream status to create back-pressure
- stop sending until streams reports events uploaded -
or resume the upload without re sending all events.

### Uploaders

`MultipartUploader` interface handles multipart uploads and downloads for session streams.

```go
type MultipartUploader interface {
	// CreateUpload creates a multipart upload
	CreateUpload(ctx context.Context, sessionID session.ID) (*StreamUpload, error)
	// CompleteUpload completes the upload
	CompleteUpload(ctx context.Context, upload StreamUpload, parts []StreamPart) error
	// UploadPart uploads part and returns the part
	UploadPart(ctx context.Context, upload StreamUpload, partNumber int64, partBody io.ReadSeeker) (*StreamPart, error)
	// ListParts returns all uploaded parts for the completed upload in sorted order
	ListParts(ctx context.Context, upload StreamUpload) ([]StreamPart, error)
	// ListUploads lists uploads that have been initated but not completed with
	// earlier uploads returned first
	ListUploads(ctx context.Context) ([]StreamUpload, error)
}
```

Uploaders provide abstraction over multipart upload API, specifically S3 for AWS and GCS for Google.
The stream on-disk format is optimized to support parallel uploads of events to S3 and resuming of uploads.

### Session events storage format

The storage format for session recordings is designed for fast marshal and unmarshal
using protobuf, compression using gzip and support for parallel uploads to S3 or GCS storage.

Unlike previous file recording format using JSON and storing multiple files in a tarball,
V1 format represents session as continuous globally ordered sequence of events
serialized to protobuf.

Each session is stored in one or many slices. Each slice is composed of three parts:

1. Slice starts with 24 bytes version header:

   * 8 bytes for the format version (used for future expansion)
   * 8 bytes for meaningful size of the part
   * 8 bytes for padding at the end of the slice (if present)

2. Slice body is gzipped protobuf messages in binary format.

3. Optional padding if specified in the header is required to
ensure that slices are of the minimum slice size.

The slice size is determined by S3 multipart upload requirements:

https://docs.aws.amazon.com/AmazonS3/latest/dev/qfacts.html

This design allows the streamer to upload slices S3-compatible APIs
in parallel without buffering to disk.

### GRPC

Nodes and proxies are using GRPC interface implementation to submit
individual global events and create and resume streams.

**GRPC/HTTPs protocol switching**

[ServeHTTP](https://godoc.org/google.golang.org/grpc#Server.ServeHTTP)
compatibility handler used to serve GRPC over HTTPs connection had to be replaced with
native GRPC transport, because of the problems described [here](https://github.com/gravitational/oom).

Because of that protocol switching has to be done on TLS level using NextProto.

### Sync and async streams

The V0 stream implementation is async - the sessions are streamed on disk of
proxy and node and then uploaded as a single tarball.

This created performance and stability problems for large uploads, teleport was consuming
all disk space with multipart uploads and security
issues - storage on disk required disk encryption to support FedRamp mode.

In V1 sync and async streams are using the same GRPC API. The only difference
is that in async mode, proxy and nodes are first storing events on disk
and later replay the events to GRPC, while in sync mode clients send GRPC
events as the session generates them.

Each session chooses sync or async emitter based on the cluster configuration
when session is started.

**Sync streams**

New recording modes `proxy-sync` and `node-sync` cause proxy and node send events
directly to the auth server that uploads the recordings to external storage
without buffering the records on disk.

This created potential problem of resuming the session stream.
The new audit writer takes advantage of stream status reporting and a
new option to resume stream to replay the events that have not been uploaded to the storage.

Auth server never stores any local data of the stream and instead initiates
[multipart upload](https://docs.aws.amazon.com/AmazonS3/latest/dev/mpuoverview.html),
it can be resumed by any other auth server. The loss of the single auth server
will not lead to sync sessions termination if another auth server is available
to resume the stream.

**Async streams**

Default mode remains async, the file uploader the events on disk in the new protobuf format. 

Disk uploader now attempts to resume the upload to the auth server based on the
last reported status if possible. This solves the problem of very large uploads
interrupted because of the server overload or intermittent network problems and
auth server can check every event when received, unlike in V0 that required
the tarball to be unpacked first.

### Completing interrupted sessions

In teleport 4.3 and earlier some streams and sessions were never uploaded
to the auth server. The session would stay on the proxy or node without being
uploaded for example in cases when node or proxy crashed before marking
the session on disk as complete.

Switching to S3 based multipart upload API allows auth server to watch uploads
that haven't been completed over grace period (> 12 hours) and complete them.
