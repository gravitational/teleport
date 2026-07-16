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

package discoveryconfig

import (
	"context"

	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
)

// Client is an DiscoveryConfig client that conforms to the following lib/services interfaces:
//   - services.DiscoveryConfigs
type Client struct {
	grpcClient discoveryconfigv1.DiscoveryConfigServiceClient
}

// NewClient creates a new Discovery Config client.
func NewClient(grpcClient discoveryconfigv1.DiscoveryConfigServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListDiscoveryConfigs returns a paginated list of DiscoveryConfigs.
func (c *Client) ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	resp, err := c.grpcClient.ListDiscoveryConfigs(ctx, &discoveryconfigv1.ListDiscoveryConfigsRequest{PageSize: int32(pageSize), NextToken: nextToken})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return convertList(resp)
}

// ListSyntheticDiscoveryConfigs returns synthetic inventory explicitly.
func (c *Client) ListSyntheticDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	resp, err := c.grpcClient.ListSyntheticDiscoveryConfigs(ctx, &discoveryconfigv1.ListDiscoveryConfigsRequest{PageSize: int32(pageSize), NextToken: nextToken})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return convertList(resp)
}

func convertList(resp *discoveryconfigv1.ListDiscoveryConfigsResponse) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	discoveryConfigs := make([]*discoveryconfig.DiscoveryConfig, 0, len(resp.DiscoveryConfigs))
	for _, discoveryConfig := range resp.DiscoveryConfigs {
		dc, err := conv.FromProto(discoveryConfig)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		discoveryConfigs = append(discoveryConfigs, dc)
	}

	return discoveryConfigs, resp.GetNextKey(), nil
}

// UpsertSyntheticDiscoveryConfig publishes observed static inventory.
func (c *Client) UpsertSyntheticDiscoveryConfig(ctx context.Context, status discoveryconfig.SyntheticStatus) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.UpsertSyntheticDiscoveryConfig(ctx, &discoveryconfigv1.UpsertSyntheticDiscoveryConfigRequest{Synthetic: conv.SyntheticStatusToProto(&status)})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.FromProto(resp)
}

// GetDiscoveryConfig returns the named DiscoveryConfig resource: regular
// resources first, falling back to owner-published synthetic inventory for
// reserved synthetic names. The server keeps the legacy RPC regular-only so
// older clients never receive a synthetic resource they cannot decode; the
// combined lookup lives here.
func (c *Client) GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.GetDiscoveryConfig(ctx, &discoveryconfigv1.GetDiscoveryConfigRequest{
		Name: name,
	})
	if err == nil {
		discoveryConfig, err := conv.FromProto(resp)
		return discoveryConfig, trace.Wrap(err)
	}
	if !trace.IsNotFound(err) || !discoveryconfig.IsReservedSyntheticName(name) {
		return nil, trace.Wrap(err)
	}
	dc, syntheticErr := c.GetSyntheticDiscoveryConfig(ctx, name)
	return dc, trace.Wrap(syntheticErr)
}

// GetSyntheticDiscoveryConfig returns the named owner-published synthetic
// DiscoveryConfig resource.
func (c *Client) GetSyntheticDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.GetSyntheticDiscoveryConfig(ctx, &discoveryconfigv1.GetDiscoveryConfigRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	discoveryConfig, err := conv.FromProto(resp)
	return discoveryConfig, trace.Wrap(err)
}

// CreateDiscoveryConfig creates the DiscoveryConfig.
func (c *Client) CreateDiscoveryConfig(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.CreateDiscoveryConfig(ctx, &discoveryconfigv1.CreateDiscoveryConfigRequest{
		DiscoveryConfig: conv.ToProto(discoveryConfig),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := conv.FromProto(resp)
	return dc, trace.Wrap(err)
}

// UpdateDiscoveryConfig updates the DiscoveryConfig.
func (c *Client) UpdateDiscoveryConfig(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.UpdateDiscoveryConfig(ctx, &discoveryconfigv1.UpdateDiscoveryConfigRequest{
		DiscoveryConfig: conv.ToProto(discoveryConfig),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := conv.FromProto(resp)
	return dc, trace.Wrap(err)
}

// UpsertDiscoveryConfig creates or updates a DiscoveryConfig.
func (c *Client) UpsertDiscoveryConfig(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.UpsertDiscoveryConfig(ctx, &discoveryconfigv1.UpsertDiscoveryConfigRequest{
		DiscoveryConfig: conv.ToProto(discoveryConfig),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := conv.FromProto(resp)
	return dc, trace.Wrap(err)
}

// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
func (c *Client) DeleteDiscoveryConfig(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteDiscoveryConfig(ctx, &discoveryconfigv1.DeleteDiscoveryConfigRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// DeleteAllDiscoveryConfigs removes all DiscoveryConfigs.
func (c *Client) DeleteAllDiscoveryConfigs(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAllDiscoveryConfigs(ctx, &discoveryconfigv1.DeleteAllDiscoveryConfigsRequest{})
	return trace.Wrap(err)
}

// UpdateDiscoveryConfigStatus updates the DiscoveryConfig Status field.
func (c *Client) UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error) {
	resp, err := c.grpcClient.UpdateDiscoveryConfigStatus(ctx, &discoveryconfigv1.UpdateDiscoveryConfigStatusRequest{
		Name:   name,
		Status: conv.StatusToProto(status),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := conv.FromProto(resp)
	return dc, trace.Wrap(err)
}
