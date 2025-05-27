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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/scopes"
)

// item is a a basic helper used for cache testing.
type item[K any] struct {
	key   K
	scope string
}

func (i item[K]) Key() K {
	return i.key
}

func (i item[K]) Scope() string {
	return i.scope
}

// TestCacheConcurrency verifies the basic expected concurrency behavior of the cache.
func TestCacheConcurrency(t *testing.T) {
	t.Parallel()

	items := []item[int]{
		{1, "/"},
		{2, "/aa"},
		{3, "/aa/bb"},
		{4, "/aa/bb/cc"},
	}

	cache, err := New(Config[item[int], int]{
		Scope: (item[int]).Scope,
		Key:   (item[int]).Key,
	})
	require.NoError(t, err)

	for _, item := range items {
		cache.Put(item)
	}

	// lockstepC is used to force the background queries to progress in lockstep
	lockstepC := make(chan struct{})

	// proceedC is used to signal the leading background query to proceed
	proceedC := make(chan struct{})

	go func() {
		// perform a policy application query that will match all items
		for _ = range cache.PoliciesApplicableToResourceScope("/aa/bb/cc") {
			lockstepC <- struct{}{} // block until the second query has caught up
			<-proceedC              // wait for the main test routine to unblock us
		}
	}()

	go func() {
		// perform a resource subjugation query that will match all items
		for _ = range cache.ResourcesSubjectToPolicyScope("/") {
			<-lockstepC // wait until we get the signal from the first query
		}
	}()

	// send proceed signal (ensures that background queries have got their first
	// item and will start acquiring the next, ensuring that both queries are "in progress").
	select {
	case proceedC <- struct{}{}:
	case <-time.After(time.Second * 5):
		t.Fatalf("timed out waiting for proceed signal send")
	}

	// start a writer now that we know we have a "happens after" relationship to the
	// background queries.
	putDone := make(chan struct{})
	go func() {
		// perform a write that will block until the background queries are done
		cache.Put(item[int]{5, "/aa/bb/cc/dd"})
		close(putDone)
	}()

	delDone := make(chan struct{})
	go func() {
		// perform a delete that will block until the background queries are done
		cache.Del(1)
		close(delDone)
	}()

	// we can't really guarantee that the background writer is waiting, but we can
	// be reasonably sure by stepping through multiple qery cycles and asserting that
	// the writer hasn't completed at each iteration.
	for i := 0; i < 3; i++ {
		// perform initial check to verify that write hasn't succeeded (racy)
		select {
		case <-putDone:
			t.Fatal("put was able to proceed while queries were in progress")
		case <-delDone:
			t.Fatal("del was able to proceed while queries were in progress")
		case <-time.After(time.Millisecond * 333):
		}

		// progress both background queries to start processing their next items
		select {
		case proceedC <- struct{}{}:
		case <-time.After(time.Second * 5):
			t.Fatalf("timed out waiting for proceed signal send")
		}
	}

	// our final proceed signal should have unblocked the queries for the last time,
	// the writers should now succeed.

	select {
	case <-putDone:
	case <-time.After(time.Second * 5):
		t.Fatalf("timed out waiting for put to complete")
	}

	select {
	case <-delDone:
	case <-time.After(time.Second * 5):
		t.Fatalf("timed out waiting for del to complete")
	}
}

