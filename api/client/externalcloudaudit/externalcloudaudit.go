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

package externalcloudaudit

import (
	"context"

	"github.com/gravitational/trace"

	externalcloudauditv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalcloudaudit/v1"
	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	conv "github.com/gravitational/teleport/api/types/externalcloudaudit/convert/v1"
)

// Client is an external cloud audit client that conforms to the following lib/services interfaces:
// * services.Externalcloudaudit
type Client struct {
	grpcClient externalcloudauditv1.ExternalCloudAuditServiceClient
}

func NewClient(grpcClient externalcloudauditv1.ExternalCloudAuditServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetDraftExternalCloudAudit returns the draft external cloud audit configuration resource.
func (c *Client) GetDraftExternalCloudAudit(ctx context.Context) (*externalcloudaudit.ExternalCloudAudit, error) {
	resp, err := c.grpcClient.GetDraftExternalCloudAudit(ctx, &externalcloudauditv1.GetDraftExternalCloudAuditRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := conv.FromProtoDraft(resp.GetExternalCloudAudit())
	return externalAudit, trace.Wrap(err)
}

// UpsertDraftExternalCloudAudit upserts a draft external cloud audit resource.
func (c *Client) UpsertDraftExternalCloudAudit(ctx context.Context, in *externalcloudaudit.ExternalCloudAudit) (*externalcloudaudit.ExternalCloudAudit, error) {
	resp, err := c.grpcClient.UpsertDraftExternalCloudAudit(ctx, &externalcloudauditv1.UpsertDraftExternalCloudAuditRequest{
		ExternalCloudAudit: conv.ToProto(in),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := conv.FromProtoDraft(resp.GetExternalCloudAudit())
	return out, trace.Wrap(err)
}

// DeleteDraftExternalCloudAudit removes draft external cloud audit resource.
func (c *Client) DeleteDraftExternalCloudAudit(ctx context.Context) error {
	_, err := c.grpcClient.DeleteDraftExternalCloudAudit(ctx, &externalcloudauditv1.DeleteDraftExternalCloudAuditRequest{})
	return trace.Wrap(err)
}

// PromoteToClusterExternalCloudAudit promotes the current draft external cloud
// audit configuration to be used in the cluster.
func (c *Client) PromoteToClusterExternalCloudAudit(ctx context.Context) error {
	_, err := c.grpcClient.PromoteToClusterExternalCloudAudit(ctx, &externalcloudauditv1.PromoteToClusterExternalCloudAuditRequest{})
	return trace.Wrap(err)
}

// GetClusterExternalCloudAudit gets cluster external cloud audit.
func (c *Client) GetClusterExternalCloudAudit(ctx context.Context) (*externalcloudaudit.ExternalCloudAudit, error) {
	resp, err := c.grpcClient.GetClusterExternalCloudAudit(ctx, &externalcloudauditv1.GetClusterExternalCloudAuditRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	externalAudit, err := conv.FromProtoCluster(resp.GetClusterExternalCloudAudit())
	return externalAudit, trace.Wrap(err)
}

// DisableClusterExternalCloudAudit disables the external cloud audit feature,
// which means default cloud audit will be used.
func (c *Client) DisableClusterExternalCloudAudit(ctx context.Context) error {
	_, err := c.grpcClient.DisableClusterExternalCloudAudit(ctx, &externalcloudauditv1.DisableClusterExternalCloudAuditRequest{})
	return trace.Wrap(err)
}
