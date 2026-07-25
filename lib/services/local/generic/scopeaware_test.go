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

package generic

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
)

func newScopedTestResource(sqn scopes.QualifiedName) *testResource {
	r := newTestResource(sqn.Name)
	r.Scope = sqn.Scope
	return r
}

func TestScopeAwareService(t *testing.T) {
	memBackend, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewScopeAwareService(&ScopeAwareServiceConfig[*testResource]{
		Backend:               memBackend,
		ResourceKind:          "generic resource",
		UnscopedBackendPrefix: backend.NewKey("generic_prefix"),
		ScopedBackendPrefix:   backend.NewKey("scoped", "generic_prefix"),
		PageLimit:             200,
		UnmarshalFunc:         unmarshalResource,
		MarshalFunc:           marshalResource,
	})
	require.NoError(t, err)

	var resources []*testResource

	// Create some unscoped resources.
	const numUnscopedResources = 10
	for nameIndex := range numUnscopedResources {
		resources = append(resources, newTestResource(fmt.Sprintf("name%d", nameIndex)))
	}

	// Add a bunch of scoped resources at various scopes.
	for baseScopeIndex := range 10 {
		// Add ten at this base scope.
		for nameIndex := range 10 {
			resources = append(resources, newScopedTestResource(scopes.QualifiedName{
				Scope: fmt.Sprintf("/base%d", baseScopeIndex),
				Name:  fmt.Sprintf("name%d", nameIndex),
			}))
		}

		// Add ten more at each of 10 sub scopes.
		for subScopeIndex := range 10 {
			for nameIndex := range 10 {
				resources = append(resources, newScopedTestResource(scopes.QualifiedName{
					Scope: fmt.Sprintf("/base%d/sub%d", baseScopeIndex, subScopeIndex),
					Name:  fmt.Sprintf("name%d", nameIndex),
				}))
			}
		}
	}

	expectedResourceNames := make(map[scopes.QualifiedName]struct{}, len(resources))
	for _, resource := range resources {
		expectedResourceNames[scopes.QualifiedName{
			Scope: resource.GetScope(),
			Name:  resource.GetName(),
		}] = struct{}{}
	}

	checkExpectedResources := func(t *testing.T, expectedResourceNames map[scopes.QualifiedName]struct{}, resources iter.Seq2[*testResource, error]) {
		t.Helper()

		expected := maps.Clone(expectedResourceNames)

		for resource, err := range resources {
			require.NoError(t, err)

			key := scopes.QualifiedName{
				Scope: resource.GetScope(),
				Name:  resource.GetName(),
			}

			assert.Contains(t, expected, key, "found unexpected resource %v", key)
			delete(expected, key)
		}
		assert.Empty(t, expected, "did not find expected resources")
	}

	expectedResourcesInCursorRange := func(t *testing.T, startKey, endKey string) map[scopes.QualifiedName]struct{} {
		t.Helper()

		expected := make(map[scopes.QualifiedName]struct{})
		for _, resource := range resources {
			cursor := scopes.MakeResourceCursor(resource.GetScope(), resource.GetName())
			if startKey != "" && cursor < startKey {
				continue
			}
			if endKey != "" && cursor >= endKey {
				continue
			}
			expected[scopes.QualifiedName{Scope: resource.GetScope(), Name: resource.GetName()}] = struct{}{}
		}
		return expected
	}

	// Create all the resources.
	for _, resource := range resources {
		_, err := service.CreateResource(t.Context(), resource)
		require.NoError(t, err)
	}

	t.Run("Resources", func(t *testing.T) {
		checkExpectedResources(t, expectedResourceNames, service.Resources(t.Context(), "", ""))

		foundScoped := false
		for resource, err := range service.Resources(t.Context(), "", "") {
			require.NoError(t, err)
			if foundScoped && resource.GetScope() == "" {
				t.Fatal("found unscoped resource after scoped resource")
			}
			foundScoped = foundScoped || resource.GetScope() != ""
		}

		countResources := func(resources iter.Seq2[*testResource, error]) int {
			t.Helper()
			var count int
			for _, err := range resources {
				require.NoError(t, err)
				count++
			}
			return count
		}

		// The scoped start cursor is the boundary between unscoped resources and
		// scoped resources in the unified logical resource stream.
		require.Equal(t, numUnscopedResources,
			countResources(service.Resources(t.Context(), "", scopes.ResourceCursorScopedStart())))
		require.Equal(t, len(resources)-numUnscopedResources,
			countResources(service.Resources(t.Context(), scopes.ResourceCursorScopedStart(), "")))

		// Verify a range that starts in unscoped resources and ends in scoped
		// resources, forcing Resources to concatenate both underlying services.
		scopedEnd := scopes.MakeResourceCursor("/base1", "name0")
		checkExpectedResources(t,
			expectedResourcesInCursorRange(t, "name5", scopedEnd),
			service.Resources(t.Context(), "name5", scopedEnd),
		)
	})

	// Check all resources can be listed, at various page sizes.
	for _, pageSize := range []int{0, 1, 3, 5, 10, 11, 99, 100, 101} {
		t.Run(fmt.Sprintf("pageSize=%d", pageSize), func(t *testing.T) {
			t.Run("unfiltered", func(t *testing.T) {
				resourceIter := clientutils.ResourcesWithPageSize(t.Context(), service.ListResources, pageSize)
				checkExpectedResources(t, expectedResourceNames, resourceIter)
			})

			t.Run("filtered", func(t *testing.T) {
				iter := clientutils.ResourcesWithPageSize(t.Context(), func(ctx context.Context, pageSize int, nextPageToken string) ([]*testResource, string, error) {
					return service.ListResourcesWithFilter(ctx, pageSize, nextPageToken, func(resource *testResource) bool {
						return strings.HasPrefix(resource.GetScope(), "/base7/")
					})
				}, pageSize)
				count := 0
				for resource, err := range iter {
					require.NoError(t, err)
					require.True(t, strings.HasPrefix(resource.GetScope(), "/base7/"))
					count++
				}
				require.Equal(t, 100, count)

			})
		})

	}

	// Check that unscoped resources sort before scoped resources.
	foundScoped := false
	for resource, err := range clientutils.Resources(t.Context(), service.ListResources) {
		require.NoError(t, err)
		scope := resource.GetScope()
		if foundScoped && scope == "" {
			t.Fatal("found unscoped resource after scoped resource")
		}
		foundScoped = foundScoped || scope != ""
	}

	for resourceName := range expectedResourceNames {
		// Get should work.
		resource, err := service.GetResource(t.Context(), resourceName)
		require.NoError(t, err)

		// Update should work.
		resource.Spec.PropA = "updated"
		resource, err = service.UpdateResource(t.Context(), resource)
		require.NoError(t, err)
		require.Equal(t, "updated", resource.Spec.PropA)

		// Try ConditionalUpdate with incorrect revision.
		rev := resource.Metadata.Revision
		resource.Metadata.Revision = ""
		_, err = service.ConditionalUpdateResource(t.Context(), resource)
		require.Error(t, err)

		// ConditionalUpdate with correct revision.
		resource.Metadata.Revision = rev
		resource.Spec.PropA = "conditional_updated"
		resource, err = service.ConditionalUpdateResource(t.Context(), resource)
		require.NoError(t, err)
		require.Equal(t, "conditional_updated", resource.Spec.PropA)

		// Delete the resource and Get should fail.
		err = service.DeleteResource(t.Context(), resourceName)
		require.NoError(t, err)
		_, err = service.GetResource(t.Context(), resourceName)
		require.Error(t, err)

		// Upsert the resource.
		resource.Spec.PropA = "upserted"
		resource, err = service.UpsertResource(t.Context(), resource)
		require.NoError(t, err)
		require.Equal(t, "upserted", resource.Spec.PropA)
	}

	// Make sure all the expected resources are still there.
	resourceIter := clientutils.Resources(t.Context(), service.ListResources)
	checkExpectedResources(t, expectedResourceNames, resourceIter)

	// Delete all resources and make sure they're gone.
	err = service.DeleteAllResources(t.Context())
	require.NoError(t, err)
	page, _, err := service.ListResources(t.Context(), 1, "")
	require.NoError(t, err)
	require.Empty(t, page)
}

