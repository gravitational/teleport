// Copyright 2025 Gravitational, Inc.
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

package vnetconfig

import (
	"context"

	"github.com/gravitational/trace"

	vnet "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	vnettypes "github.com/gravitational/teleport/api/types/vnet"
)

// Client is a VnetConfig client.
type Client struct {
	grpcClient vnet.VnetConfigServiceClient
}

// NewClient creates a new VnetConfig client.
func NewClient(grpcClient vnet.VnetConfigServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetVnetConfig returns the singleton VnetConfig resource.
func (c *Client) GetVnetConfig(ctx context.Context) (*vnet.VnetConfig, error) {
	resp, err := c.grpcClient.GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
	return resp, trace.Wrap(err)
}

// CreateVnetConfig creates the singleton VnetConfig resource.
func (c *Client) CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	resp, err := c.grpcClient.CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
	return resp, trace.Wrap(err)
}

// UpdateVnetConfig updates the singleton VnetConfig resource.
func (c *Client) UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	resp, err := c.grpcClient.UpdateVnetConfig(ctx, &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig})
	return resp, trace.Wrap(err)
}

// UpsertVnetConfig creates or updates the singleton VnetConfig resource.
func (c *Client) UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	resp, err := c.grpcClient.UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
	return resp, trace.Wrap(err)
}

// DeleteVnetConfig deletes the singleton VnetConfig resource.
func (c *Client) DeleteVnetConfig(ctx context.Context) error {
	_, err := c.grpcClient.DeleteVnetConfig(ctx, &vnet.DeleteVnetConfigRequest{})
	return trace.Wrap(err)
}

// ResetVnetConfig resets the singleton VnetConfig resource to defaults.
func (c *Client) ResetVnetConfig(ctx context.Context) error {
	defaultConfig, err := vnettypes.DefaultVnetConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.grpcClient.UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: defaultConfig})
	return trace.Wrap(err)
}
