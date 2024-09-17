/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// Client is an AutoUpdateService client to manage autoupdate configuration and version.
type Client struct {
	grpcClient autoupdate.AutoUpdateServiceClient
}

// NewClient creates a new AutoUpdateService client.
func NewClient(grpcClient autoupdate.AutoUpdateServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetAutoUpdateConfig returns the specified AutoUpdateConfig resource.
func (c *Client) GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.GetAutoUpdateConfig(ctx, &autoupdate.GetAutoUpdateConfigRequest{})
	return resp, trace.Wrap(err)
}

// CreateAutoUpdateConfig creates a AutoUpdateConfig.
func (c *Client) CreateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.CreateAutoUpdateConfig(ctx, &autoupdate.CreateAutoUpdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// UpdateAutoUpdateConfig updates a AutoUpdateConfig.
func (c *Client) UpdateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.UpdateAutoUpdateConfig(ctx, &autoupdate.UpdateAutoUpdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// UpsertAutoUpdateConfig creates or updates a AutoUpdateConfig.
func (c *Client) UpsertAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.UpsertAutoUpdateConfig(ctx, &autoupdate.UpsertAutoUpdateConfigRequest{Config: config})
	return resp, trace.Wrap(err)
}

// DeleteAutoUpdateConfig removes the specified AutoUpdateConfig resource.
func (c *Client) DeleteAutoUpdateConfig(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoUpdateConfig(ctx, &autoupdate.DeleteAutoUpdateConfigRequest{})
	return trace.Wrap(err)
}

// GetAutoUpdateVersion returns the specified AutoUpdateVersion resource.
func (c *Client) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.GetAutoUpdateVersion(ctx, &autoupdate.GetAutoUpdateVersionRequest{})
	return resp, trace.Wrap(err)
}

// CreateAutoUpdateVersion creates a AutoUpdateVersion.
func (c *Client) CreateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.CreateAutoUpdateVersion(ctx, &autoupdate.CreateAutoUpdateVersionRequest{Version: config})
	return resp, trace.Wrap(err)
}

// UpdateAutoUpdateVersion updates a AutoUpdateVersion.
func (c *Client) UpdateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.UpdateAutoUpdateVersion(ctx, &autoupdate.UpdateAutoUpdateVersionRequest{Version: config})
	return resp, trace.Wrap(err)
}

// UpsertAutoUpdateVersion creates or updates a AutoUpdateVersion.
func (c *Client) UpsertAutoUpdateVersion(ctx context.Context, version *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.UpsertAutoUpdateVersion(ctx, &autoupdate.UpsertAutoUpdateVersionRequest{Version: version})
	return resp, trace.Wrap(err)
}

// DeleteAutoUpdateVersion removes the specified AutoUpdateVersion resource.
func (c *Client) DeleteAutoUpdateVersion(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoUpdateVersion(ctx, &autoupdate.DeleteAutoUpdateVersionRequest{})
	return trace.Wrap(err)
}
