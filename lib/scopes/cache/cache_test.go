/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/scopes"
)

// item is a a basic helper used for cache testing.
type item struct {
	key   string
	scope string
}

func (i *item) Key() string {
	return i.key
}

func (i *item) Scope() string {
	return i.scope
}

// TestCacheBasics verifies basic functionality of the cache, including insertion, query, and deletion, and
// correct handling of collisions.
func TestCacheBasics(t *testing.T) {
	t.Parallel()

	items := []*item{
		{
			key:   "root-scoped",
			scope: "/",
		},
		{
			key:   "other-root-scoped",
			scope: "/",
		},
		{
			key:   "child-scoped",
			scope: "/child",
		},
		{
			key:   "other-child-scoped",
			scope: "/child",
		},
		{
			key:   "child-sub-scoped",
			scope: "/child/subchild",
		},
		{
			key:   "child-orthogonal",
			scope: "/orthogonal",
		},
	}

	cache, err := New(Config[*item, string]{
		Scope: (*item).Scope,
		Key:   (*item).Key,
	})
	require.NoError(t, err)
	require.Equal(t, 0, cache.Len())

	// verify empty reads of root
	requireEqualItemKeys(t, map[string][]string{}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/")))
	requireEqualItemKeys(t, map[string][]string{}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify empty sub-scope reads
	requireEqualItemKeys(t, map[string][]string{}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))
	requireEqualItemKeys(t, map[string][]string{}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child")))

	for _, item := range items {
		cache.Put(item)
	}

	require.Equal(t, len(items), cache.Len())

	// verify basic policies-applicable-to-resource iteration

	requireEqualItemKeys(t, map[string][]string{
		"/": {"root-scoped", "other-root-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/")))

	requireEqualItemKeys(t, map[string][]string{
		"/":      {"root-scoped", "other-root-scoped"},
		"/child": {"child-scoped", "other-child-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"root-scoped", "other-root-scoped"},
		"/child":          {"child-scoped", "other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	// verify basic resources-subject-to-policy iteration

	requireEqualItemKeys(t, map[string][]string{},
		collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/nonexistent")))

	requireEqualItemKeys(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/child":          {"child-scoped", "other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child")))

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"root-scoped", "other-root-scoped"},
		"/child":          {"child-scoped", "other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify that the concept of policies-applicable-to-resource used by the cache
	// matches up with the definition expected by the scopes package.
	for scope := range cache.PoliciesApplicableToResourceScope("/child/subchild") {
		require.True(t, scopes.ResourceScope("/child/subchild").IsSubjectToPolicyScope(scope.Scope()), "scope=%s", scope.Scope())
	}

	// verify that the concept of resources-subject-to-policy used by the cache
	// matches up with the definition expected by the scopes package.
	for scope := range cache.ResourcesSubjectToPolicyScope("/child") {
		require.True(t, scopes.PolicyScope("/child").AppliesToResourceScope(scope.Scope()), "scope=%s", scope.Scope())
	}

	// verify deletion of a single intermediate item
	cache.Del("child-scoped")
	require.Equal(t, len(items)-1, cache.Len())

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"root-scoped", "other-root-scoped"},
		"/child":          {"other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"root-scoped", "other-root-scoped"},
		"/child":          {"other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify deletion of a single root-scoped item
	cache.Del("root-scoped")
	require.Equal(t, len(items)-2, cache.Len())

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"other-root-scoped"},
		"/child":          {"other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"other-root-scoped"},
		"/child":          {"other-child-scoped"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify full deletion of all contents of an intermediate scope
	cache.Del("other-child-scoped")
	require.Equal(t, len(items)-3, cache.Len())

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"other-root-scoped"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/":               {"other-root-scoped"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verfiy full deletion of all contents of a root scope
	cache.Del("other-root-scoped")
	require.Equal(t, len(items)-4, cache.Len())

	requireEqualItemKeys(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify deletion of leaf scope
	cache.Del("child-sub-scoped")
	require.Equal(t, len(items)-5, cache.Len())

	requireEqualItemKeys(t, map[string][]string{},
		collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/orthogonal": {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify basic re-add
	cache.Put(&item{
		key:   "child-scoped",
		scope: "/child",
	})
	require.Equal(t, len(items)-4, cache.Len())

	requireEqualItemKeys(t, map[string][]string{
		"/child": {"child-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))

	requireEqualItemKeys(t, map[string][]string{
		"/child":      {"child-scoped"},
		"/orthogonal": {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify overwrite of existing item by primary key
	cache.Put(&item{
		key:   "child-scoped",
		scope: "/child/other",
	})
	require.Equal(t, len(items)-4, cache.Len())

	requireEqualItemKeys(t, map[string][]string{},
		collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	requireEqualItemKeys(t, map[string][]string{
		"/child/other": {"child-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/other")))

	requireEqualItemKeys(t, map[string][]string{
		"/child/other": {"child-scoped"},
		"/orthogonal":  {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))
}

// requireEualItemKeys is a workaround because scope/item iteration is currently unordered.
// TODO(fspmarshall): make scope/item iteration ordered.
func requireEqualItemKeys(t *testing.T, expected, actual map[string][]string) {
	t.Helper()
	for _, val := range expected {
		slices.Sort(val)
	}

	for _, val := range actual {
		slices.Sort(val)
	}

	require.Equal(t, expected, actual)
}

// collectScopedItemKeys aggregates a scoped iterator from one of the cache iteration
// methods into a map of scope -> keys.
func collectScopedItemKeys(iterator iter.Seq[ScopedItems[*item]]) map[string][]string {
	itemKeys := make(map[string][]string)
	for scope := range iterator {
		var keys []string
		for item := range scope.Items() {
			keys = append(keys, item.Key())
		}
		itemKeys[scope.Scope()] = keys
	}
	return itemKeys
}
