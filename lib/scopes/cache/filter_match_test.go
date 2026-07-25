/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cache

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

// newFilterMatchCache returns a cache pre-populated with items that exercise the filter-matching logic.
func newFilterMatchCache(t *testing.T) *Cache[item[int], int] {
	t.Helper()
	c, err := New(Config[item[int], int]{
		Scope: (item[int]).Scope,
		Key:   (item[int]).Key,
	})
	require.NoError(t, err)

	filterMatchItems := []item[int]{
		{1, "/"},
		{2, "/aa"},
		{3, "/aa/bb"},
		{4, "/aa/bb/cc"},
		{5, "/aa/cc"},
	}

	for _, it := range filterMatchItems {
		c.Put(it)
	}
	return c
}

// TestCacheExactScope verifies that ExactScope yields only the members at exactly the given scope.
func TestCacheExactScope(t *testing.T) {
	t.Parallel()
	c := newFilterMatchCache(t)

	tests := []struct {
		scope string
		want  map[string][]int
	}{
		{scope: "/aa/bb", want: map[string][]int{"/aa/bb": {3}}},
		{scope: "/", want: map[string][]int{"/": {1}}},
		{scope: "/aa/bb/cc", want: map[string][]int{"/aa/bb/cc": {4}}},
		{scope: "/nonexistent", want: map[string][]int{}},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			require.Equal(t, tt.want, collectScopedItemKeys(c.ExactScope(tt.scope)))
		})
	}
}

// TestResourcesMatchingScopeFilter verifies that the primary-filter mode is mapped to the correct cache
// traversal, including the wildcard (nil/unspecified/all) and unscoped (matches-nothing) cases.
func TestResourcesMatchingScopeFilter(t *testing.T) {
	t.Parallel()
	c := newFilterMatchCache(t)

	const filterScope = "/aa/bb"
	all := map[string][]int{"/": {1}, "/aa": {2}, "/aa/bb": {3}, "/aa/bb/cc": {4}, "/aa/cc": {5}}

	tests := []struct {
		name   string
		filter *scopesv1.Filter
		want   map[string][]int
	}{
		{
			name:   "nil is wildcard",
			filter: nil,
			want:   all,
		},
		{
			name:   "unspecified is wildcard",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSPECIFIED}.Build(),
			want:   all,
		},
		{
			name:   "all is wildcard",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			want:   all,
		},
		{
			name:   "exact",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: filterScope}.Build(),
			want:   map[string][]int{"/aa/bb": {3}},
		},
		{
			name:   "descendants",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: filterScope}.Build(),
			want:   map[string][]int{"/aa/bb": {3}, "/aa/bb/cc": {4}},
		},
		{
			name:   "ancestors",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: filterScope}.Build(),
			want:   map[string][]int{"/": {1}, "/aa": {2}, "/aa/bb": {3}},
		},
		{
			name:   "relatives",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_RELATIVES, Scope: filterScope}.Build(),
			want:   map[string][]int{"/": {1}, "/aa": {2}, "/aa/bb": {3}, "/aa/bb/cc": {4}},
		},
		{
			name:   "unscoped matches nothing",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			want:   map[string][]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter, err := c.ResourcesMatchingScopeFilter(tt.filter)
			require.NoError(t, err)
			require.Equal(t, tt.want, collectScopedItemKeys(getter))
		})
	}
}

// TestResourcesMatchingScopeFilterInvalid verifies that a malformed filter is rejected rather than silently
// mishandled.
func TestResourcesMatchingScopeFilterInvalid(t *testing.T) {
	t.Parallel()
	c := newFilterMatchCache(t)

	// EXACT requires a non-empty scope.
	_, err := c.ResourcesMatchingScopeFilter(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build())
	require.Error(t, err)

	// UNSCOPED requires an empty scope.
	_, err = c.ResourcesMatchingScopeFilter(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED, Scope: "/aa"}.Build())
	require.Error(t, err)
}
