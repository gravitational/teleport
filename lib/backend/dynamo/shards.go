/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package dynamo

import (
	"context"
	"errors"
	"iter"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodbstreams/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	iterstream "github.com/gravitational/teleport/lib/itertools/stream"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type shardEvent struct {
	err     error
	shardID string
	events  []backend.Event
}

func (b *Backend) asyncPollStreams(ctx context.Context) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: b.RetryPeriod / 10,
		Max:  b.RetryPeriod,
	})
	if err != nil {
		b.logger.ErrorContext(ctx, "Bad retry parameters", "error", err)
		return trace.Wrap(err)
	}

	for {
		err := b.pollStreams(ctx)
		if err != nil {
			// this is optimization to avoid extra logging
			// and extra checks, the code path could end up
			// in ctx.Done() select condition below
			if b.isClosed() {
				return trace.Wrap(err)
			}
			b.logger.ErrorContext(ctx, "Poll streams returned with error", "error", err)
		}
		b.logger.DebugContext(ctx, "Reloading", "retry_duration", retry.Duration())
		select {
		case <-retry.After():
			retry.Inc()
		case <-ctx.Done():
			b.logger.DebugContext(ctx, "Closed, returning from asyncPollStreams loop.")
			return nil
		}
	}
}

type shardClosedError struct{}

func (shardClosedError) Error() string {
	return "shard closed"
}

// deleteShardsWithParents removes any shards that have a parent shard also present in the list.
// In a rare case where both a parent and child shard are present in the list of active shards, we want to ignore the child
// shard until the parent shard is closed and removed from the list.
func (b *Backend) deleteShardsWithParents(ctx context.Context, shards []streamtypes.Shard) []streamtypes.Shard {
	shardSet := make(map[string]struct{}, len(shards))
	for _, shard := range shards {
		if shard.ShardId != nil {
			shardSet[aws.ToString(shard.ShardId)] = struct{}{}
		}
	}

	return slices.DeleteFunc(shards, func(s streamtypes.Shard) bool {
		if s.ParentShardId == nil {
			return false
		}
		_, exists := shardSet[aws.ToString(s.ParentShardId)]
		if exists {
			b.logger.DebugContext(ctx, "Skipping shard with parent shard in active list", "shard_id", aws.ToString(s.ShardId), "parent_shard_id", aws.ToString(s.ParentShardId))
		}
		return exists // delete if parent is present
	})
}

