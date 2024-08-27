// Copyright 2024 Gravitational, Inc.
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

package autoupdate

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// Client is an AutoupdateService client that conforms to the following lib/services interfaces:
//   - services.AutoupdateService
type Client struct {
	grpcClient autoupdate.AutoupdateServiceClient
}

// NewClient creates a new AutoupdateService client.
func NewClient(grpcClient autoupdate.AutoupdateServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetAutoupdateConfig returns the specified AutoupdateConfig resource.
func (c *Client) GetAutoupdateConfig(ctx context.Context) (*autoupdate.AutoupdateConfig, error) {
	resp, err := c.grpcClient.GetAutoupdateConfig(ctx, &autoupdate.GetAutoupdateConfigRequest{})
	return resp, trace.Wrap(err)
}

// CreateAutoupdateConfig creates a AutoupdateConfig.
func (c *Client) CreateAutoupdateConfig(ctx context.Context, config *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error) {
	resp, err := c.grpcClient.CreateAutoupdateConfig(ctx, &autoupdate.CreateAutoupdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// UpdateAutoupdateConfig updates a AutoupdateConfig.
func (c *Client) UpdateAutoupdateConfig(ctx context.Context, config *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error) {
	resp, err := c.grpcClient.UpdateAutoupdateConfig(ctx, &autoupdate.UpdateAutoupdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// UpsertAutoupdateConfig creates or updates a AutoupdateConfig.
func (c *Client) UpsertAutoupdateConfig(ctx context.Context, config *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error) {
	resp, err := c.grpcClient.UpsertAutoupdateConfig(ctx, &autoupdate.UpsertAutoupdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// DeleteAutoupdateConfig removes the specified AutoupdateConfig resource.
func (c *Client) DeleteAutoupdateConfig(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoupdateConfig(ctx, &autoupdate.DeleteAutoupdateConfigRequest{})
	return trace.Wrap(err)
}

// GetAutoupdateVersion returns the specified AutoupdateVersion resource.
func (c *Client) GetAutoupdateVersion(ctx context.Context) (*autoupdate.AutoupdateVersion, error) {
	resp, err := c.grpcClient.GetAutoupdateVersion(ctx, &autoupdate.GetAutoupdateVersionRequest{})
	return resp, trace.Wrap(err)
}

// CreateAutoupdateVersion creates a AutoupdateVersion.
func (c *Client) CreateAutoupdateVersion(ctx context.Context, config *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error) {
	resp, err := c.grpcClient.CreateAutoupdateVersion(ctx, &autoupdate.CreateAutoupdateVersionRequest{Version: config})
	return resp, trace.Wrap(err)
}

// UpdateAutoupdateVersion updates a AutoupdateVersion.
func (c *Client) UpdateAutoupdateVersion(ctx context.Context, config *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error) {
	resp, err := c.grpcClient.UpdateAutoupdateVersion(ctx, &autoupdate.UpdateAutoupdateVersionRequest{Version: config})
	return resp, trace.Wrap(err)
}

// UpsertAutoupdateVersion creates or updates a AutoupdateVersion.
func (c *Client) UpsertAutoupdateVersion(ctx context.Context, version *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error) {
	resp, err := c.grpcClient.UpsertAutoupdateVersion(ctx, &autoupdate.UpsertAutoupdateVersionRequest{Version: version})
	return resp, trace.Wrap(err)
}

// DeleteAutoupdateVersion removes the specified AutoupdateVersion resource.
func (c *Client) DeleteAutoupdateVersion(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoupdateVersion(ctx, &autoupdate.DeleteAutoupdateVersionRequest{})
	return trace.Wrap(err)
}
