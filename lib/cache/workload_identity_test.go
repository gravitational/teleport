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
	"fmt"
	"iter"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// collectWorkloadIdentities drains a WorkloadIdentity range into a slice,
// failing the test on error.
func collectWorkloadIdentities(t require.TestingT, it iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error]) []*workloadidentityv1pb.WorkloadIdentity {
	out, err := stream.Collect(it)
	require.NoError(t, err)
	return out
}

// workloadIdentityPageFunc adapts RangeWorkloadIdentities to the paginated list
// signature expected by the generic cache test harness.
func workloadIdentityPageFunc(
	rangeFn func(ctx context.Context, start, end string, sortField services.WorkloadIdentitySortField, sortDesc bool) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error],
) func(ctx context.Context, pageSize int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
	return func(ctx context.Context, pageSize int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
		return generic.CollectPageAndCursor(
			rangeFn(ctx, pageToken, "", "", false),
			pageSize,
			func(wi *workloadidentityv1pb.WorkloadIdentity) string { return wi.GetMetadata().GetName() },
		)
	}
}

func newWorkloadIdentity(name string) *workloadidentityv1pb.WorkloadIdentity {
	return workloadidentityv1pb.WorkloadIdentity_builder{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
			Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
				Id: "/example",
			}.Build(),
		}.Build(),
	}.Build()
}

// createWorkloadIdentities creates the given identities (keyed by name, valued
// by SPIFFE ID) in the backend and blocks until the cache has observed all of
// them. Names must be unique.
func createWorkloadIdentities(t *testing.T, ctx context.Context, p *testPack, ids map[string]string) {
	t.Helper()
	for name, spiffeID := range ids {
		wid := workloadidentityv1pb.WorkloadIdentity_builder{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: name,
			}.Build(),
			Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
				Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
					Id: spiffeID,
				}.Build(),
			}.Build(),
		}.Build()
		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, wid)
		require.NoError(t, err, "failed to create WorkloadIdentity %q", name)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results := collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(ctx, "", "", "", false))
		require.Len(t, results, len(ids))
	}, 10*time.Second, 100*time.Millisecond)
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
		list: workloadIdentityPageFunc(p.workloadIdentity.RangeWorkloadIdentities),
		deleteAll: func(ctx context.Context) error {
			return p.workloadIdentity.DeleteAllWorkloadIdentities(ctx)
		},
		cacheList: workloadIdentityPageFunc(p.cache.RangeWorkloadIdentities),
		cacheGet:  p.cache.GetWorkloadIdentity,
	})
}

// TestWorkloadIdentityCacheRange tests that RangeWorkloadIdentities iterates in
// the requested order and honors range bounds.
func TestWorkloadIdentityCacheRange(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	createWorkloadIdentities(t, ctx, p, map[string]string{
		"test-workload-identity-1": "/test/spiffe/2",
		"test-workload-identity-3": "/test/spiffe/1",
		"test-workload-identity-2": "/test/spiffe/3",
		"Test-workload-identity-4": "/Test/spiffe/2",
		"Test-workload-identity-5": "/Test/spiffe/1",
		"Test-workload-identity-6": "/Test/spiffe/3",
	})

	collectRange := func(t *testing.T, start, end string, sortField services.WorkloadIdentitySortField, sortDesc bool) []*workloadidentityv1pb.WorkloadIdentity {
		return collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(ctx, start, end, sortField, sortDesc))
	}

	names := func(in []*workloadidentityv1pb.WorkloadIdentity) []string {
		out := make([]string, len(in))
		for i, wi := range in {
			out[i] = wi.GetMetadata().GetName()
		}
		return out
	}
	spiffeIDs := func(in []*workloadidentityv1pb.WorkloadIdentity) []string {
		out := make([]string, len(in))
		for i, wi := range in {
			out[i] = wi.GetSpec().GetSpiffe().GetId()
		}
		return out
	}

	t.Run("full range ascending by name", func(t *testing.T) {
		got := collectRange(t, "", "", "name", false)
		require.Equal(t, []string{
			"Test-workload-identity-4",
			"Test-workload-identity-5",
			"Test-workload-identity-6",
			"test-workload-identity-1",
			"test-workload-identity-2",
			"test-workload-identity-3",
		}, names(got))
	})

	t.Run("full range descending by name", func(t *testing.T) {
		got := collectRange(t, "", "", "name", true)
		require.Equal(t, []string{
			"test-workload-identity-3",
			"test-workload-identity-2",
			"test-workload-identity-1",
			"Test-workload-identity-6",
			"Test-workload-identity-5",
			"Test-workload-identity-4",
		}, names(got))
	})

	t.Run("empty sort field defaults to ascending by name", func(t *testing.T) {
		got := collectRange(t, "", "", "", false)
		require.Equal(t, []string{
			"Test-workload-identity-4",
			"Test-workload-identity-5",
			"Test-workload-identity-6",
			"test-workload-identity-1",
			"test-workload-identity-2",
			"test-workload-identity-3",
		}, names(got))
	})

	t.Run("full range ascending by spiffe_id", func(t *testing.T) {
		got := collectRange(t, "", "", "spiffe_id", false)
		require.Equal(t, []string{
			"/Test/spiffe/1",
			"/test/spiffe/1",
			"/Test/spiffe/2",
			"/test/spiffe/2",
			"/Test/spiffe/3",
			"/test/spiffe/3",
		}, spiffeIDs(got))
	})

	t.Run("full range descending by spiffe_id", func(t *testing.T) {
		got := collectRange(t, "", "", "spiffe_id", true)
		require.Equal(t, []string{
			"/test/spiffe/3",
			"/Test/spiffe/3",
			"/test/spiffe/2",
			"/Test/spiffe/2",
			"/test/spiffe/1",
			"/Test/spiffe/1",
		}, spiffeIDs(got))
	})

	t.Run("bounded name range is exclusive of end", func(t *testing.T) {
		got := collectRange(t, "Test-workload-identity-5", "test-workload-identity-2", "name", false)
		require.Equal(t, []string{
			"Test-workload-identity-5",
			"Test-workload-identity-6",
			"test-workload-identity-1",
		}, names(got))
	})

	t.Run("unsupported sort field yields an error", func(t *testing.T) {
		var err error
		for _, iterErr := range p.cache.RangeWorkloadIdentities(ctx, "", "", "blah", false) {
			err = iterErr
		}
		require.ErrorContains(t, err, `unsupported sort "blah" but expected name or spiffe_id`)
	})
}

