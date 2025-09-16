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
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams/dynamodbstreamsiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

// StreamsMetricsAPI wraps a dynamodbstreamsiface.DynamoDBStreamsAPI implementation and
// reports statistics about the dynamo api operations
type StreamsMetricsAPI struct {
	dynamodbstreamsiface.DynamoDBStreamsAPI
	tableType TableType
}

// NewStreamsMetricsAPI returns a new StreamsMetricsAPI for the provided TableType
func NewStreamsMetricsAPI(tableType TableType, api dynamodbstreamsiface.DynamoDBStreamsAPI) (*StreamsMetricsAPI, error) {
	if err := metrics.RegisterPrometheusCollectors(dynamoCollectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	return &StreamsMetricsAPI{
		DynamoDBStreamsAPI: api,
		tableType:          tableType,
	}, nil
}

func (m *StreamsMetricsAPI) DescribeStreamWithContext(ctx context.Context, input *dynamodbstreams.DescribeStreamInput, opts ...request.Option) (*dynamodbstreams.DescribeStreamOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBStreamsAPI.DescribeStreamWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "describe_stream", err, time.Since(start).Seconds())

	return output, err
}

func (m *StreamsMetricsAPI) GetShardIteratorWithContext(ctx context.Context, input *dynamodbstreams.GetShardIteratorInput, opts ...request.Option) (*dynamodbstreams.GetShardIteratorOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBStreamsAPI.GetShardIteratorWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "get_shard_iterator", err, time.Since(start).Seconds())

	return output, err
}

func (m *StreamsMetricsAPI) GetRecordsWithContext(ctx context.Context, input *dynamodbstreams.GetRecordsInput, opts ...request.Option) (*dynamodbstreams.GetRecordsOutput, error) {
	start := time.Now()
	output, err := m.DynamoDBStreamsAPI.GetRecordsWithContext(ctx, input, opts...)

	recordMetrics(m.tableType, "get_records", err, time.Since(start).Seconds())

	return output, err
}