func newScopedOnlyServiceForTest(t *testing.T) *ScopeAwareService[*testResource] {
	t.Helper()

	memBackend, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewScopeAwareService(&ScopeAwareServiceConfig[*testResource]{
		ScopedOnly:          true,
		Backend:             memBackend,
		ResourceKind:        "generic resource",
		ScopedBackendPrefix: backend.NewKey(testScopedTopPrefix, testUnscopedPrefix),
		PageLimit:           200,
		MarshalFunc:         marshalResource,
		UnmarshalFunc:       unmarshalResource,
	})
	require.NoError(t, err)
	return service
}

func newScopedTestResourceWithBody(sqn scopes.QualifiedName) *testResource {
	r := newScopedTestResource(sqn)
	r.Spec.PropA = specDataFor(sqn)
	return r
}

func requireTestResourceBody(t *testing.T, want scopes.QualifiedName, got *testResource) {
	t.Helper()
	require.Equal(t, want.Name, got.GetName())
	require.Equal(t, want.Scope, got.GetScope())
	require.Equal(t, specDataFor(want), got.Spec.PropA)
}

func scopedNames(resources []*testResource) []scopes.QualifiedName {
	out := make([]scopes.QualifiedName, 0, len(resources))
	for _, r := range resources {
		out = append(out, scopes.QualifiedName{Scope: r.GetScope(), Name: r.GetName()})
	}
	return out
}

