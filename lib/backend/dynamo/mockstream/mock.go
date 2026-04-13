package mockstream

import (
	"context"
	"iter"
	"maps"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/gravitational/trace"
)

const getRecordsLimit = 1000
const fullPathAttrName = "FullPath"

// MockDynamoStreamsAPI is a mock implementation of the DynamoDB Streams API and a small subset of DynamoDB API, intended for testing the Backend's stream
// processing logic without making real AWS calls.
// - Partitions and hash keys are not supported.
// - GetRecords size limits are not supported.
// - Shard splits are controlled by the calling test.
type MockDynamoStreamsAPI struct {
	Config

	mu     sync.Mutex // Protects below
	shards map[string]*shard

	hooks struct {
		mu                   sync.Mutex // Protects below
		DescribeStreamHook   func(ctx context.Context, input *dynamodbstreams.DescribeStreamInput) error
		GetShardIteratorHook func(ctx context.Context, input *dynamodbstreams.GetShardIteratorInput) error
		GetRecordsHook       func(ctx context.Context, input *dynamodbstreams.GetRecordsInput) error
	}
}

type Config struct {
	StreamArn string
	TableName string
}

func NewMockDynamoStreamsAPI(ctx context.Context, cfg Config) *MockDynamoStreamsAPI {
	if cfg.StreamArn == "" {
		cfg.StreamArn = "mock-stream-arn"
	}
	if cfg.TableName == "" {
		cfg.TableName = "mock-table"
	}

	m := &MockDynamoStreamsAPI{
		shards: make(map[string]*shard),
		Config: cfg,
	}

	root := &shard{
		id:    newShardID(),
		start: 0,
		end:   ^uint64(0),
	}
	m.shards[root.id] = root
	return m
}

func (m *MockDynamoStreamsAPI) routeShard(hash uint64) *shard {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.shards {
		if s.writtable(hash) {
			return s
		}
	}
	return nil
}

func (m *MockDynamoStreamsAPI) getShard(id string) *shard {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shards[id]
}

func (m *MockDynamoStreamsAPI) GetShards() iter.Seq[string] {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Keys(m.shards)
}

func (m *MockDynamoStreamsAPI) GetShardRecords(id string) ([]streamtypes.Record, error) {
	sh := m.getShard(id)
	if sh == nil {
		return nil, trace.NotFound("shard %q not found", id)
	}

	recs, _ := sh.getRecords(0, 0)
	return recs, nil
}

func (m *MockDynamoStreamsAPI) SplitShard(id string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sh := m.shards[id]
	if sh == nil {
		return nil, trace.NotFound("shard %q not found", id)
	}

	children, err := sh.split()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ids []string

	for _, child := range children {
		m.shards[child.id] = child
		ids = append(ids, child.id)
	}

	return ids, nil
}

func (m *MockDynamoStreamsAPI) SpawnChild(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sh := m.shards[id]
	if sh == nil {
		return "", trace.NotFound("shard %q not found", id)
	}

	child, err := sh.spawn()
	if err != nil {
		return "", trace.Wrap(err)
	}

	m.shards[child.id] = child
	return child.id, nil
}

func (m *MockDynamoStreamsAPI) Close() {

}
