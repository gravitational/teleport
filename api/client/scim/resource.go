package scim

import (
	"context"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/trace"
)

type ResourceClient struct {
}

func (c *ResourceClient) ListSCIMUserResources(ctx context.Context, pageSize int64, lastKey string) ([]*scimpb.ResourceItem, string, error) {
	return nil, "", trace.NotImplemented("not implemented")
}

func (c *ResourceClient) GetSCIMUserResource(ctx context.Context, name string) (*scimpb.ResourceItem, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *ResourceClient) CreateSCIMUserResource(ctx context.Context, item *scimpb.ResourceItem) (*scimpb.ResourceItem, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *ResourceClient) UpdateSCIMUserResource(ctx context.Context, item *scimpb.ResourceItem) (*scimpb.ResourceItem, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *ResourceClient) UpsertSCIMUserResource(ctx context.Context, item *scimpb.ResourceItem) (*scimpb.ResourceItem, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *ResourceClient) DeleteSCIMUserResource(ctx context.Context, name string) error {
	return trace.NotImplemented("not implemented")
}

func (c *ResourceClient) DeleteAllSCIMUserResources(ctx context.Context) error {
	return trace.NotImplemented("not implemented")
}