// TestCursorScenarios verifies the expected output of the cache given various cursor values.
func TestCursorScenarios(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name                 string
		items                []item[int]
		scope                string
		cursor               Cursor[int]
		policiesApplicableTo []item[int]
		resourcesSubjectTo   []item[int]
	}{
		{
			name: "basic",
			items: []item[int]{
				{9, "/"},
				{8, "/aa"},
				{7, "/aa"},
				{6, "/aa"},
				{5, "/xx"},
				{4, "/aa/bb"},
				{3, "/aa/bb"},
				{2, "/aa/bb"},
				{1, "/xx/yy"},
			},
			scope: "/aa",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   7,
			},
			policiesApplicableTo: []item[int]{
				{7, "/aa"},
				{8, "/aa"},
			},
			resourcesSubjectTo: []item[int]{
				{7, "/aa"},
				{8, "/aa"},
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/aa/bb"},
			},
		},
		{
			name: "missing item at beginning of scope range",
			items: []item[int]{
				{9, "/"},
				{8, "/aa"},
				{7, "/aa"},
				{5, "/xx"},
				{4, "/aa/bb"},
				{3, "/aa/bb"},
				{2, "/aa/bb"},
				{1, "/xx/yy"},
			},
			scope: "/aa",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   6,
			},
			policiesApplicableTo: []item[int]{
				{7, "/aa"},
				{8, "/aa"},
			},
			resourcesSubjectTo: []item[int]{
				{7, "/aa"},
				{8, "/aa"},
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/aa/bb"},
			},
		},
		{
			name: "missing item in middle of scope range",
			items: []item[int]{
				{9, "/"},
				{8, "/aa"},
				{6, "/aa"},
				{5, "/xx"},
				{4, "/aa/bb"},
				{3, "/aa/bb"},
				{2, "/aa/bb"},
				{1, "/xx/yy"},
			},
			scope: "/aa",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   7,
			},
			policiesApplicableTo: []item[int]{
				{8, "/aa"},
			},
			resourcesSubjectTo: []item[int]{
				{8, "/aa"},
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/aa/bb"},
			},
		},
		{
			name: "missing item at end of scope range",
			items: []item[int]{
				{9, "/"},
				{7, "/aa"},
				{6, "/aa"},
				{5, "/xx"},
				{4, "/aa/bb"},
				{3, "/aa/bb"},
				{2, "/aa/bb"},
				{1, "/xx/yy"},
			},
			scope: "/aa",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   8,
			},
			policiesApplicableTo: nil,
			resourcesSubjectTo: []item[int]{
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/aa/bb"},
			},
		},
		{
			name: "deep scopes with high cursor",
			items: []item[int]{
				{0, "/"},
				{1, "/"},
				{2, "/aa"},
				{3, "/aa"},
				{4, "/aa/bb"},
				{5, "/aa/bb"},
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{10, "/aa/bb/cc/dd/ee"},
				{11, "/aa/bb/cc/dd/ee"},
			},
			scope: "/aa/bb/cc",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   3,
			},
			policiesApplicableTo: []item[int]{
				{3, "/aa"},
				{4, "/aa/bb"},
				{5, "/aa/bb"},
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
			},
			resourcesSubjectTo: []item[int]{
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{10, "/aa/bb/cc/dd/ee"},
				{11, "/aa/bb/cc/dd/ee"},
			},
		},
		{
			name: "deep scopes with low cursor",
			items: []item[int]{
				{0, "/"},
				{1, "/"},
				{2, "/aa"},
				{3, "/aa"},
				{4, "/aa/bb"},
				{5, "/aa/bb"},
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{10, "/aa/bb/cc/dd/ee"},
				{11, "/aa/bb/cc/dd/ee"},
			},
			scope: "/aa/bb/cc",
			cursor: Cursor[int]{
				Scope: "/aa/bb/cc",
				Key:   6,
			},
			policiesApplicableTo: []item[int]{
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
			},
			resourcesSubjectTo: []item[int]{
				{6, "/aa/bb/cc"},
				{7, "/aa/bb/cc"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{10, "/aa/bb/cc/dd/ee"},
				{11, "/aa/bb/cc/dd/ee"},
			},
		},
		{
			name: "low scope high cursor sparse",
			items: []item[int]{
				{0, "/"},
				{1, "/"},
				{4, "/aa/bb"},
				{5, "/aa/bb"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{12, "/aa/bb/cc/dd/ee/ff"},
				{13, "/aa/bb/cc/dd/ee/ff"},
			},
			scope: "/aa/bb/cc",
			cursor: Cursor[int]{
				Scope: "/aa",
				Key:   2,
			},
			policiesApplicableTo: []item[int]{
				{4, "/aa/bb"},
				{5, "/aa/bb"},
			},
			resourcesSubjectTo: []item[int]{
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{12, "/aa/bb/cc/dd/ee/ff"},
				{13, "/aa/bb/cc/dd/ee/ff"},
			},
		},
		{
			name: "high scope low cursor sparse",
			items: []item[int]{
				{0, "/"},
				{1, "/"},
				{4, "/aa/bb"},
				{5, "/aa/bb"},
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{12, "/aa/bb/cc/dd/ee/ff"},
				{13, "/aa/bb/cc/dd/ee/ff"},
			},
			scope: "/aa",
			cursor: Cursor[int]{
				Scope: "/aa/bb/cc",
				Key:   7,
			},
			policiesApplicableTo: nil,
			resourcesSubjectTo: []item[int]{
				{8, "/aa/bb/cc/dd"},
				{9, "/aa/bb/cc/dd"},
				{12, "/aa/bb/cc/dd/ee/ff"},
				{13, "/aa/bb/cc/dd/ee/ff"},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New(Config[item[int], int]{
				Scope: (item[int]).Scope,
				Key:   (item[int]).Key,
			})
			require.NoError(t, err)

			for _, item := range tt.items {
				cache.Put(item)
			}

			// verify policies-applicable-to-resource iteration
			var policiesApplicableTo []item[int]
			for scope := range cache.PoliciesApplicableToResourceScope(tt.scope, WithCursor(tt.cursor)) {
				for item := range scope.Items() {
					policiesApplicableTo = append(policiesApplicableTo, item)
				}
			}

			require.Equal(t, tt.policiesApplicableTo, policiesApplicableTo)

			// verify resources-subject-to-policy iteration
			var resourcesSubjectTo []item[int]
			for scope := range cache.ResourcesSubjectToPolicyScope(tt.scope, WithCursor(tt.cursor)) {
				for item := range scope.Items() {
					resourcesSubjectTo = append(resourcesSubjectTo, item)
				}
			}

			require.Equal(t, tt.resourcesSubjectTo, resourcesSubjectTo)
		})
	}
}

