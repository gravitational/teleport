// Copyright 2024 Gravitational, Inc.
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

package statichostuser

import (
	"context"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
)

// Client is a StaticHostUser client.
type Client struct {
	grpcClient userprovisioningpb.StaticHostUsersServiceClient
}

// NewClient creates a new StaticHostUser client.
func NewClient(grpcClient userprovisioningpb.StaticHostUsersServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListStaticHostUsers lists static host users.
func (c *Client) ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningpb.StaticHostUser, string, error) {
	resp, err := c.grpcClient.ListStaticHostUsers(ctx, &userprovisioningpb.ListStaticHostUsersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.Users, resp.NextPageToken, nil
}

// GetStaticHostUser returns a static host user by name.
func (c *Client) GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error) {
	if name == "" {
		return nil, trace.BadParameter("missing name")
	}
	out, err := c.grpcClient.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
		Name: name,
	})
	return out, trace.Wrap(err)
}

// CreateStaticHostUser creates a static host user.
func (c *Client) CreateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := c.grpcClient.CreateStaticHostUser(ctx, &userprovisioningpb.CreateStaticHostUserRequest{
		User: in,
	})
	return out, trace.Wrap(err)
}

// UpdateStaticHostUser updates a static host user.
func (c *Client) UpdateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := c.grpcClient.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
		User: in,
	})
	return out, trace.Wrap(err)
}

// UpsertStaticHostUser upserts a static host user.
func (c *Client) UpsertStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := c.grpcClient.UpsertStaticHostUser(ctx, &userprovisioningpb.UpsertStaticHostUserRequest{
		User: in,
	})
	return out, trace.Wrap(err)
}

// DeleteStaticHostUser deletes a static host user. Note that this does not
// remove any host users created on nodes from the resource.
func (c *Client) DeleteStaticHostUser(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing name")
	}
	_, err := c.grpcClient.DeleteStaticHostUser(ctx, &userprovisioningpb.DeleteStaticHostUserRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
