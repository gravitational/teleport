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

package foo

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestFooServiceAuthzAllowsPinnedScope(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	backend, scopedRoles := newLocalBackends(t)
	srv := newFooServerForScope(t, backend, scopedRoles, "/security")

	created, err := srv.CreateFoo(ctx, foov1.CreateFooRequest_builder{
		Foo: newFoo("foo-1", "/security", "initial"),
	}.Build())
	require.NoError(t, err)
	require.Equal(t, "foo-1", created.GetFoo().GetMetadata().GetName())

	got, err := srv.GetFoo(ctx, foov1.GetFooRequest_builder{
		Name:  "foo-1",
		Scope: "/security",
	}.Build())
	require.NoError(t, err)
	require.Equal(t, "initial", got.GetFoo().GetSpec().GetValue())

	updated := proto.Clone(got.GetFoo()).(*foov1.Foo)
	updated.GetSpec().SetValue("updated")
	updateResp, err := srv.UpdateFoo(ctx, foov1.UpdateFooRequest_builder{Foo: updated}.Build())
	require.NoError(t, err)
	require.Equal(t, "updated", updateResp.GetFoo().GetSpec().GetValue())

	_, err = srv.UpsertFoo(ctx, foov1.UpsertFooRequest_builder{
		Foo: newFoo("foo-2", "/security/child", "child"),
	}.Build())
	require.NoError(t, err)

	listResp, err := srv.ListFoos(ctx, foov1.ListFoosRequest_builder{}.Build())
	require.NoError(t, err)
	require.Equal(t, []string{"/security/foo-1"}, fooKeys(listResp.GetFoos()))

	_, err = srv.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
		Name:  "foo-1",
		Scope: "/security",
	}.Build())
	require.NoError(t, err)
}

func TestFooServiceAuthzDeniesOutsidePinnedScope(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	backend, scopedRoles := newLocalBackends(t)
	srv := newFooServerForScope(t, backend, scopedRoles, "/security")

	_, err := srv.CreateFoo(ctx, foov1.CreateFooRequest_builder{
		Foo: newFoo("foo-1", "/prod", "prod"),
	}.Build())
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)

	_, err = backend.CreateFoo(ctx, newFoo("foo-2", "/prod", "prod"))
	require.NoError(t, err)
	_, err = srv.GetFoo(ctx, foov1.GetFooRequest_builder{
		Name:  "foo-2",
		Scope: "/prod",
	}.Build())
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)

	_, err = srv.DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
		Name:  "foo-2",
		Scope: "/prod",
	}.Build())
	require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)

	_, err = backend.CreateFoo(ctx, newFoo("foo-3", "/security", "security"))
	require.NoError(t, err)
	listResp, err := srv.ListFoos(ctx, foov1.ListFoosRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_ALL,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Equal(t, []string{"/security/foo-3"}, fooKeys(listResp.GetFoos()))
}

type fakeScopedAuthorizer struct {
	ctx *authz.ScopedContext
}

func (a *fakeScopedAuthorizer) AuthorizeScoped(context.Context) (*authz.ScopedContext, error) {
	return a.ctx, nil
}

func newLocalBackends(t *testing.T) (*local.FooService, *local.ScopedAccessService) {
	t.Helper()

	mem, err := memory.New(memory.Config{Context: t.Context()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	fooBackend, err := local.NewFooService(mem)
	require.NoError(t, err)
	return fooBackend, local.NewScopedAccessService(mem)
}

func newFooServerForScope(t *testing.T, backend *local.FooService, scopedRoles *local.ScopedAccessService, scope string) *Service {
	t.Helper()

	ctx := t.Context()
	roleName := "foo-admin"
	_, err := scopedRoles.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: roleName,
			}.Build(),
			Scope: scope,
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{scope},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{foos.Kind},
						Verbs: []string{
							types.VerbCreate,
							types.VerbReadNoSecrets,
							types.VerbList,
							types.VerbUpdate,
							types.VerbDelete,
						},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}.Build())
	require.NoError(t, err)

	checkerCtx, err := services.NewScopedAccessCheckerContext(ctx, &services.AccessInfo{
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: scope,
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				scope: {scope: {scopes.QualifiedName{Scope: scope, Name: roleName}.String()}},
			}),
		}.Build(),
		Username: "alice",
	}, "test-cluster", scopedRoles)
	require.NoError(t, err)

	return &Service{cfg: &Config{
		ScopedAuthorizer: &fakeScopedAuthorizer{ctx: &authz.ScopedContext{
			User:           &types.UserV2{Metadata: types.Metadata{Name: "alice"}},
			CheckerContext: checkerCtx,
		}},
		Reader: backend,
		Writer: backend,
	}}
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

func fooKey(scope, name string) string {
	return scope + "/" + name
}

func fooKeys(foos []*foov1.Foo) []string {
	keys := make([]string, 0, len(foos))
	for _, foo := range foos {
		keys = append(keys, fooKey(foo.GetScope(), foo.GetMetadata().GetName()))
	}
	return keys
}
