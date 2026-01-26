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

package summarizer

import (
	"context"

	"github.com/gravitational/trace"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// Client wraps the gRPC SummarizerServiceClient to provide a higher-level
// interface that hides the RPC request handling details.
type Client struct {
	grpcClient summarizerv1.SummarizerServiceClient
}

// NewClient creates a new [Client] that wraps a gRPC client.
func NewClient(grpcClient summarizerv1.SummarizerServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetInferenceModel retrieves an existing InferenceModel by name.
func (c *Client) GetInferenceModel(ctx context.Context, name string) (*summarizerv1.InferenceModel, error) {
	resp, err := c.grpcClient.GetInferenceModel(ctx, &summarizerv1.GetInferenceModelRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Model, nil
}

// CreateInferenceModel creates a new InferenceModel.
func (c *Client) CreateInferenceModel(ctx context.Context, model *summarizerv1.InferenceModel) (*summarizerv1.InferenceModel, error) {
	resp, err := c.grpcClient.CreateInferenceModel(ctx, &summarizerv1.CreateInferenceModelRequest{
		Model: model,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Model, nil
}

// UpsertInferenceModel creates a new InferenceModel or updates an existing
// one.
func (c *Client) UpsertInferenceModel(ctx context.Context, model *summarizerv1.InferenceModel) (*summarizerv1.InferenceModel, error) {
	resp, err := c.grpcClient.UpsertInferenceModel(ctx, &summarizerv1.UpsertInferenceModelRequest{
		Model: model,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Model, nil
}

// DeleteInferenceModel deletes an existing InferenceModel by name.
func (c *Client) DeleteInferenceModel(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteInferenceModel(ctx, &summarizerv1.DeleteInferenceModelRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// GetInferenceSecret retrieves an existing InferenceSecret by name.
func (c *Client) GetInferenceSecret(ctx context.Context, name string) (*summarizerv1.InferenceSecret, error) {
	resp, err := c.grpcClient.GetInferenceSecret(ctx, &summarizerv1.GetInferenceSecretRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Secret, nil
}

// CreateInferenceSecret creates a new InferenceSecret.
func (c *Client) CreateInferenceSecret(ctx context.Context, secret *summarizerv1.InferenceSecret) (*summarizerv1.InferenceSecret, error) {
	resp, err := c.grpcClient.CreateInferenceSecret(ctx, &summarizerv1.CreateInferenceSecretRequest{
		Secret: secret,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Secret, nil
}

// UpsertInferenceSecret creates a new InferenceSecret or updates an existing
// one.
func (c *Client) UpsertInferenceSecret(ctx context.Context, secret *summarizerv1.InferenceSecret) (*summarizerv1.InferenceSecret, error) {
	resp, err := c.grpcClient.UpsertInferenceSecret(ctx, &summarizerv1.UpsertInferenceSecretRequest{
		Secret: secret,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Secret, nil
}

// DeleteInferenceSecret deletes an existing InferenceSecret by name.
func (c *Client) DeleteInferenceSecret(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteInferenceSecret(ctx, &summarizerv1.DeleteInferenceSecretRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// GetInferencePolicy retrieves an existing InferencePolicy by name.
func (c *Client) GetInferencePolicy(ctx context.Context, name string) (*summarizerv1.InferencePolicy, error) {
	resp, err := c.grpcClient.GetInferencePolicy(ctx, &summarizerv1.GetInferencePolicyRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Policy, nil
}

// CreateInferencePolicy creates a new InferencePolicy.
func (c *Client) CreateInferencePolicy(ctx context.Context, policy *summarizerv1.InferencePolicy) (*summarizerv1.InferencePolicy, error) {
	resp, err := c.grpcClient.CreateInferencePolicy(ctx, &summarizerv1.CreateInferencePolicyRequest{
		Policy: policy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Policy, nil
}

// UpsertInferencePolicy creates a new InferencePolicy or updates an existing
// one.
func (c *Client) UpsertInferencePolicy(ctx context.Context, policy *summarizerv1.InferencePolicy) (*summarizerv1.InferencePolicy, error) {
	resp, err := c.grpcClient.UpsertInferencePolicy(ctx, &summarizerv1.UpsertInferencePolicyRequest{
		Policy: policy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Policy, nil
}

// DeleteInferencePolicy deletes an existing InferencePolicy by name.
func (c *Client) DeleteInferencePolicy(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteInferencePolicy(ctx, &summarizerv1.DeleteInferencePolicyRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