func (b *Backend) pollStreams(externalCtx context.Context) error {
	ctx, cancel := context.WithCancel(externalCtx)
	defer cancel()

	streamArn, err := b.findStream(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	b.logger.DebugContext(ctx, "Found latest event stream", "stream_arn", aws.ToString(streamArn))

	set := make(map[string]bool)
	// The set uses sentinel values to mark when a state is ready for deletion. The shard is only removed
	// once DynamoDB confirms the shard is closed to avoid reopenning the shard and processing duplicate events.
	const shardReadyForDeletion = true
	const shardActive = false
	eventsC := make(chan shardEvent)

	filterShards := func(shard streamtypes.Shard) (streamtypes.Shard, bool) {
		closed := shard.SequenceNumberRange.EndingSequenceNumber != nil
		processed, known := set[aws.ToString(shard.ShardId)]
		_, parentPending := set[aws.ToString(shard.ParentShardId)]

		if closed && known && processed {
			// Cleanup of any tombstoned shards.
			b.logger.DebugContext(ctx, "Cleaning up closed shard", "shard_id", aws.ToString(shard.ShardId))
			delete(set, aws.ToString(shard.ShardId))
		} else if parentPending && !closed && !known {
			b.logger.Log(ctx, logutils.TraceLevel, "Not starting poll for shard with known parent until parent is closed", "shard_id", aws.ToString(shard.ShardId), "parent_shard_id", aws.ToString(shard.ParentShardId))
		}

		// For a shard to be a valid candidate for processing it must:
		// 1. Not be closed
		// 2. Not already be known
		// 3. Must not have a parent shard that is still active
		return shard, !closed && !known && !parentPending
	}

	refreshShards := func(init bool) error {
		candidateShards, err := iterstream.Collect(iterstream.FilterMap(b.fetchShards(ctx, streamArn), filterShards))
		if err != nil {
			return trace.Wrap(err)
		}

		// In the case where both a parent and child shard are present in the list of active shards
		// we want to ignore the child because the set marker may not be set yet for the parent shard.
		shards := b.deleteShardsWithParents(ctx, candidateShards)

		var initC chan error
		if init {
			// first call to  refreshShards requires us to block on shard iterator
			// registration.
			initC = make(chan error, len(shards))
		}

		for _, shard := range shards {
			shardID := aws.ToString(shard.ShardId)
			b.logger.DebugContext(ctx, "Adding active shard", "shard_id", shardID)
			set[shardID] = shardActive
			go b.asyncPollShard(ctx, streamArn, shard, eventsC, initC)
		}

		if init {
			// block on shard iterator registration.
			for range shards {
				select {
				case err = <-initC:
					if err != nil {
						return trace.Wrap(err)
					}
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				}
			}
		}

		return nil
	}

	if err := refreshShards(true); err != nil {
		return trace.Wrap(err)
	}

	// shard iterators are initialized, unblock any registered watchers
	b.buf.SetInit()
	defer b.buf.Reset()

	timer := time.NewTimer(b.PollStreamPeriod)
	defer timer.Stop()

	for {
		select {
		case event := <-eventsC:
			if event.err != nil {
				if event.shardID == "" {
					// empty shard IDs in err-variant events are programming bugs and will lead to
					// invalid state.
					b.logger.WarnContext(ctx, "Forcing watch system reset due to empty shard ID on error (this is a bug)", "error", err)
					return trace.BadParameter("empty shard ID")
				}
				set[event.shardID] = shardReadyForDeletion
				if !errors.Is(event.err, shardClosedError{}) {
					b.logger.DebugContext(ctx, "Shard closed with error, resetting buffers.", "shard_id", event.shardID, "error", event.err)
					return trace.Wrap(event.err)
				}
				b.logger.Log(ctx, logutils.TraceLevel, "Shard exited gracefully.", "shard_id", event.shardID)
			} else {
				b.buf.Emit(event.events...)
			}
		case <-timer.C:
			if err := refreshShards(false); err != nil {
				return trace.Wrap(err)
			}
			timer.Reset(b.PollStreamPeriod)
		case <-ctx.Done():
			b.logger.Log(ctx, logutils.TraceLevel, "Context is closing, returning.")
			return nil
		}
	}
}

func (b *Backend) findStream(ctx context.Context) (*string, error) {
	status, err := b.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(b.TableName),
	})
	if err != nil {
		return nil, trace.Wrap(convertError(err))
	}
	if status.Table.LatestStreamArn == nil {
		return nil, trace.NotFound("No streams found for table %v", b.TableName)
	}
	return status.Table.LatestStreamArn, nil
}

func (b *Backend) pollShard(ctx context.Context, streamArn *string, shard streamtypes.Shard, eventsC chan shardEvent, initC chan<- error) error {
	isInitializing := initC != nil

	var req *dynamodbstreams.GetShardIteratorInput
	if isInitializing {
		// On initialization register iterators at LATEST to avoid processing old events.
		req = &dynamodbstreams.GetShardIteratorInput{
			ShardId:           shard.ShardId,
			ShardIteratorType: streamtypes.ShardIteratorTypeLatest,
			StreamArn:         streamArn,
		}
	} else if shard.SequenceNumberRange != nil && shard.SequenceNumberRange.StartingSequenceNumber != nil {
		// For new shards found after initialization, register at the starting sequence number
		// unlike TRIM_HORIZON this should error if the events have been trimmed for some reason and cause
		// a watcher reset. This is not expected to ever happen under normal circumstances.
		req = &dynamodbstreams.GetShardIteratorInput{
			ShardId:           shard.ShardId,
			ShardIteratorType: streamtypes.ShardIteratorTypeAtSequenceNumber,
			StreamArn:         streamArn,
			SequenceNumber:    shard.SequenceNumberRange.StartingSequenceNumber,
		}
	} else {
		// If the shard is missing a valid starting sequence number, something is very wrong. Error out and trigger a reset.
		return trace.BadParameter("shard %q missing valid starting sequence number", aws.ToString(shard.ShardId))
	}

	shardIterator, err := b.streams.GetShardIterator(ctx, req)
	if err == nil && shardIterator.ShardIterator == nil {
		// In both cases for either new init calls or shard split we expect a valid iterator. For LATEST calls on a shard that may be
		// closed the iterator is still valid but will return no records. For AT_SEQUENCE_NUMBER the iterator will always start at the beginning
		// of the shard. Check defensively and error out which will trigger a retry.
		err = trace.BadParameter("expected valid iterator for shard %q, got nil", aws.ToString(shard.ShardId))
	}

	if isInitializing {
		select {
		case initC <- convertError(err):
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		}
	}
	if err != nil {
		return convertError(err)
	}

	ticker := time.NewTicker(b.PollStreamPeriod)
	defer ticker.Stop()
	iterator := shardIterator.ShardIterator
	shardID := aws.ToString(shard.ShardId)

	for {
		out, err := b.streams.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
			ShardIterator: iterator,
		})
		if err != nil {
			return convertError(err)
		}

		if len(out.Records) > 0 {
			b.logger.Log(ctx, logutils.TraceLevel, "Got new stream shard records.", "shard_id", shardID, "num_records", len(out.Records))

			events := make([]backend.Event, 0, len(out.Records))
			for i := range out.Records {
				event, err := toEvent(out.Records[i])
				if err != nil {
					return trace.Wrap(err)
				}
				events = append(events, *event)
			}
			select {
			case <-ctx.Done():
				return trace.ConnectionProblem(ctx.Err(), "context is closing")
			case eventsC <- shardEvent{shardID: shardID, events: events}:
			}
		}

		iterator = out.NextShardIterator
		if iterator == nil {
			// Fast exit if the shard is closed.
			b.logger.DebugContext(ctx, "Done processing shard", "shard_id", shardID)
			return shardClosedError{}
		}

		select {
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		case <-ticker.C:
			// TODO(okraport): for very low values of [PollStreamPeriod], it's possible that [GetRecords] is called
			// each tick even if previous call returns 0 items. The default poll period of 1000ms is sufficient to mitigate
			// this but we may wish to change this behavior to backoff for longer periods of time if no records are returned.
			// This is especially true when there are large number of shards and low writes after a period of high throughput.
			// Similairly avoiding sleeping when records are returned may also be desirable to improve latency.
		}
	}
}

