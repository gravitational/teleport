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

// UpsertDynamicWindowsDesktopResult is the outcome of upserting a single dynamic
// Windows desktop as part of a bulk UpsertDynamicWindowsDesktops call.
type UpsertDynamicWindowsDesktopResult struct {
	// Name identifies which desktop this result refers to.
	Name string
	// Err is the error encountered while upserting the desktop, or nil on success.
	Err error
}

// UpsertDynamicWindowsDesktops upserts multiple dynamic Windows desktops in a
// single request. The returned slice contains one result per desktop, which
// contains the desktop name and an error that is nil if successfully upserted.
//
// At most 1000 desktops may be upserted per request. Exceeding the limit fails
// the whole request with a BadParameter error.
func (c *Client) UpsertDynamicWindowsDesktops(ctx context.Context, desktops []types.DynamicWindowsDesktop) ([]UpsertDynamicWindowsDesktopResult, error) {
	req := make([]*types.DynamicWindowsDesktopV1, 0, len(desktops))
	for _, desktop := range desktops {
		d, ok := desktop.(*types.DynamicWindowsDesktopV1)
		if !ok {
			return nil, trace.BadParameter("unknown desktop type: %T", desktop)
		}
		req = append(req, d)
	}

	resp, err := c.grpcClient.UpsertDynamicWindowsDesktops(ctx, &dynamicwindows.UpsertDynamicWindowsDesktopsRequest{
		Desktops: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]UpsertDynamicWindowsDesktopResult, 0, len(resp.GetResults()))
	for _, result := range resp.GetResults() {
		var resultErr error
		if result.GetError() != "" {
			resultErr = trace.Errorf("%s", result.GetError())
		}
		results = append(results, UpsertDynamicWindowsDesktopResult{
			Name: result.GetName(),
			Err:  resultErr,
		})
	}
	return results, nil
}

func (c *Client) DeleteDynamicWindowsDesktop(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteDynamicWindowsDesktop(ctx, &dynamicwindows.DeleteDynamicWindowsDesktopRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