// TestCursorPagination verifies the expected behavior of the ordering and resumption behavior of cache queries
// using basic pagination logic/cursor construction.
func TestCursorPagination(t *testing.T) {
	t.Parallel()

	type expected struct {
		firstPage []item[int]
		remaining []item[int]
	}

	tts := []struct {
		name                 string
		items                []item[int]
		scope                string
		limit                int
		policiesApplicableTo expected
		resourcesSubjectTo   expected
	}{
		{
			name:  "empty root",
			items: nil,
			scope: "/",
			limit: 2,
			policiesApplicableTo: expected{
				firstPage: nil,
				remaining: nil,
			},
			resourcesSubjectTo: expected{
				firstPage: nil,
				remaining: nil,
			},
		},
		{
			name:  "empty child",
			items: nil,
			scope: "/child",
			limit: 2,
			policiesApplicableTo: expected{
				firstPage: nil,
				remaining: nil,
			},
			resourcesSubjectTo: expected{
				firstPage: nil,
				remaining: nil,
			},
		},
		{
			name: "basic root",
			items: []item[int]{
				{1, "/"},
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/cc"},
			},
			scope: "/",
			limit: 2,
			policiesApplicableTo: expected{
				firstPage: []item[int]{
					{1, "/"},
				},
				remaining: nil,
			},
			resourcesSubjectTo: expected{
				firstPage: []item[int]{
					{1, "/"},
					{2, "/aa/bb"},
				},
				remaining: []item[int]{
					{3, "/aa/bb"},
					{4, "/cc"},
				},
			},
		},
		{
			name: "basic child",
			items: []item[int]{
				{1, "/"},
				{2, "/aa/bb"},
				{3, "/aa/bb"},
				{4, "/cc"},
			},
			scope: "/aa/bb",
			limit: 2,
			policiesApplicableTo: expected{
				firstPage: []item[int]{
					{1, "/"},
					{2, "/aa/bb"},
				},
				remaining: []item[int]{
					{3, "/aa/bb"},
				},
			},
			resourcesSubjectTo: expected{
				firstPage: []item[int]{
					{2, "/aa/bb"},
					{3, "/aa/bb"},
				},
				remaining: nil,
			},
		},
		{
			name: "high depth",
			items: []item[int]{
				{1, "/"},
				{2, "/aa"},
				{3, "/aa/bb"},
				{4, "/aa/bb/cc"},
				{5, "/aa/bb/cc/dd"},
				{6, "/aa/bb/cc/dd/ee"},
			},
			scope: "/aa/bb/cc",
			limit: 2,
			policiesApplicableTo: expected{
				firstPage: []item[int]{
					{1, "/"},
					{2, "/aa"},
				},
				remaining: []item[int]{
					{3, "/aa/bb"},
					{4, "/aa/bb/cc"},
				},
			},
			resourcesSubjectTo: expected{
				firstPage: []item[int]{
					{4, "/aa/bb/cc"},
					{5, "/aa/bb/cc/dd"},
				},
				remaining: []item[int]{
					{6, "/aa/bb/cc/dd/ee"},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New(Config[item[int], int]{
				Scope: (item[int]).Scope,
				Key:   (item[int]).Key,
			})
			require.NoError(t, err)

			for _, item := range tt.items {
				cache.Put(item)
			}

			// verify policies-applicable-to-resource iteration
			var firstPage []item[int]
			var remaining []item[int]
			var cursor Cursor[int]
		PolicyApplicationOuter:
			for scope := range cache.PoliciesApplicableToResourceScope(tt.scope) {
				for item := range scope.Items() {
					if len(firstPage) == tt.limit {
						cursor = Cursor[int]{
							Scope: scope.Scope(),
							Key:   item.Key(),
						}
						break PolicyApplicationOuter
					}
					firstPage = append(firstPage, item)
				}
			}

			require.Equal(t, tt.policiesApplicableTo.firstPage, firstPage)

			if !cursor.IsZero() {
				for scope := range cache.PoliciesApplicableToResourceScope(tt.scope, WithCursor(cursor)) {
					for item := range scope.Items() {
						remaining = append(remaining, item)
					}
				}
			}

			require.Equal(t, tt.policiesApplicableTo.remaining, remaining)

			// verify resources-subject-to-policy iteration

			firstPage = nil
			remaining = nil
			cursor = Cursor[int]{}

		ResourceSubjugationOuter:
			for scope := range cache.ResourcesSubjectToPolicyScope(tt.scope) {
				for item := range scope.Items() {
					if len(firstPage) == tt.limit {
						cursor = Cursor[int]{
							Scope: scope.Scope(),
							Key:   item.Key(),
						}
						break ResourceSubjugationOuter
					}
					firstPage = append(firstPage, item)
				}
			}

			require.Equal(t, tt.resourcesSubjectTo.firstPage, firstPage)

			if !cursor.IsZero() {
				for scope := range cache.ResourcesSubjectToPolicyScope(tt.scope, WithCursor(cursor)) {
					for item := range scope.Items() {
						remaining = append(remaining, item)
					}
				}
			}

			require.Equal(t, tt.resourcesSubjectTo.remaining, remaining)
		})
	}
}

// TestCacheOperations verifies basic functionality of the cache, including insertion, query, and deletion, and
// correct handling of collisions.
func TestCacheOperations(t *testing.T) {
	t.Parallel()

	items := []item[string]{
		{
			key:   "root-scoped",
			scope: "/",
		},
		{
			key:   "root-scoped-other",
			scope: "/",
		},
		{
			key:   "child-scoped",
			scope: "/child",
		},
		{
			key:   "child-scoped-other",
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

	cache, err := New(Config[item[string], string]{
		Scope: (item[string]).Scope,
		Key:   (item[string]).Key,
	})
	require.NoError(t, err)
	require.Equal(t, 0, cache.Len())

	// verify empty reads of root
	require.Equal(t, map[string][]string{}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/")))
	require.Equal(t, map[string][]string{}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify empty sub-scope reads
	require.Equal(t, map[string][]string{}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))
	require.Equal(t, map[string][]string{}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child")))

	for _, item := range items {
		cache.Put(item)
	}

	require.Equal(t, len(items), cache.Len())

	// verify basic policies-applicable-to-resource iteration

	require.Equal(t, map[string][]string{
		"/": {"root-scoped", "root-scoped-other"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/")))

	require.Equal(t, map[string][]string{
		"/":      {"root-scoped", "root-scoped-other"},
		"/child": {"child-scoped", "child-scoped-other"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped", "root-scoped-other"},
		"/child":          {"child-scoped", "child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	// verify basic resources-subject-to-policy iteration

	require.Equal(t, map[string][]string{},
		collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/nonexistent")))

	require.Equal(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/child":          {"child-scoped", "child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/child")))

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped", "root-scoped-other"},
		"/child":          {"child-scoped", "child-scoped-other"},
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

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped", "root-scoped-other"},
		"/child":          {"child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped", "root-scoped-other"},
		"/child":          {"child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify deletion of a single root-scoped item
	cache.Del("root-scoped")
	require.Equal(t, len(items)-2, cache.Len())

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped-other"},
		"/child":          {"child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped-other"},
		"/child":          {"child-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify full deletion of all contents of an intermediate scope
	cache.Del("child-scoped-other")
	require.Equal(t, len(items)-3, cache.Len())

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/":               {"root-scoped-other"},
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verfiy full deletion of all contents of a root scope
	cache.Del("root-scoped-other")
	require.Equal(t, len(items)-4, cache.Len())

	require.Equal(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/child/subchild": {"child-sub-scoped"},
		"/orthogonal":     {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify deletion of leaf scope
	cache.Del("child-sub-scoped")
	require.Equal(t, len(items)-5, cache.Len())

	require.Equal(t, map[string][]string{},
		collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/orthogonal": {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify basic re-add
	cache.Put(item[string]{
		key:   "child-scoped",
		scope: "/child",
	})
	require.Equal(t, len(items)-4, cache.Len())

	require.Equal(t, map[string][]string{
		"/child": {"child-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child")))

	require.Equal(t, map[string][]string{
		"/child":      {"child-scoped"},
		"/orthogonal": {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))

	// verify overwrite of existing item by primary key
	cache.Put(item[string]{
		key:   "child-scoped",
		scope: "/child/other",
	})
	require.Equal(t, len(items)-4, cache.Len())

	require.Equal(t, map[string][]string{},
		collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/subchild")))

	require.Equal(t, map[string][]string{
		"/child/other": {"child-scoped"},
	}, collectScopedItemKeys(cache.PoliciesApplicableToResourceScope("/child/other")))

	require.Equal(t, map[string][]string{
		"/child/other": {"child-scoped"},
		"/orthogonal":  {"child-orthogonal"},
	}, collectScopedItemKeys(cache.ResourcesSubjectToPolicyScope("/")))
}

// collectScopedItemKeys aggregates a scoped iterator from one of the cache iteration
// methods into a map of scope -> keys.
func collectScopedItemKeys[K any](iterator iter.Seq[ScopedItems[item[K]]]) map[string][]K {
	itemKeys := make(map[string][]K)
	for scope := range iterator {
		var keys []K
		for item := range scope.Items() {
			keys = append(keys, item.Key())
		}
		itemKeys[scope.Scope()] = keys
	}
	return itemKeys
}
