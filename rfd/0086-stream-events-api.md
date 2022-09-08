---
authors: Vitor Enes (vitor@goteleport.com)
state: draft
---

# RFD 86 - Stream Events API

## Required Approvals

* Engineering: @r0mant && @jimbishopp

## Table of Contents

* [What](#what)
* [Why](#why)
  * [Goals](#goals)
  * [Non\-Goals](#non-goals)
* [Details](#details)
  * [`Client.StreamEvents` API](#clientstreamevents-api)
  * [`StreamEvents` RPC](#streamevents-rpc)
  * [`IAuditLog.StreamEvents` API](#iauditlogstreamevents-api)
  * [`dynamoevents.Log.StreamEvents` API](#dynamoeventslogstreamevents-api)
	* [DynamoDB stream cursor](#dynamodb-stream-cursor)
	* [`lib/backend/dynamo/shards.go`](#libbackenddynamoshardsgo)
* [Concerns and open questions](#concerns-and-open-questions)

## What

Currently streaming of audit events has to be implemented on top of [`SearchEvents`], as [done by the event-handler] plugin.
However, this may be implemented more efficiently, specially for databases with [streaming capabilities such as DynamoDB].

This RFD proposes a way to extend the [Teleport Client] with a new `StreamEvents` API that will be initially implemented only for the DynamoDB audit log.

[`SearchEvents`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/api/client/client.go#L2137
[done by the event-handler]: https://github.com/gravitational/teleport-plugins/blob/142b8bd7127fe680d42fc4d626d18b3fd8df2b49/event-handler/teleport_events_watcher.go#L175-L184
[streaming capabilities such as DynamoDB]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html
[Teleport Client]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/api/client/client.go

## Why

The Cloud team wants to start tracking the number of monthly active users (MAU) in order to understand the usage and growth of Teleport Cloud.
To achieve this, audit events will be anonymized and sent to some time-series database (TBD).
For efficiency, the plan is to implement this by leveraging DynamoDB streaming capabilities.

### Goals

* Add a new `StreamEvents` API to the Teleport Client
* Implement `StreamEvents` for the DynamoDB audit log

### Non-Goals
* Implement `StreamEvents` all for available audit logs
* Refactor the event-handler plugin to use the new API

## Details

In this section we detail how Teleport can be extended to achieve the above goals.

### `Client.StreamEvents` API

The [Teleport Client] will be extended with a new `StreamEvents` API similar to the `StreamSessionEvents` API added in [teleport#7360].

```go
func (c *Client) StreamEvents(ctx context.Context, cursor string) (chan events.StreamEvent, chan error)

func (c *Client) StreamSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan events.AuditEvent, chan error)
```

`StreamSessionEvents` returns a channel of `events.AuditEvent`s.
`StreamEvents` returns instead a channel of `events.StreamEvent`s that contain the same `events.AuditEvent` in addition to a stream `Cursor`.
This stream `Cursor` can be used to to resume streaming events by passing it as an argument to the `StreamEvents` API.

```go
type StreamEvent struct {
	// Event is an audit event.
	Event AuditEvent
	// Cursor is a stream cursor that can be used to resume the stream.
	Cursor string
}
```

[teleport#7360]: https://github.com/gravitational/teleport/pull/7360

### `StreamEvents` RPC

These two APIs are build on top of server-streaming RPCs with the same name:

```protobuf
// StreamEventsRequest is a request to start or resume streaming audit events.
message StreamEventsRequest {
    // Cursor is an optional stream cursor that can be used to resume the stream.
    string Cursor = 1;
}

message StreamEvent {
	// Event is a typed gRPC formatted audit event.
	events.OneOf Event = 1;
	// Cursor is a stream cursor that can be used to resume the stream.
	string Cursor = 2;
}

service AuthService {
	// ...

	// StreamEvents streams audit events.
	rpc StreamEvents(StreamEventsRequest) returns (stream StreamEvent);
	// StreamSessionEvents streams audit events from a given session recording.
	rpc StreamSessionEvents(StreamSessionEventsRequest) returns (stream events.OneOf);

	// ...
}
```

Similarly to the Teleport API call, the `StreamSessionEvents` RPC returns a stream of `events.OneOf`s, while `StreamEvents` returns a stream of `StreamEvent`s that contain an `events.OneOf` and a stream `Cursor`.

### `IAuditLog.StreamEvents` API

In order to implement the `StreamEvents` RPC, the `IAuditLog` interface will also be extended with a `StreamEvents` API (equal to the `Client.StreamEvents` being added):

```go
type IAuditLog interface {
	// ...

	StreamEvents(ctx context.Context, cursor string) (chan apievents.StreamEvent, chan error)

	StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error)

	// ...
}
```

### `dynamoevents.Log.StreamEvents` API

`IAuditLog.StreamEvents` will only be implemented for [`dynamoevents.Log`].
For that, the existing streaming implementation in [`lib/backend/dynamo/shards.go`], which is used to watch for backend changes, will be generalized in order to support both needs.

In particular, this streaming implementation will have to support resuming the stream given some stream cursor.
This is currently not supported as, upon an error or a server restart, the backend starts streaming from the `LATEST` event in each active shard.

[`dynamoevents.Log`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/events/dynamoevents/dynamoevents.go
[`lib/backend/dynamo/shards.go`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go

#### DynamoDB Stream cursor

Similarly to how [`dynamodb.Log.SearchEvents`] returns a [checkpoint key] that is JSON-encoded, a DynamoDB Stream cursor will be the following JSON-encoded struct:

```go
type streamCursor struct {
	// ShardIdToSequenceNumber is a mapping from a shard id to the latest sequence number
	// (from such shard) returned by the stream
	ShardIdToSequenceNumber map[string]string `json:"shard_id_to_sequence_number,omitempty"`
}
```

Next we give some information about DynamoDB streams, explaining why we have chosen such a representation for the stream cursor.
From the DynamoDB documentation:

> A stream consists of stream records.
> Each stream record is assigned a sequence number, reflecting the order in which the record was published to the stream.
> Stream records are organized into groups, or shards. 
> Shards are ephemeral: They are created and deleted automatically, as needed.
> Any shard can also split into multiple new shards; this also occurs automatically. (It's also possible for a parent shard to have just one child shard.)
> A shard might split in response to high levels of write activity on its parent table, so that applications can process records from multiple shards in parallel.
> Because shards have a lineage (parent and children), an application must always process a parent shard before it processes a child shard. This helps ensure that the stream records are also processed in the correct order.

When starting streaming, we can use [`DescribeStream`] to retrieve a list of active stream shards:

```json
"Shards": [
	{
		"ParentShardId": "string",
		"SequenceNumberRange": {
			"EndingSequenceNumber": "string",
			"StartingSequenceNumber": "string"
		},
		"ShardId": "string"
	}
],
```

> If the `SequenceNumberRange` has a `StartingSequenceNumber` but no `EndingSequenceNumber`, then the shard is still open (able to receive more stream records).
> If both `StartingSequenceNumber` and `EndingSequenceNumber` are present, then that shard is closed and can no longer receive more data.

For each of these shards, we also retrieve a shard iterator  using [`GetShardIterator`], providing the following information:

```json
{
   "SequenceNumber": "string",
   "ShardId": "string",
   "ShardIteratorType": "string",
   "StreamArn": "string"
}
```

We have the following `ShardIteratorType`s:

> - `AT_SEQUENCE_NUMBER` - Start reading exactly from the position denoted by a specific sequence number.
> - `AFTER_SEQUENCE_NUMBER` - Start reading right after the position denoted by a specific sequence number.
> - `TRIM_HORIZON` - Start reading at the last (untrimmed) stream record, which is the oldest record in the shard. In DynamoDB Streams, there is a 24 hour limit on data retention. Stream records whose age exceeds this limit are subject to removal (trimming) from the stream.
> - `LATEST` - Start reading just after the most recent stream record in the shard, so that you always read the most recent data in the shard.

Given a shard id `$ID` (returned by `DescribeStream`), if `streamCursor.ShardIdToSequenceNumber` contains `$ID`, then we set the `ShardIteratorType` to `AFTER_SEQUENCE_NUMBER` and `SequenceNumber` to `streamCursor.ShardIdToSequenceNumber[$ID]`.
Otherwise, we can either set it to `TRIM_HORIZON` or to `LATEST`.
In this case we will set it to `LATEST` to maintain the current behaviour in `shards.go`.

Once we have a shard iterator returned by `GetShardIterator`, we can finally use it to [`GetRecords`] from the stream.

[`dynamodb.Log.SearchEvents`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/events/dynamoevents/dynamoevents.go#L558-L560
[checkpoint key]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/events/dynamoevents/dynamoevents.go#L538-L548
[`DescribeStream`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_DescribeStream.html
[`GetShardIterator`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetShardIterator.html
[`GetRecords`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetRecords.html

#### `lib/backend/dynamo/shards.go`

The steps described above are already implemented in `lib/backend/dynamo/shards.go`.
As already mentioned, the only missing feature is allowing the event stream to be resumed by providing a stream cursor.

In sum, `shards.go` will be refactored so that is supports both streaming needs:
- streaming backend changes to [watchers] (which is how it's used today), and
- streaming audit events when `StreamEvents` is called.

Note that today, in order to stream backend changes, there's a single set of goroutines polling DynamoDB shards even if there are multiple watchers.
However, for audit events, one such set of goroutines will be spawn for each `StreamEvents` call.

Also note that streaming audit events requires that DynamoDB streams are enabled for events, just like they're [enabled for the backend].

[watchers]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/dynamodbbk.go#L550-L553
[enabled for the backend]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/dynamodbbk.go#L297-L301

## Concerns and open questions

- Streaming audit events requires that DynamoDB streams are enabled for events. Should this be configurable or is it okay to enable it for all Teleport users?
- For MAU, the plan is to have `teleport.e` using the `IAuditLog.StreamEvents` API, not the `StreamEvents` Teleport API. However, this API might be useful for users and can also be used to upgrade the event-handler plugin. Do we want to implement the API now or later when strictly needed?
- For MAU, we don't strictly need the order enforced by `shards.go` (i.e. we could process all active shards in parallel, even if their parents have not been processed yet). Instead of refactoring `shards.go`, should we have a separate implementation that doesn't enforce any order? Or should `StreamEvents` have an unordered mode?