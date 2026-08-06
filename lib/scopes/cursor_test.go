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
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
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

	t.Run("unscoped with host", func(t *testing.T) {
		cursor := MakeResourceCursorWithHost("", "host-id", "name")
		require.Equal(t, "host-id/name", cursor)
		require.False(t, IsScopedResourceCursor(cursor))
	})

	t.Run("scoped with host", func(t *testing.T) {
		cursor := MakeResourceCursorWithHost("/aa/bb", "host-id", "name")
		require.True(t, IsScopedResourceCursor(cursor))
		require.Equal(t, MakeResourceCursor("/aa/bb", "host-id/name"), cursor)
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

func TestNestedResourceCursor(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		root        QualifiedName
		descendents []QualifiedName
		expected    string
	}{
		{
			desc: "empty",
		},
		{
			desc:     "unscoped",
			root:     QualifiedName{Name: "test"},
			expected: "test",
		},
		{
			desc: "unscoped nested",
			root: QualifiedName{Name: "test"},
			descendents: []QualifiedName{
				{Name: "nested"},
			},
			expected: "test/nested",
		},
		{
			desc: "unscoped double nested",
			root: QualifiedName{Name: "test"},
			descendents: []QualifiedName{
				{Name: "nested"},
				{Name: "doublenested"},
			},
			expected: "test/nested/doublenested",
		},
		{
			desc:     "scoped",
			root:     QualifiedName{Scope: "/aa", Name: "test"},
			expected: ResourceCursorPrefix + EncodeForResourceCursor("/aa") + separator + "test",
		},
		{
			desc: "scoped nested",
			root: QualifiedName{Scope: "/aa", Name: "test"},
			descendents: []QualifiedName{
				{Scope: "/", Name: "nested"},
			},
			expected: ResourceCursorPrefix + EncodeForResourceCursor("/aa") + separator + "test" +
				separator + EncodeForResourceCursor("/") + separator + "nested",
		},
		{
			desc: "scoped double nested",
			root: QualifiedName{Scope: "/aa", Name: "test"},
			descendents: []QualifiedName{
				{Scope: "/", Name: "nested"},
				{Scope: "/", Name: "doublenested"},
			},
			expected: ResourceCursorPrefix + EncodeForResourceCursor("/aa") + separator + "test" +
				separator + EncodeForResourceCursor("/") + separator + "nested" +
				separator + EncodeForResourceCursor("/") + separator + "doublenested",
		},
		{
			desc:        "mixed scopes",
			root:        QualifiedName{Scope: "/aa", Name: "test"},
			descendents: []QualifiedName{{Name: "nested"}},
			expected: ResourceCursorPrefix + EncodeForResourceCursor("/aa") + separator + "test" +
				separator + EncodeForResourceCursor("") + separator + "nested",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, MakeNestedResourceCursor(tc.root, tc.descendents...))
		})
	}
}

func TestNestedResourceCursorSort(t *testing.T) {
	sorted := [][]QualifiedName{
		{{Name: "a"}},
		{{Name: "a"}, {Name: "a"}},
		{{Name: "a"}, {Name: "b"}},

		{{Name: "b"}},
		{{Name: "b"}, {Name: "a"}},
		{{Name: "b"}, {Name: "b"}},
		{{Name: "b"}, {Name: "b"}, {Name: "a"}},

		{{Scope: "/aa", Name: "a"}},
		{{Scope: "/aa", Name: "a"}, {Scope: "/bb", Name: "a"}},
		{{Scope: "/aa", Name: "a"}, {Scope: "/bb", Name: "b"}},
		{{Scope: "/aa", Name: "a"}, {Scope: "/bb", Name: "b"}, {Scope: "/cc/dd", Name: "a"}},
		{{Scope: "/aa", Name: "a"}, {Scope: "/cc", Name: "a"}},
		{{Scope: "/aa", Name: "b"}},

		{{Scope: "/zz/aa", Name: "z"}},
		{{Scope: "/zz/aa/bb", Name: "z"}},
		{{Scope: "/zz/aa/cc", Name: "z"}},
		{{Scope: "/zz/bb", Name: "z"}},
		{{Scope: "/zz/cc", Name: "z"}},

		{{Scope: "bad scope 1"}},
		{{Scope: "bad scope 2"}},
	}
	expectSortedCursors := sliceutils.Map(sorted, func(names []QualifiedName) string {
		return MakeNestedResourceCursor(names[0], names[1:]...)
	})

	shuffledCursors := slices.Clone(expectSortedCursors)
	rand.Shuffle(len(shuffledCursors), func(i, j int) {
		shuffledCursors[i], shuffledCursors[j] = shuffledCursors[j], shuffledCursors[i]
	})

	// Assert that a string sort gets us back to the expected sorted order.
	slices.Sort(shuffledCursors)
	require.Equal(t, expectSortedCursors, shuffledCursors)
}
