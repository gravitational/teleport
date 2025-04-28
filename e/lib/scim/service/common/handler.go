package common

import (
	"context"

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
}
