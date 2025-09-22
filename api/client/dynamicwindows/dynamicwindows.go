/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
