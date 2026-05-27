// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package plugin

import (
	"context"

	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

// Client wraps the plugin gRPC client and implements services.Plugins.
type Client struct {
	grpcClient pluginspb.PluginServiceClient
}

// NewClient creates a new plugin client.
func NewClient(grpcClient pluginspb.PluginServiceClient) *Client {
	return &Client{grpcClient: grpcClient}
}

// CreatePlugin creates a new plugin.
func (c *Client) CreatePlugin(ctx context.Context, plugin types.Plugin) error {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return trace.BadParameter("plugin must be of type *types.PluginV1, got %T", plugin)
	}
	_, err := c.grpcClient.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{Plugin: pluginV1})
	return trace.Wrap(err)
}

// GetPlugin returns a plugin by name.
func (c *Client) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	plugin, err := c.grpcClient.GetPlugin(ctx, &pluginspb.GetPluginRequest{
		Name:        name,
		WithSecrets: withSecrets,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plugin, nil
}

// UpdatePlugin updates an existing plugin.
func (c *Client) UpdatePlugin(ctx context.Context, plugin types.Plugin) (types.Plugin, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin must be of type *types.PluginV1, got %T", plugin)
	}
	updated, err := c.grpcClient.UpdatePlugin(ctx, &pluginspb.UpdatePluginRequest{Plugin: pluginV1})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updated, nil
}

// DeletePlugin deletes a plugin by name.
func (c *Client) DeletePlugin(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{Name: name})
	return trace.Wrap(err)
}

// DeleteAllPlugins deletes all plugins. Not supported via gRPC; returns an error.
func (c *Client) DeleteAllPlugins(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllPlugins is not supported via gRPC client")
}

// GetPlugins returns all plugins.
func (c *Client) GetPlugins(ctx context.Context, withSecrets bool) ([]types.Plugin, error) {
	var plugins []types.Plugin
	var startKey string
	for {
		resp, err := c.grpcClient.ListPlugins(ctx, &pluginspb.ListPluginsRequest{
			WithSecrets: withSecrets,
			StartKey:    startKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, p := range resp.Plugins {
			plugins = append(plugins, p)
		}
		if resp.NextKey == "" {
			break
		}
		startKey = resp.NextKey
	}
	return plugins, nil
}

// ListPlugins returns a paginated list of plugins.
func (c *Client) ListPlugins(ctx context.Context, limit int, startKey string, withSecrets bool) ([]types.Plugin, string, error) {
	resp, err := c.grpcClient.ListPlugins(ctx, &pluginspb.ListPluginsRequest{
		PageSize:    int32(limit),
		StartKey:    startKey,
		WithSecrets: withSecrets,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	plugins := make([]types.Plugin, 0, len(resp.Plugins))
	for _, p := range resp.Plugins {
		plugins = append(plugins, p)
	}
	return plugins, resp.NextKey, nil
}

// HasPluginType returns true if a plugin of the given type exists.
func (c *Client) HasPluginType(ctx context.Context, pluginType types.PluginType) (bool, error) {
	var startKey string
	for {
		resp, err := c.grpcClient.ListPlugins(ctx, &pluginspb.ListPluginsRequest{
			StartKey:    startKey,
			WithSecrets: false,
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		for _, p := range resp.Plugins {
			if p.GetType() == pluginType {
				return true, nil
			}
		}
		if resp.NextKey == "" {
			break
		}
		startKey = resp.NextKey
	}
	return false, nil
}

// SetPluginCredentials sets credentials for a plugin.
func (c *Client) SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error {
	credsV1, ok := creds.(*types.PluginCredentialsV1)
	if !ok {
		return trace.BadParameter("credentials must be of type *types.PluginCredentialsV1, got %T", creds)
	}
	_, err := c.grpcClient.SetPluginCredentials(ctx, &pluginspb.SetPluginCredentialsRequest{
		Name:        name,
		Credentials: credsV1,
	})
	return trace.Wrap(err)
}

// SetPluginStatus sets the status for a plugin.
func (c *Client) SetPluginStatus(ctx context.Context, name string, status types.PluginStatus) error {
	statusV1, ok := status.(*types.PluginStatusV1)
	if !ok {
		return trace.BadParameter("status must be of type *types.PluginStatusV1, got %T", status)
	}
	_, err := c.grpcClient.SetPluginStatus(ctx, &pluginspb.SetPluginStatusRequest{
		Name:   name,
		Status: statusV1,
	})
	return trace.Wrap(err)
}
