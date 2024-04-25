package crownjewel

import (
	"context"

	"github.com/gravitational/trace"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types/crownjewel"
	crownjewelv1conv "github.com/gravitational/teleport/api/types/crownjewel/convert/v1"
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
func (c *Client) ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewel.CrownJewel, error) {
	resp, err := c.grpcClient.ListCrownJewels(ctx, &crownjewelv1.ListCrownJewelsRequest{
		PageSize:  pageSize,
		PageToken: nextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cjs := make([]*crownjewel.CrownJewel, 0, len(resp.CrownJewels))
	for _, cj := range resp.CrownJewels {
		cjs = append(cjs, crownjewelv1conv.FromProto(cj))
	}

	return cjs, nil
}

// CreateCrownJewel creates a new Crown Jewel.
func (c *Client) CreateCrownJewel(ctx context.Context, req *crownjewel.CrownJewel) (*crownjewel.CrownJewel, error) {
	rsp, err := c.grpcClient.CreateCrownJewel(ctx, &crownjewelv1.CreateCrownJewelRequest{
		CrownJewels: crownjewelv1conv.ToProto(req),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return crownjewelv1conv.FromProto(rsp), trace.Wrap(err)
}

// UpdateCrownJewel updates an existing Crown Jewel.
func (c *Client) UpdateCrownJewel(ctx context.Context, req *crownjewel.CrownJewel) (*crownjewel.CrownJewel, error) {
	rsp, err := c.grpcClient.UpdateCrownJewel(ctx, &crownjewelv1.UpdateCrownJewelRequest{
		CrownJewels: crownjewelv1conv.ToProto(req),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return crownjewelv1conv.FromProto(rsp), trace.Wrap(err)
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
