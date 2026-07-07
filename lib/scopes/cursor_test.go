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

package scopes

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceCursor(t *testing.T) {
	t.Parallel()

	t.Run("unscoped", func(t *testing.T) {
		cursor := MakeResourceCursor("", "name")
		require.Equal(t, "name", cursor)
		require.False(t, IsScopedResourceCursor(cursor))

		parsed, err := ParseResourceCursor(cursor)
		require.NoError(t, err)
		require.Equal(t, QualifiedName{Name: "name"}, parsed)
	})

	t.Run("scoped", func(t *testing.T) {
		cursor := MakeResourceCursor("/aa/bb", "name")
		require.True(t, IsScopedResourceCursor(cursor))
		require.Contains(t, cursor, ResourceCursorPrefix)

		parsed, err := ParseResourceCursor(cursor)
		require.NoError(t, err)
		require.Equal(t, QualifiedName{Scope: "/aa/bb", Name: "name"}, parsed)
	})

	t.Run("scoped start", func(t *testing.T) {
		require.Equal(t, ResourceCursorPrefix, ResourceCursorScopedStart())
		require.True(t, IsScopedResourceCursor(ResourceCursorScopedStart()))
	})
}

func TestParseResourceCursorErrors(t *testing.T) {
	t.Parallel()

	for _, cursor := range []string{
		ResourceCursorPrefix,
		ResourceCursorPrefix + "/name",
		ResourceCursorPrefix + "++/",
		ResourceCursorPrefix + "++/nested/name",
		ResourceCursorPrefix + "invalid/name",
	} {
		t.Run(cursor, func(t *testing.T) {
			_, err := ParseResourceCursor(cursor)
			require.Error(t, err)
		})
	}
}

func TestResourceCursorInvalidScope(t *testing.T) {
	t.Parallel()

	// A scope that cannot be encoded safely, e.g. read from invalid stored
	// data. The cursor is degraded but still deterministic and unique.
	badScope := "/foo bar"
	cursor := MakeResourceCursor(badScope, "name")

	// Degraded cursors sort after all valid cursors so the affected resource
	// ranges at the end of the logical resource stream.
	require.Greater(t, cursor, MakeResourceCursor("", "zzzz"))
	require.Greater(t, cursor, MakeResourceCursor("/zz/zz", "zzzz"))

	// Unique per scope and name.
	require.NotEqual(t, cursor, MakeResourceCursor(badScope, "other"))
	require.NotEqual(t, cursor, MakeResourceCursor("/other bad scope", "name"))

	// Degraded cursors cannot be parsed back into a scope-qualified name.
	_, err := ParseResourceCursor(cursor)
	require.Error(t, err)
}

func TestResourceCursorSort(t *testing.T) {
	t.Parallel()

	resources := []QualifiedName{
		{Scope: "/bb", Name: "aaa"},
		{Scope: "", Name: "zzz"},
		{Scope: "/aa", Name: "bbb"},
		{Scope: "/aa/bb", Name: "aaa"},
		{Scope: "", Name: "aaa"},
		{Scope: "/aa", Name: "aaa"},
	}

	cursors := make([]string, 0, len(resources))
	for _, resource := range resources {
		cursors = append(cursors, MakeResourceCursor(resource.Scope, resource.Name))
	}

	slices.Sort(cursors)

	var got []QualifiedName
	for _, cursor := range cursors {
		resource, err := ParseResourceCursor(cursor)
		require.NoError(t, err)
		got = append(got, resource)
	}

	require.Equal(t, []QualifiedName{
		// Unscoped cursors preserve historical name-only ordering and sort before
		// all scoped cursors.
		{Scope: "", Name: "aaa"},
		{Scope: "", Name: "zzz"},
		// Scoped cursors sort by encoded scope, then name within the same scope.
		{Scope: "/aa", Name: "aaa"},
		{Scope: "/aa", Name: "bbb"},
		{Scope: "/aa/bb", Name: "aaa"},
		{Scope: "/bb", Name: "aaa"},
	}, got)
}