// TestWorkloadIdentityCacheRangePagination exercises multi-page pagination
// round-trips across every sort field and direction, mirroring how the gRPC
// handler threads the page cursor when ranging over the cache
// (CollectPageAndCursor over RangeWorkloadIdentities).
func TestWorkloadIdentityCacheRangePagination(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	// Create identities whose spiffe_id ordering deliberately differs from their
	// name ordering (a permutation), so spiffe_id sorting is genuinely tested.
	const n = 12
	ids := make(map[string]string, n)
	for i := range n {
		ids[fmt.Sprintf("wi-%02d", i)] = fmt.Sprintf("/id-%02d", (i*7)%n)
	}
	createWorkloadIdentities(t, ctx, p, ids)

	paginate := func(t *testing.T, sortField services.WorkloadIdentitySortField, sortDesc bool) []*workloadidentityv1pb.WorkloadIdentity {
		var fetched []*workloadidentityv1pb.WorkloadIdentity
		token := ""
		keyFn, err := services.WorkloadIdentityKey(sortField)
		require.NoError(t, err)
		for {
			page, next, err := generic.CollectPageAndCursor(
				p.cache.RangeWorkloadIdentities(ctx, token, "", sortField, sortDesc),
				5,
				keyFn,
			)
			require.NoError(t, err)
			fetched = append(fetched, page...)
			if next == "" {
				break
			}
			token = next
		}
		return fetched
	}

	for _, tc := range []struct {
		name      string
		sortField services.WorkloadIdentitySortField
		sortDesc  bool
	}{
		{"name asc", services.WorkloadIdentitySortFieldName, false},
		{"name desc", services.WorkloadIdentitySortFieldName, true},
		{"spiffe_id asc", services.WorkloadIdentitySortFieldSPIFFEID, false},
		{"spiffe_id desc", services.WorkloadIdentitySortFieldSPIFFEID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := paginate(t, tc.sortField, tc.sortDesc)

			// Every identity is returned exactly once across page boundaries.
			seen := map[string]int{}
			for _, wi := range got {
				seen[wi.GetMetadata().GetName()]++
			}
			require.Len(t, seen, n)
			for name, count := range seen {
				require.Equalf(t, 1, count, "identity %q returned %d times", name, count)
			}

			// The global ordering across pages matches the canonical sort key.
			keyFn, err := services.WorkloadIdentityKey(tc.sortField)
			require.NoError(t, err)
			require.True(t, slices.IsSortedFunc(got, func(a, b *workloadidentityv1pb.WorkloadIdentity) int {
				if tc.sortDesc {
					return strings.Compare(keyFn(b), keyFn(a))
				}
				return strings.Compare(keyFn(a), keyFn(b))
			}))
		})
	}
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

	createWorkloadIdentities(t, ctx, p, map[string]string{
		"test-workload-identity-1": "/test/spiffe/1",
	})

	// The upstream backend only supports name-ascending iteration, so a range
	// served from the unhealthy cache is constrained to that ordering.
	t.Run("supported sort", func(t *testing.T) {
		got := collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(ctx, "", "", "name", false))
		require.Len(t, got, 1)
	})

	t.Run("unsupported sort field", func(t *testing.T) {
		var err error
		for _, iterErr := range p.cache.RangeWorkloadIdentities(ctx, "", "", "spiffe_id", false) {
			err = iterErr
		}
		require.ErrorContains(t, err, `unsupported sort, only name field is supported, but got "spiffe_id"`)
	})

	t.Run("unsupported sort dir", func(t *testing.T) {
		var err error
		for _, iterErr := range p.cache.RangeWorkloadIdentities(ctx, "", "", "name", true) {
			err = iterErr
		}
		require.ErrorContains(t, err, "unsupported sort, only ascending order is supported")
	})
}

// TestWorkloadIdentityCaseSensitiveName tests that workload identity name index keys remain case sensitive.
func TestWorkloadIdentityCaseSensitiveName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	createWorkloadIdentities(t, ctx, p, map[string]string{
		"TEST-WORKLOAD-IDENTITY-1": "/test/spiffe/1",
		"test-workload-identity-1": "/test/spiffe/1",
	})

	// Name index keys are case sensitive: in descending order the lowercase
	// name sorts before the uppercase one.
	results := collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(ctx, "", "", "name", true))
	require.Len(t, results, 2)
	require.Equal(t, "test-workload-identity-1", results[0].Metadata.Name)
	require.Equal(t, "TEST-WORKLOAD-IDENTITY-1", results[1].Metadata.Name)
}
