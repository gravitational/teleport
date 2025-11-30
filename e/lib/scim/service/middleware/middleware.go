package middleware

import (
	"context"
	"slices"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// Middleware extends ResourceHandler with middleware methods that allow
// intercepting and extending the flow, similar to gRPC interceptors.
// Each middleware method receives a "next" handler parameter, enabling middleware chaining.
type Middleware interface {
	// GetResourceMiddleware intercepts GetResource calls.
	GetResourceMiddleware(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error)
	// CreateResourceMiddleware intercepts CreateResource calls.
	CreateResourceMiddleware(ctx context.Context, req *pb.CreateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error)
	// ListResourcesMiddleware intercepts ListResources calls.
	ListResourcesMiddleware(ctx context.Context, req *pb.ListSCIMResourcesRequest, next common.ResourceHandler) (*pb.ResourceList, error)
	// UpdateResourceMiddleware intercepts UpdateResource calls.
	UpdateResourceMiddleware(ctx context.Context, req *pb.UpdateSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error)
	// DeleteResourceMiddleware intercepts DeleteResource calls.
	DeleteResourceMiddleware(ctx context.Context, req *pb.DeleteSCIMResourceRequest, next common.ResourceHandler) error
	// PatchResourceMiddleware intercepts PatchResource calls.
	PatchResourceMiddleware(ctx context.Context, req *pb.PatchSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error)
}

// Chain allows composing multiple middleware handlers together.
// Middleware are executed in the order they are appended (first appended = outermost).
type Chain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain with the given middleware handlers.
// Middleware are executed in the order they are provided (first = outermost).
func NewMiddlewareChain(middlewareHandlers ...Middleware) *Chain {
	return &Chain{middlewares: middlewareHandlers}
}

// Wrap wraps the given handler with all middleware in the chain.
// Middleware are executed in the order they were appended (first appended = outermost).
// Returns a ResourceHandler that will execute all middleware before reaching the base handler.
func (c *Chain) Wrap(h common.ResourceHandler) common.ResourceHandler {
	if len(c.middlewares) == 0 {
		return h
	}
	current := h
	// Wrap with middleware in reverse order. It means that the first appended middleware
	// will be the outermost one.
	for _, mw := range slices.Backward(c.middlewares) {
		current = &wrappedHandler{middleware: mw, next: current}
	}
	return current
}

// wrappedHandler wraps a handler with a middleware
type wrappedHandler struct {
	middleware Middleware
	next       common.ResourceHandler
}

func (w *wrappedHandler) GetResource(ctx context.Context, req *pb.GetSCIMResourceRequest) (*pb.Resource, error) {
	return w.middleware.GetResourceMiddleware(ctx, req, w.next)
}

func (w *wrappedHandler) CreateResource(ctx context.Context, req *pb.CreateSCIMResourceRequest) (*pb.Resource, error) {
	return w.middleware.CreateResourceMiddleware(ctx, req, w.next)
}

func (w *wrappedHandler) ListResources(ctx context.Context, req *pb.ListSCIMResourcesRequest) (*pb.ResourceList, error) {
	return w.middleware.ListResourcesMiddleware(ctx, req, w.next)
}

func (w *wrappedHandler) UpdateResource(ctx context.Context, req *pb.UpdateSCIMResourceRequest) (*pb.Resource, error) {
	return w.middleware.UpdateResourceMiddleware(ctx, req, w.next)
}

func (w *wrappedHandler) DeleteResource(ctx context.Context, req *pb.DeleteSCIMResourceRequest) error {
	return w.middleware.DeleteResourceMiddleware(ctx, req, w.next)
}

func (w *wrappedHandler) PatchResource(ctx context.Context, req *pb.PatchSCIMResourceRequest) (*pb.Resource, error) {
	return w.middleware.PatchResourceMiddleware(ctx, req, w.next)
}
