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

package dynamicwindows

import (
	"context"

	"github.com/gravitational/trace"

	dynamicwindows "github.com/gravitational/teleport/api/gen/proto/go/teleport/dynamicwindows/v1"
	"github.com/gravitational/teleport/api/types"
)

// Client is a DynamicWindowsDesktop client.
type Client struct {
	grpcClient dynamicwindows.DynamicWindowsServiceClient
}

// NewClient creates a new StaticHostUser client.
func NewClient(grpcClient dynamicwindows.DynamicWindowsServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

func (c *Client) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	desktop, err := c.grpcClient.GetDynamicWindowsDesktop(ctx, &dynamicwindows.GetDynamicWindowsDesktopRequest{
		Name: name,
	})
	return desktop, trace.Wrap(err)
}

func (c *Client) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error) {
	resp, err := c.grpcClient.ListDynamicWindowsDesktops(ctx, &dynamicwindows.ListDynamicWindowsDesktopsRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	desktops := make([]types.DynamicWindowsDesktop, 0, len(resp.GetDesktops()))
	for _, desktop := range resp.GetDesktops() {
		desktops = append(desktops, desktop)
	}
	return desktops, resp.GetNextPageToken(), nil
}

func (c *Client) CreateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	switch desktop := desktop.(type) {
	case *types.DynamicWindowsDesktopV1:
		desktop, err := c.grpcClient.CreateDynamicWindowsDesktop(ctx, &dynamicwindows.CreateDynamicWindowsDesktopRequest{
			Desktop: desktop,
		})
		return desktop, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown desktop type: %T", desktop)
	}
}

func (c *Client) UpdateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	switch desktop := desktop.(type) {
	case *types.DynamicWindowsDesktopV1:
		desktop, err := c.grpcClient.UpdateDynamicWindowsDesktop(ctx, &dynamicwindows.UpdateDynamicWindowsDesktopRequest{
			Desktop: desktop,
		})
		return desktop, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown desktop type: %T", desktop)
	}
}

func (c *Client) UpsertDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	switch desktop := desktop.(type) {
	case *types.DynamicWindowsDesktopV1:
		desktop, err := c.grpcClient.UpsertDynamicWindowsDesktop(ctx, &dynamicwindows.UpsertDynamicWindowsDesktopRequest{
			Desktop: desktop,
		})
		return desktop, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown desktop type: %T", desktop)
	}
}

func (c *Client) DeleteDynamicWindowsDesktop(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteDynamicWindowsDesktop(ctx, &dynamicwindows.DeleteDynamicWindowsDesktopRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

func (c *Client) DeleteAllDynamicWindowsDesktops(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllDynamicWindowsDesktops is not supported in the gRPC client")
}
