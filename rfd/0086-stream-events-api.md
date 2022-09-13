---
authors: Vitor Enes (vitor@goteleport.com)
state: draft
---

# RFD 86 - Stream Audit Events

## Required Approvals

* Engineering: @r0mant && @jimbishopp

## Table of Contents

* [What](#what)
* [Why](#why)
  * [Goals](#goals)
  * [Non\-Goals](#non-goals)
* [Details](#details)
  * [Stream any table](#stream-any-table)
  * [Stream resuming](#stream-resuming)
	* [DynamoDB stream cursor](#dynamodb-stream-cursor)
* [Additional notes](#additional-notes)

## What

Currently streaming of audit events has to be implemented on top of [`SearchEvents`], as [done by the event-handler] plugin.
However, this may be implemented more efficiently, specially for databases with [streaming capabilities such as DynamoDB].

This RFD proposes that we refactor the existing DynamoDB streaming implementation in [`lib/backend/dynamo/shards.go`] (which is used to watch for backend changes), so that it can also be used for streaming of audit events.

[`SearchEvents`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/api/client/client.go#L2137
[done by the event-handler]: https://github.com/gravitational/teleport-plugins/blob/142b8bd7127fe680d42fc4d626d18b3fd8df2b49/event-handler/teleport_events_watcher.go#L175-L184
[streaming capabilities such as DynamoDB]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html
[`lib/backend/dynamo/shards.go`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go

## Why

The Cloud team wants to start tracking the number of monthly active users (MAU) in order to understand the usage and growth of Teleport Cloud.
To achieve this, audit events will be anonymized and sent to some time-series database (TBD).
For efficiency, the plan is to implement this by leveraging DynamoDB streaming capabilities.

### Goals
* Allow DynamoDB streaming for any DynamoDB table

### Non-Goals
* Add a new `StreamEvents` API that can be leveraged by the event-handler plugin

## Details

The following changes will be made to the existing streaming implementation in [`lib/backend/dynamo/shards.go`]:
- [Stream any table](#stream-any-table): refactor implementation so that it can stream changes for any DynamoDB table
- [Stream resuming](#stream-resuming): allow resuming the stream given some stream cursor

### Stream any table

Currently there's a [`Backend.asyncPollStreams`] function in `shards.go` that is called by the backend at the [end of its initialization].

[`Backend.asyncPollStreams`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L41-L71
[end of its initialization]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/dynamodbbk.go#L324-L328

This function does the following in a loop:
1. [find stream ARN for backend table](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L77)
2. [wait until the list of shard iterators is known](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L142)
3. [unblock registered watchers](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L147)
4. [receive stream records in a loop](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L155)
	- 4.1. [send stream records to watchers](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L170)
5. [close registered watchers on error](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L148)

Of the above, steps 1., 3., 4.1. and 5. are specific for streaming backend changes.
For this reason, we propose the following:
- step 1. should be generic so that we can stream any table
- step 4.1. should be generic so that we can send backend changes to [watchers] and audit events to the interested party
- steps 3. and 5. are not needed for audit events streaming, so they should be refactored out

[watchers]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/dynamodbbk.go#L550-L553

The above will be achieved by:
- adding new `StreamConfig` struct that contains the `TableName` from where to stream (for step 1.) and a generic `OnStreamRecords` function (for step 4.1.):
```go
type StreamConfig struct {
	// AwsSession is an AWS session
	AwsSession *session.Session
	// PollStreamPeriod is a polling period for event stream
	PollStreamPeriod time.Duration
	// TableName is the DynamoDB table that will be streamed
	TableName string
	// OnStreamRecords is the function to be called when a stream shard returns a set of records
	OnStreamRecords func([]*dynamodbstreams.Record) error
}
```
- adding new `NewStream` function that does steps 1. (using `TableName`) and 2.
- adding new `Poll` function that does step 4. (calling `OnStreamRecords`)
- removing steps 3. and 5. from `shards.go`

Then, the [`Backend.asyncPollStreams`] function would look something like this:
```go
func (b *Backend) asyncPollStreams(ctx context.Context) error {
	cfg := StreamConfig{
		AwsSession:       b.session,
		PollStreamPeriod: b.PollStreamPeriod,
		TableName:        b.TableName,
		OnStreamRecords: func(records []*dynamodbstreams.Record) error {
			events := make([]backend.Event, 0, len(records))
			for i := range records {
				event, err := toEvent(records[i])
				if err != nil {
					return trace.Wrap(err)
				}
				events = append(events, *event)
			}
			b.buf.Emit(events...)
			return nil
		},
	}

	for {
		// there's no stream resuming for backend changes so the stream cursor is empty
		cursor := ""

		stream, err := NewStream(ctx, cfg) // steps 1. and 2.
		if err == nil {
			b.buf.SetInit()                // step 3.
			err = stream.Poll(ctx, cursor) // steps 4 and 4.1
			b.buf.Reset()                  // step 5.
		}

		// (removed error handling/retries for brevity)
	}
}
```

(Note that, for brevity, the above does not include the [retries after stream errors and exits after the backend closes](https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L53-L69).)

### Stream resuming

For streaming of audit events (and tracking of MAU) we have to ensure that all events are eventually streamed, meaning that stream resuming should be possible after an error occurs.
This will be achieved by passing a stream cursor (detailed [below](#dynamodb-stream-cursor)) to the `Poll` function from above.

The current streaming implementation does not support stream resuming since, upon an error or a server restart, the backend starts streaming from the [`LATEST`] event in each active shard (which is why the stream cursor is empty in the `Backend.asyncPollStreams` function above).

[`LATEST`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/shards.go#L199

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
In this case, we will set it to `LATEST` to maintain the current behaviour in `shards.go`.

Once we have a shard iterator returned by `GetShardIterator`, we can finally use it to [`GetRecords`] from the stream.

[`dynamodb.Log.SearchEvents`]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/events/dynamoevents/dynamoevents.go#L558-L560
[checkpoint key]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/events/dynamoevents/dynamoevents.go#L538-L548
[`DescribeStream`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_DescribeStream.html
[`GetShardIterator`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetShardIterator.html
[`GetRecords`]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetRecords.html

The steps described above are already implemented in [`lib/backend/dynamo/shards.go`].
The missing feature is allowing the stream to be resumed by providing a stream cursor.

### Additional notes
- Streaming of audit events requires that DynamoDB streams are enabled for the events table (just like they're [enabled for the backend]).
As this is only needed for Teleport Cloud, the tenant-operator will be responsible for enabling this.
- There's a [limit on the number of simultaneous readers] of stream shard. This limit is 2, which is enough for MAU as we plan to stream audit events from a single process.
- For MAU, we don't strictly need the order enforced by `shards.go` (i.e. we could process all active shards in parallel, even if their parents have not been processed yet). So instead of refactoring `shards.go`, we could have a separate implementation that doesn't enforce any order.

[enabled for the backend]: https://github.com/gravitational/teleport/blob/cf205b01a5aa88fd4fcdb499ff9b9c40c4e5c335/lib/backend/dynamo/dynamodbbk.go#L297-L301
[limit on the number of simultaneous readers]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ServiceQuotas.html#limits-dynamodb-streams-simultaneous-shard-readers

