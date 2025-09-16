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
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

// APIMetrics wraps a dynamodbiface.DynamoDBAPI implementation and
// reports statistics about the dynamo api operations
type APIMetrics struct {
	dynamodbiface.DynamoDBAPI
	tableType TableType
}

// NewAPIMetrics returns a new APIMetrics for the provided TableType
func NewAPIMetrics(tableType TableType, api dynamodbiface.DynamoDBAPI) (*APIMetrics, error) {
	if err := metrics.RegisterPrometheusCollectors(dynamoCollectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	return &APIMetrics{
		DynamoDBAPI: api,
		tableType:   tableType,
	}, nil
}

func (m *APIMetrics) DescribeTimeToLiveWithContext(ctx context.Context, input *dynamodb.DescribeTimeToLiveInput, opts ...request.Option) (*dynamodb.DescribeTimeToLiveOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.DescribeTimeToLiveWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "describe_ttl", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) UpdateTimeToLiveWithContext(ctx context.Context, input *dynamodb.UpdateTimeToLiveInput, opts ...request.Option) (*dynamodb.UpdateTimeToLiveOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.UpdateTimeToLiveWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "update_ttl", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) DeleteItemWithContext(ctx context.Context, input *dynamodb.DeleteItemInput, opts ...request.Option) (*dynamodb.DeleteItemOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.DeleteItemWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "delete_item", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) GetItemWithContext(ctx context.Context, input *dynamodb.GetItemInput, opts ...request.Option) (*dynamodb.GetItemOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.GetItemWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "get_item", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) PutItemWithContext(ctx context.Context, input *dynamodb.PutItemInput, opts ...request.Option) (*dynamodb.PutItemOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.PutItemWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "put_item", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) UpdateItemWithContext(ctx context.Context, input *dynamodb.UpdateItemInput, opts ...request.Option) (*dynamodb.UpdateItemOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.UpdateItemWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "update_item", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) DeleteTableWithContext(ctx context.Context, input *dynamodb.DeleteTableInput, opts ...request.Option) (*dynamodb.DeleteTableOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.DeleteTableWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "delete_table", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) BatchWriteItemWithContext(ctx context.Context, input *dynamodb.BatchWriteItemInput, opts ...request.Option) (*dynamodb.BatchWriteItemOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.BatchWriteItemWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "batch_write_item", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) ScanWithContext(ctx context.Context, input *dynamodb.ScanInput, opts ...request.Option) (*dynamodb.ScanOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.ScanWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "scan", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) CreateTableWithContext(ctx context.Context, input *dynamodb.CreateTableInput, opts ...request.Option) (*dynamodb.CreateTableOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.CreateTableWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "create_table", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) DescribeTableWithContext(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...request.Option) (*dynamodb.DescribeTableOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.DescribeTableWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "describe_table", err, time.Since(start).Seconds())

	return output, err
}

func (m *APIMetrics) QueryWithContext(ctx context.Context, input *dynamodb.QueryInput, opts ...request.Option) (*dynamodb.QueryOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBAPI.QueryWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "query", err, time.Since(start).Seconds())

	return output, err
}
