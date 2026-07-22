package services

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
)

// FooService is a service for interacting with Foo resources, implemented only
// by the backend storage service.
//
// It should be included in [lib/auth.InitConfig] and embedded in [lib/auth.Services].
type FooService interface {
	CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	UpsertFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)
	DeleteFoo(ctx context.Context, req *foov1.DeleteFooRequest) error
	GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error)
	ListFoos(ctx context.Context, req *foov1.ListFoosRequest) ([]*foov1.Foo, string, error)
	RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error]
}

// FooReader is a read interface for reading Foos from a backend storage
// service _or_ a cache.
//
// It should be embedded in [lib/auth/authclient.Cache] and consumed by the
// gRPC API layer.
type FooReader interface {
	GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error)
	RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error]
}

// FooUpstream is a read interface for reading Foos from a backend storage
// service _or_ an API client.
//
// It should be included in [lib/cache.Config] to be consumed by the cache.
type FooUpstream interface {
	GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error)
	ListFoos(ctx context.Context, req *foov1.ListFoosRequest) ([]*foov1.Foo, string, error)
}

// fooClientAdapter adapts a plain gRPC client to implement FooUpstream to be
// consumed by the cache.
//
// It is only necessary if the resource needs to be cached on proxies or agents.
type fooClientAdapter struct {
	grpcClient foov1.FooServiceClient
}

// NewFooClientAdapter adapts a plain gRPC client to implement FooUpstream.
func NewFooClientAdapter(grpcClient foov1.FooServiceClient) FooUpstream {
	return fooClientAdapter{
		grpcClient: grpcClient,
	}
}

func (c fooClientAdapter) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error) {
	resp, err := c.grpcClient.GetFoo(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetFoo(), nil
}

func (c fooClientAdapter) ListFoos(ctx context.Context, req *foov1.ListFoosRequest) ([]*foov1.Foo, string, error) {
	resp, err := c.grpcClient.ListFoos(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.GetFoos(), resp.GetNextPageToken(), nil
}
