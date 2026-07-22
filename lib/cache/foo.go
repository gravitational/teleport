package cache

import (
	"context"
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type fooIndex string

const (
	fooNameIndex fooIndex = "name"
)

func newFooCollection(upstream services.FooUpstream, w types.WatchKind) (*collection[*foov1.Foo, fooIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter FooUpstream")
	}

	scopeFilter := w.ScopeFilter.ToProto()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, trace.Wrap(err)
	}

	return &collection[*foov1.Foo, fooIndex]{
		store: newStore(
			foos.Kind,
			proto.CloneOf[*foov1.Foo],
			map[fooIndex]func(*foov1.Foo) string{
				fooNameIndex: foos.MakeCursor,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*foov1.Foo, error) {
			return stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
				return upstream.ListFoos(ctx, foov1.ListFoosRequest_builder{
					PageSize:    int32(pageSize),
					PageToken:   pageToken,
					ScopeFilter: scopeFilter,
				}.Build())
			}))
		},
		watch: w,
	}, nil
}

func (c *Cache) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetFoo")
	defer span.End()

	getter := genericGetter[*foov1.Foo, fooIndex]{
		cache:      c,
		collection: c.collections.foos,
		index:      fooNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*foov1.Foo, error) {
			return c.FooUpstream.GetFoo(ctx, req)
		},
	}

	fooCursor := scopes.MakeResourceCursor(req.GetScope(), req.GetName())
	out, err := getter.get(ctx, fooCursor)
	return out, trace.Wrap(err)
}

func (c *Cache) RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/RangeFoos")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[*foov1.Foo](trace.Wrap(err))
	}

	lister := genericLister[*foov1.Foo, fooIndex]{
		cache:      c,
		collection: c.collections.foos,
		index:      fooNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
			return c.FooUpstream.ListFoos(ctx, foov1.ListFoosRequest_builder{
				PageSize:    int32(pageSize),
				PageToken:   pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		filter: func(foo *foov1.Foo) bool {
			return scopes.MatchScope(scopeFilter, foo.GetScope())
		},
		nextToken: foos.MakeCursor,
	}

	return lister.Range(ctx, startKey, endKey)
}
