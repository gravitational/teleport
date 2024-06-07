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

package crownjewel

import (
	"context"

	"github.com/gravitational/trace"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
)

// Client is a client for the Crown Jewel API.
type Client struct {
	grpcClient crownjewelv1.CrownJewelServiceClient
}

// NewClient creates a new Discovery Config client.
func NewClient(grpcClient crownjewelv1.CrownJewelServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListCrownJewels returns a list of Crown Jewels.
func (c *Client) ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error) {
	resp, err := c.grpcClient.ListCrownJewels(ctx, &crownjewelv1.ListCrownJewelsRequest{
		PageSize:  pageSize,
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.CrownJewels, resp.NextPageToken, nil
}

// CreateCrownJewel creates a new Crown Jewel.
func (c *Client) CreateCrownJewel(ctx context.Context, req *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	rsp, err := c.grpcClient.CreateCrownJewel(ctx, &crownjewelv1.CreateCrownJewelRequest{
		CrownJewels: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// GetCrownJewel returns a Crown Jewel by name.
func (c *Client) GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error) {
	rsp, err := c.grpcClient.GetCrownJewel(ctx, &crownjewelv1.GetCrownJewelRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpdateCrownJewel updates an existing Crown Jewel.
func (c *Client) UpdateCrownJewel(ctx context.Context, req *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	rsp, err := c.grpcClient.UpdateCrownJewel(ctx, &crownjewelv1.UpdateCrownJewelRequest{
		CrownJewels: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpsertCrownJewel upserts a Crown Jewel.
func (c *Client) UpsertCrownJewel(ctx context.Context, req *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	rsp, err := c.grpcClient.UpsertCrownJewel(ctx, &crownjewelv1.UpsertCrownJewelRequest{
		CrownJewels: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// DeleteCrownJewel deletes a Crown Jewel.
func (c *Client) DeleteCrownJewel(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteCrownJewel(ctx, &crownjewelv1.DeleteCrownJewelRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// DeleteAllCrownJewels deletes all Crown Jewels.
// Not implemented. Added to satisfy the interface.
func (c *Client) DeleteAllCrownJewels(_ context.Context) error {
	return trace.NotImplemented("DeleteAllCrownJewels is not implemented")
}
