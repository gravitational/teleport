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

package local

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
)

func TestFooServiceCRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mem, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	service, err := NewFooService(mem)
	require.NoError(t, err)

	foo := newFoo("foo-1", "/security", "initial")
	created, err := service.CreateFoo(ctx, proto.Clone(foo).(*foov1.Foo))
	require.NoError(t, err)
	require.NotEmpty(t, created.GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(foo, created,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	got, err := service.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(created, got, protocmp.Transform()))

	_, err = service.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/other"}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)

	update := proto.Clone(created).(*foov1.Foo)
	update.GetSpec().SetValue("updated")
	updated, err := service.UpdateFoo(ctx, update)
	require.NoError(t, err)
	require.Equal(t, "updated", updated.GetSpec().GetValue())
	require.NotEqual(t, created.GetMetadata().GetRevision(), updated.GetMetadata().GetRevision())

	updated.GetMetadata().SetRevision("stale")
	updated.GetSpec().SetValue("not-persisted")
	_, err = service.UpdateFoo(ctx, updated)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)

	got, err = service.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build())
	require.NoError(t, err)
	require.Equal(t, "updated", got.GetSpec().GetValue())

	upserted := newFoo("foo-2", "", "upserted")
	_, err = service.UpsertFoo(ctx, upserted)
	require.NoError(t, err)
	got, err = service.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-2"}.Build())
	require.NoError(t, err)
	require.Equal(t, "upserted", got.GetSpec().GetValue())

	require.NoError(t, service.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build()))
	_, err = service.GetFoo(ctx, foov1.GetFooRequest_builder{Name: "foo-1", Scope: "/security"}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
}

func TestFooServiceList(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mem, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	service, err := NewFooService(mem)
	require.NoError(t, err)

	for _, foo := range []*foov1.Foo{
		newFoo("a", "", "unscoped-a"),
		newFoo("b", "", "unscoped-b"),
		newFoo("a", "/security", "scoped-a"),
		newFoo("b", "/security/eu", "scoped-b"),
	} {
		_, err := service.CreateFoo(ctx, foo)
		require.NoError(t, err)
	}

	got, err := stream.Collect(service.RangeFoos(ctx, nil, "", ""))
	require.NoError(t, err)
	require.Equal(t, []string{"/a", "/b", "/security/a", "/security/eu/b"}, fooKeys(got))

	got, err = stream.Collect(service.RangeFoos(ctx, foov1.ListFoosRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_UNSCOPED,
		}.Build(),
	}.Build(), "", ""))
	require.NoError(t, err)
	require.Equal(t, []string{"/a", "/b"}, fooKeys(got))

	got, err = stream.Collect(service.RangeFoos(ctx, foov1.ListFoosRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Scope: "/security",
			Mode:  scopesv1.Mode_MODE_DESCENDANTS,
		}.Build(),
	}.Build(), "", ""))
	require.NoError(t, err)
	require.Equal(t, []string{"/security/a", "/security/eu/b"}, fooKeys(got))

	startKey := scopes.MakeResourceCursor("", "b")
	endKey := scopes.MakeResourceCursor("/security/eu", "b")
	require.NoError(t, err)
	t.Log(startKey, endKey)
	got, err = stream.Collect(service.RangeFoos(ctx, nil, startKey, endKey))
	require.NoError(t, err)
	require.Equal(t, []string{"/b", "/security/a"}, fooKeys(got))
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

func fooKeys(foos []*foov1.Foo) []string {
	keys := make([]string, 0, len(foos))
	for _, foo := range foos {
		keys = append(keys, foo.GetScope()+"/"+foo.GetMetadata().GetName())
	}
	return keys
}
