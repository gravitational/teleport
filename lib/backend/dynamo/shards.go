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
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
)

type shardEvent struct {
	events  []backend.Event
	shardID string
	err     error
}

func (b *Backend) asyncPollStreams(ctx context.Context) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: b.RetryPeriod / 10,
		Max:  b.RetryPeriod,
	})
	if err != nil {
		b.Errorf("Bad retry parameters: %v", err)
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
			b.Errorf("Poll streams returned with error: %v.", err)
		}
		b.Debugf("Reloading %v.", retry)
		select {
		case <-retry.After():
			retry.Inc()
		case <-ctx.Done():
			b.Debugf("Closed, returning from asyncPollStreams loop.")
			return nil
		}
	}
}

func (b *Backend) pollStreams(externalCtx context.Context) error {
	ctx, cancel := context.WithCancel(externalCtx)
	defer cancel()

	streamArn, err := b.findStream(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	b.Debugf("Found latest event stream %v.", aws.StringValue(streamArn))

	set := make(map[string]struct{})
	eventsC := make(chan shardEvent)

	shouldStartPoll := func(shard *dynamodbstreams.Shard) bool {
		sid := aws.StringValue(shard.ShardId)
		if _, ok := set[sid]; ok {
			// already being polled
			return false
		}
		if _, ok := set[aws.StringValue(shard.ParentShardId)]; ok {
			b.Tracef("Skipping child shard: %s, still polling parent %s", sid, aws.StringValue(shard.ParentShardId))
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
			shardID := aws.StringValue(shards[i].ShardId)
			b.Tracef("Adding active shard %v.", shardID)
			set[shardID] = struct{}{}
			go b.asyncPollShard(ctx, streamArn, shards[i], eventsC, initC)
			started++
		}

		if init {
			// block on shard iterator registration.
			for i := 0; i < started; i++ {
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

	ticker := time.NewTicker(b.PollStreamPeriod)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventsC:
			if event.err != nil {
				if event.shardID == "" {
					// empty shard IDs in err-variant events are programming bugs and will lead to
					// invalid state.
					b.WithError(err).Warnf("Forcing watch system reset due to empty shard ID on error (this is a bug)")
					return trace.BadParameter("empty shard ID")
				}
				delete(set, event.shardID)
				if event.err != io.EOF {
					b.Debugf("Shard ID %v closed with error: %v, reseting buffers.", event.shardID, event.err)
					return trace.Wrap(event.err)
				}
				b.Tracef("Shard ID %v exited gracefully.", event.shardID)
			} else {
				b.buf.Emit(event.events...)
			}
		case <-ticker.C:
			if err := refreshShards(false); err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			b.Tracef("Context is closing, returning.")
			return nil
		}
	}
}

func (b *Backend) findStream(ctx context.Context) (*string, error) {
	status, err := b.svc.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
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

func (b *Backend) pollShard(ctx context.Context, streamArn *string, shard *dynamodbstreams.Shard, eventsC chan shardEvent, initC chan<- error) error {
	shardIterator, err := b.streams.GetShardIteratorWithContext(ctx, &dynamodbstreams.GetShardIteratorInput{
		ShardId:           shard.ShardId,
		ShardIteratorType: aws.String(dynamodbstreams.ShardIteratorTypeLatest),
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
	shardID := aws.StringValue(shard.ShardId)
	for {
		select {
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		case <-ticker.C:
			out, err := b.streams.GetRecordsWithContext(ctx, &dynamodbstreams.GetRecordsInput{
				ShardIterator: iterator,
			})
			if err != nil {
				return convertError(err)
			}
			if len(out.Records) > 0 {
				b.Tracef("Got %v new stream shard records.", len(out.Records))
			}
			if len(out.Records) == 0 {
				if out.NextShardIterator == nil {
					b.Tracef("Shard is closed: %v.", aws.StringValue(shard.ShardId))
					return io.EOF
				}
				iterator = out.NextShardIterator
				continue
			}
			if out.NextShardIterator == nil {
				b.Tracef("Shard is closed: %v.", aws.StringValue(shard.ShardId))
				return io.EOF
			}
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
			iterator = out.NextShardIterator
		}
	}
}

// collectActiveShards collects shards
func (b *Backend) collectActiveShards(ctx context.Context, streamArn *string) ([]*dynamodbstreams.Shard, error) {
	var out []*dynamodbstreams.Shard

	input := &dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	}
	for {
		streamInfo, err := b.streams.DescribeStreamWithContext(ctx, input)
		if err != nil {
			return nil, convertError(err)
		}
		out = append(out, streamInfo.StreamDescription.Shards...)
		if streamInfo.StreamDescription.LastEvaluatedShardId == nil {
			return filterActiveShards(out), nil
		}
		input.ExclusiveStartShardId = streamInfo.StreamDescription.LastEvaluatedShardId
	}
}

func filterActiveShards(shards []*dynamodbstreams.Shard) []*dynamodbstreams.Shard {
	var active []*dynamodbstreams.Shard
	for i := range shards {
		if shards[i].SequenceNumberRange.EndingSequenceNumber == nil {
			active = append(active, shards[i])
		}
	}
	return active
}

func toOpType(rec *dynamodbstreams.Record) (types.OpType, error) {
	switch aws.StringValue(rec.EventName) {
	case dynamodbstreams.OperationTypeInsert, dynamodbstreams.OperationTypeModify:
		return types.OpPut, nil
	case dynamodbstreams.OperationTypeRemove:
		return types.OpDelete, nil
	default:
		return -1, trace.BadParameter("unsupported DynamodDB operation: %v", aws.StringValue(rec.EventName))
	}
}

func toEvent(rec *dynamodbstreams.Record) (*backend.Event, error) {
	op, err := toOpType(rec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch op {
	case types.OpPut:
		var r record
		if err := dynamodbattribute.UnmarshalMap(rec.Dynamodb.NewImage, &r); err != nil {
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
				ID:       r.ID,
				Revision: r.Revision,
			},
		}, nil
	case types.OpDelete:
		var r record
		if err := dynamodbattribute.UnmarshalMap(rec.Dynamodb.Keys, &r); err != nil {
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

func (b *Backend) asyncPollShard(ctx context.Context, streamArn *string, shard *dynamodbstreams.Shard, eventsC chan shardEvent, initC chan<- error) {
	var err error
	shardID := aws.StringValue(shard.ShardId)
	defer func() {
		if err == nil {
			err = trace.BadParameter("shard %q exited unexpectedly", shardID)
		}
		select {
		case eventsC <- shardEvent{err: err, shardID: shardID}:
		case <-ctx.Done():
			b.Debugf("Context is closing, returning")
			return
		}
	}()
	err = b.pollShard(ctx, streamArn, shard, eventsC, initC)
}
