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

	service, err := NewScopeAwareService(&ServiceConfig[*testResource]{
		Backend:       memBackend,
		ResourceKind:  "generic resource",
		PageLimit:     200,
		BackendPrefix: backend.NewKey("generic_prefix"),
		UnmarshalFunc: unmarshalResource,
		MarshalFunc:   marshalResource,
	}, backend.NewKey("scoped"))
	require.NoError(t, err)

	var resources []*testResource

	// Create some unscoped resources
	for nameIndex := range 10 {
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

	// Create all the resources.
	for _, resource := range resources {
		_, err := service.CreateResource(t.Context(), resource)
		require.NoError(t, err)
	}

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