func collectScopedStream(t *testing.T, seq iter.Seq2[*testResource, error]) []*testResource {
	t.Helper()
	var out []*testResource
	for r, err := range seq {
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

func TestScopeAwareService_ScopedOnly(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty scope on every operation", func(t *testing.T) {
		t.Parallel()
		svc := newScopedOnlyServiceForTest(t)
		ctx := t.Context()
		unscoped := scopes.QualifiedName{Name: "foo"}

		_, err := svc.CreateResource(ctx, newScopedTestResource(unscoped))
		require.True(t, trace.IsBadParameter(err), "create: %v", err)

		_, err = svc.UpsertResource(ctx, newScopedTestResource(unscoped))
		require.True(t, trace.IsBadParameter(err), "upsert: %v", err)

		_, err = svc.ConditionalUpdateResource(ctx, newScopedTestResource(unscoped))
		require.True(t, trace.IsBadParameter(err), "update: %v", err)

		_, err = svc.GetResource(ctx, unscoped)
		require.True(t, trace.IsBadParameter(err), "get: %v", err)

		err = svc.DeleteResource(ctx, unscoped)
		require.True(t, trace.IsBadParameter(err), "delete: %v", err)

		_, err = svc.WithScopedResourcePrefix(unscoped)
		require.True(t, trace.IsBadParameter(err), "with scoped resource prefix: %v", err)

		_, err = svc.WithScopePrefix("")
		require.True(t, trace.IsBadParameter(err), "with scope prefix: %v", err)
	})

	t.Run("CRUD", func(t *testing.T) {
		t.Parallel()
		svc := newScopedOnlyServiceForTest(t)
		ctx := t.Context()
		qn := scopes.QualifiedName{Scope: "/security", Name: "foo"}

		created, err := svc.CreateResource(ctx, newScopedTestResourceWithBody(qn))
		require.NoError(t, err)

		got, err := svc.GetResource(ctx, qn)
		require.NoError(t, err)
		requireTestResourceBody(t, qn, got)

		updated := newScopedTestResource(qn)
		updated.Spec.PropA = "updated!"
		updated.Metadata.Revision = created.Metadata.Revision
		updated, err = svc.ConditionalUpdateResource(ctx, updated)
		require.NoError(t, err)
		require.NotEqual(t, created.Metadata.Revision, updated.Metadata.Revision)

		got, err = svc.GetResource(ctx, qn)
		require.NoError(t, err)
		require.Equal(t, "updated!", got.Spec.PropA)

		_, err = svc.ConditionalUpdateResource(ctx, created)
		require.True(t, trace.IsCompareFailed(err))

		upserted, err := svc.UpsertResource(ctx, created)
		require.NoError(t, err)
		requireTestResourceBody(t, qn, upserted)

		require.NoError(t, svc.DeleteResource(ctx, qn))
		_, err = svc.GetResource(ctx, qn)
		require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)
	})

	t.Run("same name across scopes does not conflict", func(t *testing.T) {
		t.Parallel()
		svc := newScopedOnlyServiceForTest(t)
		ctx := t.Context()
		a := scopes.QualifiedName{Scope: "/a", Name: "shared"}
		b := scopes.QualifiedName{Scope: "/b", Name: "shared"}
		_, err := svc.CreateResource(ctx, newScopedTestResourceWithBody(a))
		require.NoError(t, err)
		_, err = svc.CreateResource(ctx, newScopedTestResourceWithBody(b))
		require.NoError(t, err)

		gotA, err := svc.GetResource(ctx, a)
		require.NoError(t, err)
		requireTestResourceBody(t, a, gotA)
		gotB, err := svc.GetResource(ctx, b)
		require.NoError(t, err)
		requireTestResourceBody(t, b, gotB)
	})

	t.Run("Resources bounded ranges", func(t *testing.T) {
		t.Parallel()
		svc := newScopedOnlyServiceForTest(t)
		ctx := t.Context()
		resources := []scopes.QualifiedName{
			{Scope: "/a", Name: "1"},
			{Scope: "/b", Name: "2"},
			{Scope: "/c", Name: "3"},
		}
		for _, qn := range resources {
			_, err := svc.CreateResource(ctx, newScopedTestResource(qn))
			require.NoError(t, err)
		}

		// A scoped-only service has no unscoped resources, so ranges ending
		// before the scoped resource range must be empty.
		require.Empty(t, collectScopedStream(t, svc.Resources(ctx, "", "name5")))
		require.Empty(t, collectScopedStream(t, svc.Resources(ctx, "", scopes.ResourceCursorScopedStart())))

		start := scopes.MakeResourceCursor("/b", "2")
		end := scopes.MakeResourceCursor("/c", "3")
		require.Equal(t,
			[]scopes.QualifiedName{{Scope: "/b", Name: "2"}},
			scopedNames(collectScopedStream(t, svc.Resources(ctx, start, end))),
		)
	})

	t.Run("ListResourcesWithFilter pagination", func(t *testing.T) {
		t.Parallel()
		svc := newScopedOnlyServiceForTest(t)
		ctx := t.Context()
		want := []scopes.QualifiedName{
			{Scope: "/a", Name: "1"},
			{Scope: "/b", Name: "2"},
			{Scope: "/c", Name: "3"},
		}
		for _, qn := range want {
			_, err := svc.CreateResource(ctx, newScopedTestResource(qn))
			require.NoError(t, err)
		}

		// Single full page.
		got, next, err := svc.ListResourcesWithFilter(ctx, 100, "", func(*testResource) bool { return true })
		require.NoError(t, err)
		require.Empty(t, next)
		require.Equal(t, want, scopedNames(got))

		// One item per page.
		var paged []*testResource
		token := ""
		for {
			page, nextToken, err := svc.ListResourcesWithFilter(ctx, 1, token, func(*testResource) bool { return true })
			require.NoError(t, err)
			if nextToken != "" {
				require.True(t, scopes.IsScopedResourceCursor(nextToken))
			}
			paged = append(paged, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		require.Equal(t, want, scopedNames(paged))

		// Default page size.
		got, _, err = svc.ListResourcesWithFilter(ctx, 0, "", func(*testResource) bool { return true })
		require.NoError(t, err)
		require.Len(t, got, 3)

		// A scoped-only service must reject non-empty unscoped tokens. Such a
		// token could only come from a different service mode and would otherwise
		// be silently interpreted as a scoped service relative key.
		_, _, err = svc.ListResourcesWithFilter(ctx, 100, "legacy", func(*testResource) bool { return true })
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)

		require.NoError(t, svc.DeleteAllResources(ctx))
		require.Empty(t, collectScopedStream(t, svc.Resources(ctx, "", "")))
	})
}
