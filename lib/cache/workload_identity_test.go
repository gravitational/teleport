// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func newWorkloadIdentity(name string) *workloadidentityv1pb.WorkloadIdentity {
	return &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/example",
			},
		},
	}
}

func TestWorkloadIdentity(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*workloadidentityv1pb.WorkloadIdentity]{
		newResource: func(s string) (*workloadidentityv1pb.WorkloadIdentity, error) {
			return newWorkloadIdentity(s), nil
		},

		create: func(ctx context.Context, item *workloadidentityv1pb.WorkloadIdentity) error {
			_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
			items, _, err := p.workloadIdentity.ListWorkloadIdentities(ctx, 0, "", nil)
			return items, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return p.workloadIdentity.DeleteAllWorkloadIdentities(ctx)
		},

		cacheList: func(ctx context.Context, _ int) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
			items, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil)
			return items, trace.Wrap(err)
		},
		cacheGet: p.cache.GetWorkloadIdentity,
	}, withSkipPaginationTest())
}

// TestWorkloadIdentityCacheSorting tests that cache items are sorted.
func TestWorkloadIdentityCacheSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	rs := []struct {
		name     string
		spiffeId string
	}{
		{"test-workload-identity-1", "/test/spiffe/2"},
		{"test-workload-identity-3", "/test/spiffe/1"},
		{"test-workload-identity-2", "/test/spiffe/3"},
	}

	for _, r := range rs {
		id := &workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: r.name,
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: r.spiffeId,
				},
			},
		}

		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, id)
		require.NoError(t, err, "failed to create WorkloadIdentity %q", r.name)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("sort ascending by spiffe_id", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			Sort: &types.SortBy{
				Field:  "spiffe_id",
				IsDesc: false,
			},
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "test-workload-identity-3", results[0].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-1", results[1].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-2", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by spiffe_id", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			Sort: &types.SortBy{
				Field:  "spiffe_id",
				IsDesc: true,
			},
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "test-workload-identity-2", results[0].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-1", results[1].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-3", results[2].GetMetadata().GetName())
	})

	t.Run("sort ascending by name", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil) // empty sort should default to `name:asc`
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "test-workload-identity-1", results[0].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-2", results[1].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-3", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by name", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			Sort: &types.SortBy{
				Field:  "name",
				IsDesc: true,
			},
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "test-workload-identity-3", results[0].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-2", results[1].GetMetadata().GetName())
		require.Equal(t, "test-workload-identity-1", results[2].GetMetadata().GetName())
	})
}

// TestWorkloadIdentityCacheFallback tests that requests fallback to the upstream when the cache is unhealthy.
func TestWorkloadIdentityCacheFallback(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, func(cfg Config) Config {
		cfg.neverOK = true // Force the cache into an unhealthy state
		return ForAuth(cfg)
	})
	t.Cleanup(p.Close)

	_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test-workload-identity-1",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/test/spiffe/1",
			},
		},
	})
	require.NoError(t, err)

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 1)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("supported sort", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			Sort: &types.SortBy{
				Field:  "name",
				IsDesc: false,
			},
		})
		require.NoError(t, err) // asc by name is the only sort supported by the upstream
		require.Len(t, results, 1)
	})

	t.Run("unsupported sort", func(t *testing.T) {
		_, _, err = p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			Sort: &types.SortBy{
				Field:  "name",
				IsDesc: true,
			},
		})
		require.ErrorContains(t, err, "unsupported sort, only name:asc is supported, but got \"name\" (desc = true)")
	})
}

// TestWorkloadIdentityCacheSearchFilter tests that cache items are filtered by search query.
func TestWorkloadIdentityCacheSearchFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for n := range 10 {
		name := "test-workload-identity-" + strconv.Itoa(n)
		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/test/" + strconv.Itoa(n%2) + "/id" + strconv.Itoa(n),
				},
			},
		})
		require.NoError(t, err, "failed to create WorkloadIdentity %q", name)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 10)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
		FilterSearchTerm: "test/1",
	})
	require.NoError(t, err)
	require.Len(t, results, 5)
}
