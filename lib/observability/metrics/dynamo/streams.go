// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamo

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams/dynamodbstreamsiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// StreamsMetricsAPI wraps a dynamodbstreamsiface.DynamoDBStreamsAPI implementation and
// reports statistics about the dynamo api operations
type StreamsMetricsAPI struct {
	dynamodbstreamsiface.DynamoDBStreamsAPI
	tableType TableType
}

// NewStreamsMetricsAPI returns a new StreamsMetricsAPI for the provided TableType
func NewStreamsMetricsAPI(tableType TableType, api dynamodbstreamsiface.DynamoDBStreamsAPI) (*StreamsMetricsAPI, error) {
	if err := utils.RegisterPrometheusCollectors(dynamoCollectors...); err != nil {
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
