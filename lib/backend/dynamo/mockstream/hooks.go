package mockstream

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
)

func (m *MockDynamoStreamsAPI) SetDescribeStreamHook(fn func(ctx context.Context, input *dynamodbstreams.DescribeStreamInput) error) {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	m.hooks.DescribeStreamHook = fn
}

func (m *MockDynamoStreamsAPI) SetGetShardIteratorHook(fn func(ctx context.Context, input *dynamodbstreams.GetShardIteratorInput) error) {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	m.hooks.GetShardIteratorHook = fn
}

func (m *MockDynamoStreamsAPI) SetGetRecordsHook(fn func(ctx context.Context, input *dynamodbstreams.GetRecordsInput) error) {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	m.hooks.GetRecordsHook = fn
}

func (m *MockDynamoStreamsAPI) maybeDescribeStreamHook(ctx context.Context, input *dynamodbstreams.DescribeStreamInput) error {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	if m.hooks.DescribeStreamHook != nil {
		return m.hooks.DescribeStreamHook(ctx, input)
	}
	return nil
}

func (m *MockDynamoStreamsAPI) maybeGetShardIteratorHook(ctx context.Context, input *dynamodbstreams.GetShardIteratorInput) error {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	if m.hooks.GetShardIteratorHook != nil {
		return m.hooks.GetShardIteratorHook(ctx, input)
	}
	return nil
}

func (m *MockDynamoStreamsAPI) maybeGetRecordsHook(ctx context.Context, input *dynamodbstreams.GetRecordsInput) error {
	m.hooks.mu.Lock()
	defer m.hooks.mu.Unlock()
	if m.hooks.GetRecordsHook != nil {
		return m.hooks.GetRecordsHook(ctx, input)
	}
	return nil
}
