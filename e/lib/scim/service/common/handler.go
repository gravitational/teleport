package common

import (
	"context"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

// ResourceHandler is an abstraction for providing CRUD over different resource types.
type ResourceHandler interface {
	// CreateResource creates a new resource.
	CreateResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error)
	// ListResources lists all resources of a given type.
	ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error)
	// GetResource fetches a single resource.
	GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error)
	// UpdateResource updates an existing resource.
	UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error)
	// DeleteResource deletes a resource.
	DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error
	// PatchResource patches an existing resource.
	PatchResource(ctx context.Context, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error)
}

// NotImplementedHandler provides default stub methods for unsupported SCIM operations.
// It needs to fulfill the SCIM CRUD interface for the discovery resource that
// support only List or Get operations.
type NotImplementedHandler struct{}

func (NotImplementedHandler) GetResource(context.Context, *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (NotImplementedHandler) CreateResource(context.Context, *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (NotImplementedHandler) ListResources(context.Context, *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (NotImplementedHandler) UpdateResource(context.Context, *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (NotImplementedHandler) DeleteResource(context.Context, *scimpb.DeleteSCIMResourceRequest) error {
	return trace.NotImplemented("not implemented")
}

func (NotImplementedHandler) PatchResource(context.Context, *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}
