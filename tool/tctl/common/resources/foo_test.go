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
	"testing"

	"github.com/stretchr/testify/require"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/foos"
)

func TestFooCollection_WriteText(t *testing.T) {
	collection := &fooCollection{foos: []*foov1.Foo{
		makeFoo("foo-1", "/security", "value-1", map[string]string{"env": "prod"}),
		makeFoo("foo-2", "/security/eu", "value-2", map[string]string{"env": "dev"}),
	}}

	table := asciitable.MakeTable(
		[]string{"ID", "Value", "Labels"},
		[]string{"/security/eu::foo-2", "value-2", "env=dev"},
		[]string{"/security::foo-1", "value-1", "env=prod"},
	)
	formatted := table.AsBuffer().String()

	collectionFormatTest(t, collection, formatted, formatted)
}

func TestFooCollection_Resources(t *testing.T) {
	t.Parallel()

	collection := &fooCollection{foos: []*foov1.Foo{
		makeFoo("foo-1", "/security", "value-1", nil),
	}}

	resources := collection.Resources()
	require.Len(t, resources, 1)
	require.Equal(t, foos.Kind, resources[0].GetKind())
	require.Equal(t, "foo-1", resources[0].GetName())
}

func TestFooScopedHandler(t *testing.T) {
	t.Parallel()

	handler, ok := ScopedHandlers()[foos.Kind]
	require.True(t, ok)
	require.NotNil(t, handler.getHandler)
	require.NotNil(t, handler.createHandler)
	require.NotNil(t, handler.updateHandler)
	require.NotNil(t, handler.deleteHandler)
	require.False(t, handler.mfaRequired)
	require.Equal(t, "A scope-aware Foo resource", handler.description)
}

func makeFoo(name, scope, value string, labels map[string]string) *foov1.Foo {
	return foov1.Foo_builder{
		Kind:    foos.Kind,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:   name,
			Labels: labels,
		}.Build(),
		Scope: scope,
		Spec: foov1.FooSpec_builder{
			Value: value,
		}.Build(),
	}.Build()
}
