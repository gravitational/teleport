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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams/dynamodbstreamsiface"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type shardEvent struct {
	records []*dynamodbstreams.Record
	shardID string
	err     error
}

type StreamConfig struct {
	*log.Entry
	DynamoDB         dynamodbiface.DynamoDBAPI
	DynamoDBStreams  dynamodbstreamsiface.DynamoDBStreamsAPI
	PollStreamPeriod time.Duration
	TableName        string
	OnStreamRecords  func([]*dynamodbstreams.Record) error
}

// Check is a helper returns an error if there's any missing configuration.
func (cfg *StreamConfig) Check() error {
	if cfg.DynamoDB == nil {
		return trace.BadParameter("Shards: missing DynamoDB")
	}

	if cfg.DynamoDBStreams == nil {
		return trace.BadParameter("Shards: missing DynamoDStreams ")
	}

	if cfg.PollStreamPeriod == 0 {
		return trace.BadParameter("Shards: missing PollStreamPeriod")
	}

	if cfg.TableName == "" {
		return trace.BadParameter("Shards: missing TableName")
	}

	if cfg.OnStreamRecords == nil {
		return trace.BadParameter("Shards: missing OnStreamRecords")
	}

	return nil
}

type Stream struct {
	StreamConfig
	streamArn string
	shardIds  map[string]struct{}
	eventsC   chan shardEvent
}

