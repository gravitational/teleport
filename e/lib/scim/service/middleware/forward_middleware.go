package middleware

import (
	"context"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// ForwardingMiddleware implements [Middleware] by forwarding all calls to the next handler.
// Embed this in your custom middleware to only implement the methods you care about.
type ForwardingMiddleware struct{}

// PatchResourceMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) PatchResourceMiddleware(ctx context.Context, req *pb.PatchSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	return next.PatchResource(ctx, req)
}

// GetResourceMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) GetResourceMiddleware(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	return next.GetResource(ctx, req)
}

// CreateResourceMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) CreateResourceMiddleware(ctx context.Context, req *pb.CreateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	return next.CreateResource(ctx, req)
}

// ListResourcesMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) ListResourcesMiddleware(ctx context.Context, req *pb.ListSCIMResourcesRequest, next common.ResourceHandler) (*pb.ResourceList, error) {
	return next.ListResources(ctx, req)
}

// UpdateResourceMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) UpdateResourceMiddleware(ctx context.Context, req *pb.UpdateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	return next.UpdateResource(ctx, req)
}

// DeleteResourceMiddleware implements Middleware by forwarding the call to the next handler.
func (f *ForwardingMiddleware) DeleteResourceMiddleware(ctx context.Context, req *pb.DeleteSCIMResourceRequest, next common.ResourceHandler) error {
	return next.DeleteResource(ctx, req)
}
