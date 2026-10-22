// Copyright 2026 Gravitational, Inc.
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

package client

import (
	"context"

	"github.com/gravitational/trace"

	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
)

// LLMUsageClient returns an [LLMUsageClient].
func (c *Client) LLMUsageClient() *LLMUsageClient {
	return NewLLMUsageClient(llmv1.NewUsageServiceClient(c.conn))
}

// LLMUsageClient is a client for the LLM usage API.
type LLMUsageClient struct {
	grpcClient llmv1.UsageServiceClient
}

// NewLLMUsageClient creates a new LLM usage client.
func NewLLMUsageClient(grpcClient llmv1.UsageServiceClient) *LLMUsageClient {
	return &LLMUsageClient{grpcClient: grpcClient}
}

// GetLLMEndpointQuota returns the specified LLMEndpointQuota.
func (l *LLMUsageClient) GetLLMEndpointQuota(ctx context.Context, name string) (*llmv1.LLMEndpointQuota, error) {
	resp, err := l.grpcClient.GetLLMEndpointQuota(ctx, llmv1.GetLLMEndpointQuotaRequest_builder{
		Name: name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetQuota(), trace.Wrap(err)
}

// ListLLMEndpointQuotas lists LLMEndpointQuota resources.
func (l *LLMUsageClient) ListLLMEndpointQuotas(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error) {
	resp, err := l.grpcClient.ListLLMEndpointQuotas(ctx, llmv1.ListLLMEndpointQuotasRequest_builder{
		PageSize:  int32(limit),
		PageToken: startKey,
	}.Build())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.GetQuotas(), resp.GetNextPageToken(), nil
}

// ReportUsage reports an amount of LLM tokens consumed.
func (l *LLMUsageClient) ReportUsage(ctx context.Context, usages []*llmv1.EndpointUsage) (*llmv1.ReportUsageResponse, error) {
	resp, err := l.grpcClient.ReportUsage(ctx, llmv1.ReportUsageRequest_builder{
		Endpoints: usages,
	}.Build())
	return resp, trace.Wrap(err)
}
