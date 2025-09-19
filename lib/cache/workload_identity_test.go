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
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
			return p.workloadIdentity.ListWorkloadIdentities(ctx, pageSize, pageToken, nil)
		},
		deleteAll: func(ctx context.Context) error {
			return p.workloadIdentity.DeleteAllWorkloadIdentities(ctx)
		},

		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
			return p.cache.ListWorkloadIdentities(ctx, 0, "", nil)
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
		spiffeID string
	}{
		{"test-workload-identity-1", "/test/spiffe/2"},
		{"test-workload-identity-3", "/test/spiffe/1"},
		{"test-workload-identity-2", "/test/spiffe/3"},
		{"Test-workload-identity-4", "/Test/spiffe/2"},
		{"Test-workload-identity-5", "/Test/spiffe/1"},
		{"Test-workload-identity-6", "/Test/spiffe/3"},
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
					Id: r.spiffeID,
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
		require.Len(t, results, 6)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("sort ascending by spiffe_id", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortField: "spiffe_id",
			SortDesc:  false,
		})
		require.NoError(t, err)
		require.Len(t, results, 6)
		assert.Equal(t, "/Test/spiffe/1", results[0].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/test/spiffe/1", results[1].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/Test/spiffe/2", results[2].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/test/spiffe/2", results[3].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/Test/spiffe/3", results[4].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/test/spiffe/3", results[5].GetSpec().GetSpiffe().GetId())
	})

	t.Run("sort descending by spiffe_id", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortField: "spiffe_id",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 6)
		assert.Equal(t, "/test/spiffe/3", results[0].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/Test/spiffe/3", results[1].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/test/spiffe/2", results[2].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/Test/spiffe/2", results[3].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/test/spiffe/1", results[4].GetSpec().GetSpiffe().GetId())
		assert.Equal(t, "/Test/spiffe/1", results[5].GetSpec().GetSpiffe().GetId())
	})

	t.Run("sort ascending by name", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", nil) // empty sort should default to `name:asc`
		require.NoError(t, err)
		require.Len(t, results, 6)
		assert.Equal(t, "Test-workload-identity-4", results[0].GetMetadata().GetName())
		assert.Equal(t, "Test-workload-identity-5", results[1].GetMetadata().GetName())
		assert.Equal(t, "Test-workload-identity-6", results[2].GetMetadata().GetName())
		assert.Equal(t, "test-workload-identity-1", results[3].GetMetadata().GetName())
		assert.Equal(t, "test-workload-identity-2", results[4].GetMetadata().GetName())
		assert.Equal(t, "test-workload-identity-3", results[5].GetMetadata().GetName())
	})

	t.Run("sort descending by name", func(t *testing.T) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortField: "name",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 6)
		assert.Equal(t, "test-workload-identity-3", results[0].GetMetadata().GetName())
		assert.Equal(t, "test-workload-identity-2", results[1].GetMetadata().GetName())
		assert.Equal(t, "test-workload-identity-1", results[2].GetMetadata().GetName())
		assert.Equal(t, "Test-workload-identity-6", results[3].GetMetadata().GetName())
		assert.Equal(t, "Test-workload-identity-5", results[4].GetMetadata().GetName())
		assert.Equal(t, "Test-workload-identity-4", results[5].GetMetadata().GetName())
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
			SortField: "name",
			SortDesc:  false,
		})
		require.NoError(t, err) // asc by name is the only sort supported by the upstream
		require.Len(t, results, 1)
	})

	t.Run("unsupported sort field", func(t *testing.T) {
		_, _, err = p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortField: "spiffe_id",
		})
		require.ErrorContains(t, err, `unsupported sort, only name field is supported, but got "spiffe_id"`)
	})

	t.Run("unsupported sort dir", func(t *testing.T) {
		_, _, err = p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortDesc: true,
		})
		require.ErrorContains(t, err, "unsupported sort, only ascending order is supported")
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

// TestWorkloadIdentityCaseSensitiveName tests that workload identity name index keys remain case sensitive.
func TestWorkloadIdentityCaseSensitiveName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, func(cfg Config) Config {
		return ForAuth(cfg)
	})
	t.Cleanup(p.Close)

	{
		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "TEST-WORKLOAD-IDENTITY-1",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/test/spiffe/1",
				},
			},
		})
		require.NoError(t, err)
	}

	{
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
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListWorkloadIdentities(ctx, 0, "", &services.ListWorkloadIdentitiesRequestOptions{
			SortField: "name",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 2)

		require.Equal(t, "test-workload-identity-1", results[0].Metadata.Name)
		require.Equal(t, "TEST-WORKLOAD-IDENTITY-1", results[1].Metadata.Name)
	}, 10*time.Second, 100*time.Millisecond)
}
