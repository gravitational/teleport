// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type fooCollection struct {
	foos []*foov1.Foo
}

func (c *fooCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.foos))
	for _, foo := range c.foos {
		out = append(out, types.ProtoResource153ToLegacy(foo))
	}
	return out
}

func (c *fooCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"ID", "Value", "Labels"}
	rows := make([][]string, 0, len(c.foos))
	for _, foo := range c.foos {
		rows = append(rows, []string{
			scopes.QualifiedName{Scope: foo.GetScope(), Name: foo.GetMetadata().GetName()}.String(),
			foo.GetSpec().GetValue(),
			PrintMetadataLabels(foo.GetMetadata().GetLabels()),
		})
	}

	t := asciitable.MakeTable(headers, rows...)
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func fooScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getFoo,
		createHandler: createFoo,
		updateHandler: updateFoo,
		deleteHandler: deleteFoo,
		description:   "A scope-aware Foo resource",
	}
}

func fooHandler() Handler {
	return Handler{
		getHandler:    getUnscopedFoo,
		createHandler: createFoo,
		updateHandler: updateFoo,
		deleteHandler: deleteUnscopedFoo,
		description:   "A scope-aware Foo resource",
	}
}

func createFoo(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	foo, err := services.UnmarshalProtoResource[*foov1.Foo](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	verb := "created"
	if opts.Force {
		verb = "upserted"
		_, err = client.FooClient().UpsertFoo(ctx, foov1.UpsertFooRequest_builder{
			Foo: foo,
		}.Build())
	} else {
		_, err = client.FooClient().CreateFoo(ctx, foov1.CreateFooRequest_builder{
			Foo: foo,
		}.Build())
	}
	if err != nil {
		return trace.Wrap(err)
	}

	sqn := scopes.QualifiedName{
		Scope: foo.GetScope(),
		Name:  foo.GetMetadata().GetName(),
	}
	fmt.Printf("%v %q has been %s\n", foos.Kind, sqn.String(), verb)
	return nil
}

func updateFoo(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	foo, err := services.UnmarshalProtoResource[*foov1.Foo](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.FooClient().UpdateFoo(ctx, foov1.UpdateFooRequest_builder{
		Foo: foo,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}

	sqn := scopes.QualifiedName{
		Scope: foo.GetScope(),
		Name:  foo.GetMetadata().GetName(),
	}
	fmt.Printf("%v %q has been updated\n", foos.Kind, sqn.String())
	return nil
}

func getFoo(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(foos.Kind, subKind)
	}

	if sqn != nil {
		resp, err := client.FooClient().GetFoo(ctx, foov1.GetFooRequest_builder{
			Name:  sqn.Name,
			Scope: sqn.Scope,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &fooCollection{foos: []*foov1.Foo{resp.GetFoo()}}, nil
	}

	items, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
		resp, err := client.FooClient().ListFoos(ctx, foov1.ListFoosRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			// exhaustive user-facing views use MODE_ALL per RFD 0229i
			ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
		}.Build())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.GetFoos(), resp.GetNextPageToken(), nil
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fooCollection{foos: items}, nil
}

func getUnscopedFoo(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.SubKind != "" {
		return nil, rejectSubKind(foos.Kind, ref.SubKind)
	}
	if ref.Name != "" {
		return getFoo(ctx, client, "", &scopes.QualifiedName{Name: ref.Name}, opts)
	}
	return getFoo(ctx, client, "", nil, opts)
}

func deleteFoo(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind != "" {
		return rejectSubKind(foos.Kind, subKind)
	}

	if _, err := client.FooClient().DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
		Name:  sqn.Name,
		Scope: sqn.Scope,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v %q has been deleted\n", foos.Kind, sqn.String())
	return nil
}

func deleteUnscopedFoo(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.SubKind != "" {
		return rejectSubKind(foos.Kind, ref.SubKind)
	}
	return deleteFoo(ctx, client, "", scopes.QualifiedName{Name: ref.Name})
}
