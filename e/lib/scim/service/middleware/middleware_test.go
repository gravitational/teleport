package middleware_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/middleware"
)

// mockHandler is a simple mock implementation of ResourceHandler for testing
type mockHandler struct {
	common.NotImplementedHandler
}

func (m *mockHandler) GetResource(ctx context.Context, req *pb.GetSCIMResourceRequest) (*pb.Resource, error) {
	return pb.Resource_builder{Id: "get-resource"}.Build(), nil
}

type testMiddlewareHandler struct {
	middleware.ForwardingMiddleware
	get func(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error)
}

func (tm testMiddlewareHandler) GetResourceMiddleware(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	return tm.get(ctx, req, next)
}

func TestMiddleware(t *testing.T) {
	t.Parallel()
	realHandler := &mockHandler{}

	t.Run("test request modification in middleware", func(t *testing.T) {
		middlewareHandler := &testMiddlewareHandler{
			get: func(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
				if req.GetTarget().GetResourceId() == "" {
					return nil, trace.BadParameter("missing resource ID")
				}
				return next.GetResource(ctx, req)
			},
		}

		chain := middleware.NewMiddlewareChain(middlewareHandler)
		handler := chain.Wrap(realHandler)

		_, err := handler.GetResource(t.Context(), &pb.GetSCIMResourceRequest{})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))

		res, err := handler.GetResource(t.Context(), pb.GetSCIMResourceRequest_builder{Target: pb.RequestTarget_builder{ResourceId: "123"}.Build()}.Build())
		require.NoError(t, err)
		require.Equal(t, "get-resource", res.GetId())
	})

	t.Run("test response modification in middleware", func(t *testing.T) {
		middlewareHandler := &testMiddlewareHandler{
			get: func(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
				res, err := next.GetResource(ctx, req)
				if err != nil {
					return nil, err
				}
				// Modify the response
				res.SetId("modified-" + res.GetId())
				return res, nil
			},
		}

		chain := middleware.NewMiddlewareChain(middlewareHandler)
		handler := chain.Wrap(realHandler)

		res, err := handler.GetResource(t.Context(), &pb.GetSCIMResourceRequest{})
		require.NoError(t, err)
		require.Equal(t, "modified-get-resource", res.GetId())
	})

	t.Run("test middleware execution order", func(t *testing.T) {
		var executionOrder []string

		mw1 := &testMiddlewareHandler{
			get: func(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
				executionOrder = append(executionOrder, "mw1-before")
				res, err := next.GetResource(ctx, req)
				executionOrder = append(executionOrder, "mw1-after")
				return res, err
			},
		}

		mw2 := &testMiddlewareHandler{
			get: func(ctx context.Context, req *pb.GetSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
				executionOrder = append(executionOrder, "mw2-before")
				res, err := next.GetResource(ctx, req)
				executionOrder = append(executionOrder, "mw2-after")
				return res, err
			},
		}

		chain := middleware.NewMiddlewareChain(mw1, mw2)
		handler := chain.Wrap(realHandler)

		_, err := handler.GetResource(t.Context(), &pb.GetSCIMResourceRequest{})
		require.NoError(t, err)

		// First appended middleware executes first (outermost)
		require.Equal(t, []string{
			"mw1-before",
			"mw2-before",
			"mw2-after",
			"mw1-after",
		}, executionOrder)
	})

	t.Run("test empty chain passes through to handler", func(t *testing.T) {
		chain := middleware.NewMiddlewareChain()
		handler := chain.Wrap(realHandler)

		res, err := handler.GetResource(t.Context(), &pb.GetSCIMResourceRequest{})
		require.NoError(t, err)
		require.Equal(t, "get-resource", res.GetId())
	})

	t.Run("test empty middleware chain passes through to handler", func(t *testing.T) {
		chain := middleware.NewMiddlewareChain()
		handler := chain.Wrap(realHandler)

		res, err := handler.GetResource(t.Context(), &pb.GetSCIMResourceRequest{})
		require.NoError(t, err)
		require.Equal(t, "get-resource", res.GetId())
	})
}
