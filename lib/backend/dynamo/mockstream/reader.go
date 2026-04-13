package mockstream

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"

	"github.com/gravitational/trace"
)

func (m *MockDynamoStreamsAPI) DescribeStream(ctx context.Context, params *dynamodbstreams.DescribeStreamInput, optFns ...func(*dynamodbstreams.Options)) (*dynamodbstreams.DescribeStreamOutput, error) {
	if err := m.maybeDescribeStreamHook(ctx, params); err != nil {
		return nil, err
	}

	if params.StreamArn == nil || *params.StreamArn != m.StreamArn {
		return nil, trace.NotFound("stream %q not found", m.StreamArn)
	}

	var shards []streamtypes.Shard

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sh := range m.shards {
		shards = append(shards, sh.toStreamType())
	}

	return &dynamodbstreams.DescribeStreamOutput{
		StreamDescription: &streamtypes.StreamDescription{
			StreamArn: params.StreamArn,
			Shards:    shards,
		},
	}, nil
}

func (m *MockDynamoStreamsAPI) GetShardIterator(ctx context.Context, params *dynamodbstreams.GetShardIteratorInput, optFns ...func(*dynamodbstreams.Options)) (*dynamodbstreams.GetShardIteratorOutput, error) {
	if err := m.maybeGetShardIteratorHook(ctx, params); err != nil {
		return nil, err
	}

	if params.StreamArn == nil || *params.StreamArn != m.StreamArn {
		return nil, trace.NotFound("stream not found")
	}

	if params.ShardId == nil {
		return nil, trace.BadParameter("ShardId missing")
	}

	sh := m.getShard(*params.ShardId)
	if sh == nil {
		return nil, trace.NotFound("shard %q not found", *params.ShardId)
	}

	switch params.ShardIteratorType {
	case streamtypes.ShardIteratorTypeAfterSequenceNumber,
		streamtypes.ShardIteratorTypeAtSequenceNumber:
		if params.SequenceNumber == nil {
			return nil, trace.BadParameter("sequence number required")
		}
	}

	iter, err := sh.getIter(params.ShardIteratorType, aws.ToString(params.SequenceNumber))
	if err != nil {
		return nil, err
	}

	return &dynamodbstreams.GetShardIteratorOutput{ShardIterator: iter}, nil
}

func (m *MockDynamoStreamsAPI) GetRecords(ctx context.Context, params *dynamodbstreams.GetRecordsInput, optFns ...func(*dynamodbstreams.Options)) (*dynamodbstreams.GetRecordsOutput, error) {
	if err := m.maybeGetRecordsHook(ctx, params); err != nil {
		return nil, err
	}

	if params.ShardIterator == nil {
		return nil, trace.BadParameter("ShardIterator missing")
	}

	state, err := decodeIterator(*params.ShardIterator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sh := m.getShard(state.ShardID)
	if sh == nil {
		return nil, trace.NotFound("shard %q not found", state.ShardID)
	}

	// enforce parent consumption rule
	if sh.parent != nil {
		parent := m.getShard(*sh.parent)
		if parent == nil {
			return nil, trace.BadParameter("parent %q does not exist for shard %q (this is a bug)", *sh.parent, sh.id)
		}

		if !parent.isClosed() {
			return nil, trace.CompareFailed(
				"cannot read shard %q before parent %q is fully consumed",
				sh.id, *sh.parent,
			)
		}
	}

	limit := int(aws.ToInt32(params.Limit))
	if limit == 0 || limit > getRecordsLimit {
		limit = getRecordsLimit
	}

	recs, next := sh.getRecords(state.Index, limit)
	return &dynamodbstreams.GetRecordsOutput{
		Records:           recs,
		NextShardIterator: next,
	}, nil
}

func (m *MockDynamoStreamsAPI) ListStreams(ctx context.Context, params *dynamodbstreams.ListStreamsInput, optFns ...func(*dynamodbstreams.Options)) (*dynamodbstreams.ListStreamsOutput, error) {
	return &dynamodbstreams.ListStreamsOutput{
		Streams: []types.Stream{
			{
				StreamArn: aws.String(m.StreamArn),
			},
		},
	}, nil
}

type iteratorState struct {
	ShardID string
	Index   int
}

func encodeIterator(s iteratorState) *string {
	b, _ := json.Marshal(s)
	return aws.String(base64.StdEncoding.EncodeToString(b))
}

func decodeIterator(it string) (iteratorState, error) {
	var s iteratorState

	b, err := base64.StdEncoding.DecodeString(it)
	if err != nil {
		return s, trace.BadParameter("invalid iterator encoding")
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return s, trace.BadParameter("invalid iterator payload")
	}

	return s, nil
}
