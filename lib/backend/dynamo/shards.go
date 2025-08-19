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

func (b *Backend) pollStreams(externalCtx context.Context) error {
	ctx, cancel := context.WithCancel(externalCtx)
	defer cancel()

	streamArn, err := b.findStream(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	b.logger.DebugContext(ctx, "Found latest event stream", "stream_arn", aws.ToString(streamArn))

	set := make(map[string]struct{})
	eventsC := make(chan shardEvent)

	shouldStartPoll := func(shard streamtypes.Shard) bool {
		sid := aws.ToString(shard.ShardId)
		if _, ok := set[sid]; ok {
			// already being polled
			return false
		}
		if _, ok := set[aws.ToString(shard.ParentShardId)]; ok {
			b.logger.Log(ctx, logutils.TraceLevel, "Skipping child shard, still polling parent", "child_shard_id", sid, "parent_shard_id", aws.ToString(shard.ParentShardId))
			// still processing parent
			return false
		}
		return true
	}

	refreshShards := func(init bool) error {
		shards, err := b.collectActiveShards(ctx, streamArn)
		if err != nil {
			return trace.Wrap(err)
		}

		var initC chan error
		if init {
			// first call to  refreshShards requires us to block on shard iterator
			// registration.
			initC = make(chan error, len(shards))
		}

		started := 0
		for i := range shards {
			if !shouldStartPoll(shards[i]) {
				continue
			}
			shardID := aws.ToString(shards[i].ShardId)
			b.logger.Log(ctx, logutils.TraceLevel, "Adding active shard", "shard_id", shardID)
			set[shardID] = struct{}{}
			go b.asyncPollShard(ctx, streamArn, shards[i], eventsC, initC)
			started++
		}

		if init {
			// block on shard iterator registration.
			for range started {
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
				delete(set, event.shardID)
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
	shardIterator, err := b.streams.GetShardIterator(ctx, &dynamodbstreams.GetShardIteratorInput{
		ShardId:           shard.ShardId,
		ShardIteratorType: streamtypes.ShardIteratorTypeLatest,
		StreamArn:         streamArn,
	})

	if initC != nil {
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
	for iterator != nil {
		select {
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		case <-ticker.C:
		}

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
	}

	b.logger.Log(ctx, logutils.TraceLevel, "Shard is closed", "shard_id", shardID)
	return shardClosedError{}
}

// collectActiveShards collects shards
func (b *Backend) collectActiveShards(ctx context.Context, streamArn *string) ([]streamtypes.Shard, error) {
	var out []streamtypes.Shard

	input := &dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	}
	for {
		streamInfo, err := b.streams.DescribeStream(ctx, input)
		if err != nil {
			return nil, convertError(err)
		}
		out = append(out, streamInfo.StreamDescription.Shards...)
		if streamInfo.StreamDescription.LastEvaluatedShardId == nil {
			return filterActiveShards(out), nil
		}
		input.ExclusiveStartShardId = streamInfo.StreamDescription.LastEvaluatedShardId
		select {
		case <-ctx.Done():
			// let the next call deal with the context error
		case <-time.After(200 * time.Millisecond):
			// 10 calls per second with two auths, with ample margin
		}
	}
}

func filterActiveShards(shards []streamtypes.Shard) []streamtypes.Shard {
	var active []streamtypes.Shard
	for i := range shards {
		if shards[i].SequenceNumberRange.EndingSequenceNumber == nil {
			active = append(active, shards[i])
		}
	}
	return active
}

func toOpType(rec streamtypes.Record) (types.OpType, error) {
	switch rec.EventName {
	case streamtypes.OperationTypeInsert, streamtypes.OperationTypeModify:
		return types.OpPut, nil
	case streamtypes.OperationTypeRemove:
		return types.OpDelete, nil
	default:
		return -1, trace.BadParameter("unsupported DynamodDB operation: %v", rec.EventName)
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
