// Copyright 2023 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package externalauditstorage

import (
	"context"

	"github.com/gravitational/trace"

	externalauditstoragev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	conv "github.com/gravitational/teleport/api/types/externalauditstorage/convert/v1"
)

// Client is an External Audit Storage client.
type Client struct {
	grpcClient externalauditstoragev1.ExternalAuditStorageServiceClient
}

// NewClient creates a new ExternalAuditStorage client.
func NewClient(grpcClient externalauditstoragev1.ExternalAuditStorageServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// TestDraftExternalAuditStorageBuckets tests the connection to the current draft buckets.
func (c *Client) TestDraftExternalAuditStorageBuckets(ctx context.Context) error {
	_, err := c.grpcClient.TestDraftExternalAuditStorageBuckets(ctx, &externalauditstoragev1.TestDraftExternalAuditStorageBucketsRequest{})
	return trace.Wrap(err)
}

// TestDraftExternalAuditStorageGlue tests the configuration to the current draft glue table and database.
func (c *Client) TestDraftExternalAuditStorageGlue(ctx context.Context) error {
	_, err := c.grpcClient.TestDraftExternalAuditStorageGlue(ctx, &externalauditstoragev1.TestDraftExternalAuditStorageGlueRequest{})
	return trace.Wrap(err)
}

// TestDraftExternalAuditStorageAthena tests the configuration to the current draft athena.
func (c *Client) TestDraftExternalAuditStorageAthena(ctx context.Context) error {
	_, err := c.grpcClient.TestDraftExternalAuditStorageAthena(ctx, &externalauditstoragev1.TestDraftExternalAuditStorageAthenaRequest{})
	return trace.Wrap(err)
}

// GetDraftExternalAuditStorage returns the draft External Audit Storage configuration resource.
func (c *Client) GetDraftExternalAuditStorage(ctx context.Context) (*externalauditstorage.ExternalAuditStorage, error) {
	resp, err := c.grpcClient.GetDraftExternalAuditStorage(ctx, &externalauditstoragev1.GetDraftExternalAuditStorageRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := conv.FromProtoDraft(resp.GetExternalAuditStorage())
	return externalAudit, trace.Wrap(err)
}

// CreateDraftExternalAuditStorage creates a draft External Audit Storage
// resource if one does not already exist.
func (c *Client) CreateDraftExternalAuditStorage(ctx context.Context, in *externalauditstorage.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	resp, err := c.grpcClient.CreateDraftExternalAuditStorage(ctx, &externalauditstoragev1.CreateDraftExternalAuditStorageRequest{
		ExternalAuditStorage: conv.ToProto(in),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := conv.FromProtoDraft(resp.GetExternalAuditStorage())
	return out, trace.Wrap(err)
}

// UpsertDraftExternalAuditStorage upserts a draft External Audit Storage resource.
func (c *Client) UpsertDraftExternalAuditStorage(ctx context.Context, in *externalauditstorage.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	resp, err := c.grpcClient.UpsertDraftExternalAuditStorage(ctx, &externalauditstoragev1.UpsertDraftExternalAuditStorageRequest{
		ExternalAuditStorage: conv.ToProto(in),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := conv.FromProtoDraft(resp.GetExternalAuditStorage())
	return out, trace.Wrap(err)
}

// GenerateDraftExternalAuditStorage create a new draft External Audit Storage
// resource with randomized resource names and upserts it as the current
// draft.
func (c *Client) GenerateDraftExternalAuditStorage(ctx context.Context, integrationName, region string) (*externalauditstorage.ExternalAuditStorage, error) {
	resp, err := c.grpcClient.GenerateDraftExternalAuditStorage(ctx, &externalauditstoragev1.GenerateDraftExternalAuditStorageRequest{
		IntegrationName: integrationName,
		Region:          region,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := conv.FromProtoDraft(resp.GetExternalAuditStorage())
	return out, trace.Wrap(err)
}

// DeleteDraftExternalAuditStorage removes draft External Audit Storage resource.
func (c *Client) DeleteDraftExternalAuditStorage(ctx context.Context) error {
	_, err := c.grpcClient.DeleteDraftExternalAuditStorage(ctx, &externalauditstoragev1.DeleteDraftExternalAuditStorageRequest{})
	return trace.Wrap(err)
}

// PromoteToClusterExternalAuditStorage promotes the current draft External
// Audit Storage configuration to be used in the cluster.
func (c *Client) PromoteToClusterExternalAuditStorage(ctx context.Context) error {
	_, err := c.grpcClient.PromoteToClusterExternalAuditStorage(ctx, &externalauditstoragev1.PromoteToClusterExternalAuditStorageRequest{})
	return trace.Wrap(err)
}

// GetClusterExternalAuditStorage gets cluster External Audit Storage.
func (c *Client) GetClusterExternalAuditStorage(ctx context.Context) (*externalauditstorage.ExternalAuditStorage, error) {
	resp, err := c.grpcClient.GetClusterExternalAuditStorage(ctx, &externalauditstoragev1.GetClusterExternalAuditStorageRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := conv.FromProtoCluster(resp.GetClusterExternalAuditStorage())
	return externalAudit, trace.Wrap(err)
}

// DisableClusterExternalAuditStorage disables the External Audit Storage feature,
// which means default cloud audit will be used.
func (c *Client) DisableClusterExternalAuditStorage(ctx context.Context) error {
	_, err := c.grpcClient.DisableClusterExternalAuditStorage(ctx, &externalauditstoragev1.DisableClusterExternalAuditStorageRequest{})
	return trace.Wrap(err)
}
