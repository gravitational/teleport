// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

func TestFoos(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	ctx := t.Context()
	unscoped := newFoo("foo-1", "", "unscoped")
	scoped := newFoo("foo-1", "/security", "scoped")

	_, err := p.foos.CreateFoo(ctx, unscoped)
	require.NoError(t, err)
	_, err = p.foos.CreateFoo(ctx, scoped)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
		cmpopts.EquateEmpty(),
	}

	assertCacheFoos := func(expected []*foov1.Foo) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			got, err := stream.Collect(p.cache.RangeFoos(ctx, &foov1.ListFoosRequest{}, "", ""))
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(expected, got, cmpOpts...))
		}, 2*time.Second, 10*time.Millisecond)
	}

	assertCacheFoos([]*foov1.Foo{unscoped, scoped})

	got, err := p.cache.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1"}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(unscoped, got, cmpOpts...))

	got, err = p.cache.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(scoped, got, cmpOpts...))

	scopedFoos, err := stream.Collect(p.cache.RangeFoos(ctx, foov1.ListFoosRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{Scope: "/security", Mode: scopesv1.Mode_MODE_EXACT}.Build(),
	}.Build(), "", ""))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*foov1.Foo{scoped}, scopedFoos, cmpOpts...))

	require.NoError(t, p.foos.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build()))
	assertCacheFoos([]*foov1.Foo{unscoped})

	_, err = p.cache.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound after delete, got %v", err)
}

func TestFoos153(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*foov1.Foo]{
		newResource: func(name string) (*foov1.Foo, error) {
			return newFoo(name, "", name), nil
		},
		create: func(ctx context.Context, foo *foov1.Foo) error {
			_, err := p.foos.CreateFoo(ctx, foo)
			return err
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
			return p.foos.ListFoos(ctx, foov1.ListFoosRequest_builder{
				PageSize:  int32(pageSize),
				PageToken: pageToken,
			}.Build())
		},
		cacheGet: func(ctx context.Context, name string) (*foov1.Foo, error) {
			return p.cache.GetFoo(ctx, foov1.GetFooRequest_builder{
				Name: name,
			}.Build())
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
			return generic.CollectPageAndCursor(
				p.cache.RangeFoos(ctx, nil, pageToken, ""),
				pageSize,
				foos.MakeCursor,
			)
		},
		cacheRange: func(ctx context.Context, startKey, endKey string) iter.Seq2[*foov1.Foo, error] {
			return p.cache.RangeFoos(ctx, nil, startKey, endKey)
		},
		update: func(ctx context.Context, foo *foov1.Foo) error {
			_, err := p.foos.UpdateFoo(ctx, foo)
			return err
		},
		delete: func(ctx context.Context, name string) error {
			return p.foos.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
				Name: name,
			}.Build())
		},
		deleteAll: func(ctx context.Context) error {
			for foo, err := range p.foos.RangeFoos(ctx, nil, "", "") {
				if err != nil {
					return trace.Wrap(err)
				}
				if err := p.foos.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
					Scope: foo.GetScope(),
					Name:  foo.GetMetadata().GetName(),
				}.Build()); err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		},
	})
}

func newFoo(name, scope, value string) *foov1.Foo {
	return foov1.Foo_builder{
		Kind:    foos.Kind,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Scope: scope,
		Spec: foov1.FooSpec_builder{
			Value: value,
		}.Build(),
	}.Build()
}