func StreamInit(ctx context.Context, cfg StreamConfig) (*Stream, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	streamArn, err := findStream(ctx, cfg.DynamoDB, cfg.TableName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Debugf("Found latest event stream %s.", streamArn)

	s := &Stream{
		StreamConfig: cfg,
		streamArn:    streamArn,
		shardIds:     make(map[string]struct{}),
		eventsC:      make(chan shardEvent),
	}
	if err := s.refreshShards(ctx, true); err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

func (s *Stream) shouldStartPoll(shard *dynamodbstreams.Shard) bool {
	shardId := aws.StringValue(shard.ShardId)
	parentShardId := aws.StringValue(shard.ParentShardId)
	if _, ok := s.shardIds[shardId]; ok {
		// already being polled
		return false
	}
	if _, ok := s.shardIds[parentShardId]; ok {
		s.Debugf("Skipping child shard: %s, still polling parent %s", shardId, parentShardId)
		// still processing parent
		return false
	}
	return true
}

func (s *Stream) refreshShards(ctx context.Context, init bool) error {
	shards, err := s.collectActiveShards(ctx, s.streamArn)
	if err != nil {
		return trace.Wrap(err)
	}

	var initC chan error
	if init {
		// first call to refreshShards requires us to block on shard iterator
		// registration.
		initC = make(chan error, len(shards))
	}

	started := 0
	for i := range shards {
		if !s.shouldStartPoll(shards[i]) {
			continue
		}
		shardID := aws.StringValue(shards[i].ShardId)
		s.Debugf("Adding active shard %v.", shardID)
		s.shardIds[shardID] = struct{}{}
		go s.asyncPollShard(ctx, shards[i], initC)
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

func (s *Stream) Poll(externalCtx context.Context, cursor string) error {
	ctx, cancel := context.WithCancel(externalCtx)
	defer cancel()

	ticker := time.NewTicker(s.PollStreamPeriod)
	defer ticker.Stop()

	for {
		select {
		case event := <-s.eventsC:
			if event.err != nil {
				if event.shardID == "" {
					// empty shard IDs in err-variant events are programming bugs and will lead to
					// invalid state.
					s.WithError(event.err).Warnf("Forcing watch system reset due to empty shard ID on error (this is a bug)")
					return trace.BadParameter("empty shard ID")
				}
				delete(s.shardIds, event.shardID)
				if event.err != io.EOF {
					s.Debugf("Shard ID %v closed with error: %v, resetting buffers.", event.shardID, event.err)
					return trace.Wrap(event.err)
				}
				s.Debugf("Shard ID %v exited gracefully.", event.shardID)
			} else {
				// Q: It seems that there's no checkpointing when streaming changes to the backend.
				if err := s.OnStreamRecords(event.records); err != nil {
					return trace.Wrap(err)
				}
			}
		case <-ticker.C:
			if err := s.refreshShards(ctx, false); err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			s.Debugf("Context is closing, returning.")
			return nil
		}
	}
}

func findStream(ctx context.Context, dynamoDB dynamodbiface.DynamoDBAPI, tableName string) (string, error) {
	status, err := dynamoDB.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return "", trace.Wrap(convertError(err))
	}
	if status.Table.LatestStreamArn == nil {
		return "", trace.NotFound("No streams found for table %v", tableName)
	}

	return aws.StringValue(status.Table.LatestStreamArn), nil
}

func (s *Stream) pollShard(ctx context.Context, shard *dynamodbstreams.Shard, initC chan<- error) error {
	shardIterator, err := s.DynamoDBStreams.GetShardIteratorWithContext(ctx, &dynamodbstreams.GetShardIteratorInput{
		ShardId: shard.ShardId,
		// Q: Besides no checkpointing, the shard iterator type is set to LATEST, meaning that there's no worry about retrieving all events.
		// With checkpointing, we would know the last event retrieved from each (known) shard, and could set the shard iterator type to AFTER_SEQUENCE_NUMBER.
		// If the shard is unknown (i.e. no checkpointing info about it), we should probably set the shard iterator type to TRIM_HORIZON, which can retrieve events up-to 24h old.
		ShardIteratorType: aws.String(dynamodbstreams.ShardIteratorTypeLatest),
		StreamArn:         aws.String(s.streamArn),
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

	ticker := time.NewTicker(s.PollStreamPeriod)
	defer ticker.Stop()
	iterator := shardIterator.ShardIterator
	shardID := aws.StringValue(shard.ShardId)
	for {
		select {
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		case <-ticker.C:
			out, err := s.DynamoDBStreams.GetRecordsWithContext(ctx, &dynamodbstreams.GetRecordsInput{
				ShardIterator: iterator,
			})
			if err != nil {
				return convertError(err)
			}
			if len(out.Records) > 0 {
				s.Debugf("Got %v new stream shard records.", len(out.Records))
			}
			if len(out.Records) == 0 {
				if out.NextShardIterator == nil {
					s.Debugf("Shard is closed: %v.", aws.StringValue(shard.ShardId))
					return io.EOF
				}
				iterator = out.NextShardIterator
				continue
			}
			if out.NextShardIterator == nil {
				s.Debugf("Shard is closed: %v.", aws.StringValue(shard.ShardId))
				return io.EOF
			}
			shardEvent := shardEvent{
				shardID: shardID,
				records: out.Records,
			}
			select {
			case <-ctx.Done():
				return trace.ConnectionProblem(ctx.Err(), "context is closing")
			case s.eventsC <- shardEvent:
			}
			iterator = out.NextShardIterator
		}
	}
}

// collectActiveShards collects shards
func (b *Stream) collectActiveShards(ctx context.Context, streamArn string) ([]*dynamodbstreams.Shard, error) {
	var out []*dynamodbstreams.Shard

	input := &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(streamArn),
	}
	for {
		streamInfo, err := b.DynamoDBStreams.DescribeStreamWithContext(ctx, input)
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
			// from https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_DescribeStream.html:
			// > Each shard in the stream has a SequenceNumberRange associated with it.
			// > If the SequenceNumberRange has a StartingSequenceNumber but no EndingSequenceNumber, then the shard is still open (able to receive more stream records).
			// > If both StartingSequenceNumber and EndingSequenceNumber are present, then that shard is closed and can no longer receive more data.
			//
			// Q: From the above, I don't understand why we're filtering out these shards.
			// If the ending sequence number is non-nil, then shard is closed and can't receive more data.
			// But does that mean that we have polled everything?
			// i don't think so!
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
		return -1, trace.BadParameter("unsupported DynamoDB operation: %v", aws.StringValue(rec.EventName))
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

func (s *Stream) asyncPollShard(ctx context.Context, shard *dynamodbstreams.Shard, initC chan<- error) {
	var err error
	shardID := aws.StringValue(shard.ShardId)
	defer func() {
		if err == nil {
			err = trace.BadParameter("shard %q exited unexpectedly", shardID)
		}
		select {
		case s.eventsC <- shardEvent{err: err, shardID: shardID}:
		case <-ctx.Done():
			s.Debugf("Context is closing, returning")
			return
		}
	}()
	err = s.pollShard(ctx, shard, initC)
}