func (b *Backend) fetchShards(ctx context.Context, streamArn *string) iter.Seq2[streamtypes.Shard, error] {
	return func(yield func(streamtypes.Shard, error) bool) {
		input := &dynamodbstreams.DescribeStreamInput{
			StreamArn: streamArn,
		}
		for {
			streamInfo, err := b.streams.DescribeStream(ctx, input)
			if err != nil {
				yield(*new(streamtypes.Shard), trace.Wrap(convertError(err)))
				return
			}

			for _, shard := range streamInfo.StreamDescription.Shards {
				if !yield(shard, nil) {
					return
				}
			}

			input.ExclusiveStartShardId = streamInfo.StreamDescription.LastEvaluatedShardId
			if input.ExclusiveStartShardId == nil {
				return
			}

			select {
			case <-ctx.Done():
				// let the next call deal with the context error
			case <-time.After(200 * time.Millisecond):
				// 10 calls per second with two auths, with ample margin
			}
		}

	}

}

func toOpType(rec streamtypes.Record) (types.OpType, error) {
	switch rec.EventName {
	case streamtypes.OperationTypeInsert, streamtypes.OperationTypeModify:
		return types.OpPut, nil
	case streamtypes.OperationTypeRemove:
		return types.OpDelete, nil
	default:
		return -1, trace.BadParameter("unsupported DynamoDB operation: %v", rec.EventName)
	}
}

func toEvent(rec streamtypes.Record) (*backend.Event, error) {
	op, err := toOpType(rec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch op {
	case types.OpPut:
		var r record
		if err := attributevalue.UnmarshalMap(rec.Dynamodb.NewImage, &r); err != nil {
			return nil, trace.Wrap(err)
		}
		var expires time.Time
		if r.Expires != nil {
			expires = time.Unix(*r.Expires, 0)
		}
		return &backend.Event{
			Type: op,
			Item: backend.Item{
				Key:      trimPrefix(r.FullPath),
				Value:    r.Value,
				Expires:  expires,
				Revision: r.Revision,
			},
		}, nil
	case types.OpDelete:
		var r record
		if err := attributevalue.UnmarshalMap(rec.Dynamodb.Keys, &r); err != nil {
			return nil, trace.Wrap(err)
		}
		return &backend.Event{
			Type: op,
			Item: backend.Item{
				Key: trimPrefix(r.FullPath),
			},
		}, nil
	default:
		return nil, trace.BadParameter("unsupported operation type: %v", op)
	}
}

func (b *Backend) asyncPollShard(ctx context.Context, streamArn *string, shard streamtypes.Shard, eventsC chan shardEvent, initC chan<- error) {
	var err error
	shardID := aws.ToString(shard.ShardId)
	defer func() {
		if err == nil {
			err = trace.BadParameter("shard %q exited unexpectedly", shardID)
		}
		select {
		case eventsC <- shardEvent{err: err, shardID: shardID}:
		case <-ctx.Done():
			b.logger.DebugContext(ctx, "Context is closing, returning")
			return
		}
	}()
	err = b.pollShard(ctx, streamArn, shard, eventsC, initC)
}
