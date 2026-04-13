package mockstream

import (
	"context"

	dynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gravitational/trace"
)

func (m *MockDynamoStreamsAPI) DescribeTimeToLive(ctx context.Context, params *dynamodb.DescribeTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.DescribeTimeToLive not implemented")
}
func (m *MockDynamoStreamsAPI) UpdateTimeToLive(ctx context.Context, params *dynamodb.UpdateTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTimeToLiveOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.UpdateTimeToLive not implemented")
}
func (m *MockDynamoStreamsAPI) UpdateTable(ctx context.Context, params *dynamodb.UpdateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTableOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.UpdateTable not implemented")
}
func (m *MockDynamoStreamsAPI) DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.DeleteTable not implemented")
}
func (m *MockDynamoStreamsAPI) UpdateContinuousBackups(ctx context.Context, params *dynamodb.UpdateContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateContinuousBackupsOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.UpdateContinuousBackups not implemented")
}
func (m *MockDynamoStreamsAPI) DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.DescribeContinuousBackups not implemented")
}
func (m *MockDynamoStreamsAPI) BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.BatchWriteItem not implemented")
}
func (m *MockDynamoStreamsAPI) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.DeleteItem not implemented")
}
func (m *MockDynamoStreamsAPI) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.UpdateItem not implemented")
}
func (m *MockDynamoStreamsAPI) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.GetItem not implemented")
}
func (m *MockDynamoStreamsAPI) CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.CreateTable not implemented")
}
func (m *MockDynamoStreamsAPI) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.Query not implemented")
}
func (m *MockDynamoStreamsAPI) TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	return nil, trace.NotImplemented("MockDynamoStreamsAPI.TransactWriteItems not implemented")
}
