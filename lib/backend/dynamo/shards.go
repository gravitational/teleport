/*
Copyright 2015-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamo

import (
	"context"
	"io"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/gravitational/trace"
)

type shardEvent struct {
	events  []backend.Event
	shardID string
	err     error
}

func (b *Backend) asyncPollStreams(ctx context.Context) error {
	retry, err := utils.NewLinear(utils.LinearConfig{
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

	refreshShards := func() error {
		shards, err := b.collectActiveShards(ctx, streamArn)
		if err != nil {
			return trace.Wrap(err)
		}
		for i := range shards {
			shardID := aws.StringValue(shards[i].ShardId)
			if _, ok := set[shardID]; !ok {
				b.Debugf("Adding active shard %v.", shardID)
				set[shardID] = struct{}{}
				go b.asyncPollShard(ctx, streamArn, shards[i], eventsC)
			}
		}
		return nil
	}

	if err := refreshShards(); err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(b.PollStreamPeriod)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventsC:
			if event.err != nil {
				delete(set, event.shardID)
				if event.err != io.EOF {
					b.Debugf("Shard ID %v closed with error: %v, reseting buffers.", event.shardID, event.err)
					return trace.Wrap(event.err)
				}
				b.Debugf("Shard ID %v exited gracefully.", event.shardID)
			} else {
				b.buf.PushBatch(event.events)
			}
		case <-ticker.C:
			if err := refreshShards(); err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			b.Debugf("Context is closing, returning.")
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

func (b *Backend) pollShard(ctx context.Context, streamArn *string, shard *dynamodbstreams.Shard, eventsC chan shardEvent) error {
	shardIterator, err := b.streams.GetShardIteratorWithContext(ctx, &dynamodbstreams.GetShardIteratorInput{
		ShardId:           shard.ShardId,
		ShardIteratorType: aws.String(dynamodbstreams.ShardIteratorTypeLatest),
		StreamArn:         streamArn,
	})
	if err != nil {
		return convertError(err)
	}
	ticker := time.NewTicker(b.PollStreamPeriod)
	defer ticker.Stop()
	iterator := shardIterator.ShardIterator
	shardID := aws.StringValue(shard.ShardId)
	b.signalWatchStart()
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
				b.Debugf("Got %v new stream shard records.", len(out.Records))
			}
			if len(out.Records) == 0 {
				if out.NextShardIterator == nil {
					b.Debugf("Shard is closed: %v.", aws.StringValue(shard.ShardId))
					return io.EOF
				}
				continue
			}
			if out.NextShardIterator == nil {
				b.Debugf("Shard is closed: %v.", aws.StringValue(shard.ShardId))
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
				Key:     trimPrefix(r.FullPath),
				Value:   r.Value,
				Expires: expires,
				ID:      r.ID,
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

func (b *Backend) asyncPollShard(ctx context.Context, streamArn *string, shard *dynamodbstreams.Shard, eventsC chan shardEvent) {
	var err error
	defer func() {
		if err == nil {
			err = trace.BadParameter("shard exited unexpectedly")
		}
		select {
		case eventsC <- shardEvent{err: err}:
		case <-ctx.Done():
			b.Debugf("Context is closing, returning")
			return
		}
	}()
	err = b.pollShard(ctx, streamArn, shard, eventsC)
}

func (b *Backend) turnOnTimeToLive(ctx context.Context) error {
	status, err := b.svc.DescribeTimeToLiveWithContext(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(b.TableName),
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	switch aws.StringValue(status.TimeToLiveDescription.TimeToLiveStatus) {
	case dynamodb.TimeToLiveStatusEnabled, dynamodb.TimeToLiveStatusEnabling:
		return nil
	}
	_, err = b.svc.UpdateTimeToLiveWithContext(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(b.TableName),
		TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
			AttributeName: aws.String(ttlKey),
			Enabled:       aws.Bool(true),
		},
	})
	return convertError(err)
}

func (b *Backend) turnOnStreams(ctx context.Context) error {
	status, err := b.svc.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(b.TableName),
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	if status.Table.StreamSpecification != nil && aws.BoolValue(status.Table.StreamSpecification.StreamEnabled) {
		return nil
	}
	_, err = b.svc.UpdateTableWithContext(ctx, &dynamodb.UpdateTableInput{
		TableName: aws.String(b.TableName),
		StreamSpecification: &dynamodb.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: aws.String(dynamodb.StreamViewTypeNewImage),
		},
	})
	return convertError(err)
}
