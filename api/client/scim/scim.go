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

package scim

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

// Client wraps the underlying GRPC client with some more human-friendly tooling
type Client struct {
	grpcClient scimpb.SCIMServiceClient
}

func NewClientFromConn(cc grpc.ClientConnInterface) *Client {
	return NewClient(scimpb.NewSCIMServiceClient(cc))
}

func NewClient(grpcClient scimpb.SCIMServiceClient) *Client {
	return &Client{grpcClient: grpcClient}
}

// List fetches all (or a subset of all) resources resources of a given type
func (c *Client) ListSCIMResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	resp, err := c.grpcClient.ListSCIMResources(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "handling SCIM list request")
	}
	return resp, nil
}

// GetSCIMResource fetches a single SCIM resource from the server by name
func (c *Client) GetSCIMResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	resp, err := c.grpcClient.GetSCIMResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "handling SCIM get request")
	}
	return resp, nil
}

// CreateSCIResource creates a new SCIM resource based on a supplied
// resource description
func (c *Client) CreateSCIMResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	resp, err := c.grpcClient.CreateSCIMResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "handling SCIM create request")
	}
	return resp, nil
}

// UpdateSCIMResource handles a request to update a resource, returning a
// representation of the updated resource
func (c *Client) UpdateSCIMResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	res, err := c.grpcClient.UpdateSCIMResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "handling SCIM update request")
	}
	return res, nil
}

// DeleteSCIMResource handles a request to delete a resource.
func (c *Client) DeleteSCIMResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) (*emptypb.Empty, error) {
	res, err := c.grpcClient.DeleteSCIMResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "handling SCIM delete request")
	}
	return res, nil
}
