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
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
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
		cacheGet: func(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error) {
			return p.cache.GetWorkloadIdentity(ctx, workloadidentityv1pb.GetWorkloadIdentityRequest_builder{Name: name}.Build())
		},
	})
}

// TestWorkloadIdentityCollectionSeedHonorsWatchScopeFilter verifies the seed
// selects the same set as the event stream. The stream is filtered per-event by
// services.WatchKindMatchesScope, so an unfiltered seed would leave permanently
// stale out-of-scope entries in the store.
func TestWorkloadIdentityCollectionSeedHonorsWatchScopeFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p, err := newPack(t, ForAuth)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	// Fixture names say which scope they live in.
	_, err = p.workloadIdentity.CreateWorkloadIdentity(ctx, newWorkloadIdentity("unscoped"))
	require.NoError(t, err)
	for name, scope := range map[string]string{
		"foo":     "/foo",
		"foo-sub": "/foo/sub",
		"bar":     "/bar",
	} {
		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, workloadidentityv1pb.WorkloadIdentity_builder{
			Kind:     types.KindWorkloadIdentity,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: name}.Build(),
			Scope:    scope,
			Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
				Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{Id: scope + "/_/" + name}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
	}

	for _, tc := range []struct {
		name        string
		scopeFilter *scopesv1.Filter
		want        []string
	}{
		{
			name:        "mode ALL matches every scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			want:        []string{"bar", "foo", "foo-sub", "unscoped"},
		},
		{
			name:        "mode UNSCOPED matches only unscoped",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			want:        []string{"unscoped"},
		},
		{
			name:        "mode EXACT matches one scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/foo"}.Build(),
			want:        []string{"foo"},
		},
		{
			name:        "mode DESCENDANTS matches the scope and below",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
			want:        []string{"foo", "foo-sub"},
		},
		{
			name:        "mode ANCESTORS matches the scope and above",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: "/foo/sub"}.Build(),
			want:        []string{"foo", "foo-sub"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collection, err := newWorkloadIdentityCollection(p.workloadIdentity, types.WatchKind{
				Kind:        types.KindWorkloadIdentity,
				ScopeFilter: types.ScopeFilterFromProto(tc.scopeFilter),
			})
			require.NoError(t, err)

			seeded, err := collection.fetcher(t.Context(), false)
			require.NoError(t, err)

			var names []string
			for _, wi := range seeded {
				names = append(names, wi.GetMetadata().GetName())
			}
			slices.Sort(names)
			require.Equal(t, tc.want, names)
		})
	}

	t.Run("malformed watch filter is rejected at construction", func(t *testing.T) {
		_, err := newWorkloadIdentityCollection(p.workloadIdentity, types.WatchKind{
			Kind: types.KindWorkloadIdentity,
			ScopeFilter: types.ScopeFilterFromProto(
				scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build(),
			),
		})
		require.ErrorContains(t, err, "requires a non-empty scope")
	})

	t.Run("unspecified watch filter is rejected at construction", func(t *testing.T) {
		_, err := newWorkloadIdentityCollection(p.workloadIdentity, types.WatchKind{
			Kind: types.KindWorkloadIdentity,
		})
		require.ErrorContains(t, err, "explicit scope filter mode")
	})
}

// TestWorkloadIdentityCacheScoped verifies the cache serves scoped reads, keeps
// scoped and unscoped identities of the same name distinct, and evicts a scoped
// identity when it is deleted, exercising the scope-aware watch path.
func TestWorkloadIdentityCacheScoped(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		const scope = "/staging"
		scopedName := scopes.QualifiedName{Scope: scope, Name: "shared"}
		unscopedName := scopes.QualifiedName{Name: "shared"}
		getReq := func(n scopes.QualifiedName) *workloadidentityv1pb.GetWorkloadIdentityRequest {
			return workloadidentityv1pb.GetWorkloadIdentityRequest_builder{Scope: n.Scope, Name: n.Name}.Build()
		}
		delReq := func(n scopes.QualifiedName) *workloadidentityv1pb.DeleteWorkloadIdentityRequest {
			return workloadidentityv1pb.DeleteWorkloadIdentityRequest_builder{Scope: n.Scope, Name: n.Name}.Build()
		}

		// An unscoped and a scoped identity that share a name must not collide.
		_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, newWorkloadIdentity("shared"))
		require.NoError(t, err)
		scoped := workloadidentityv1pb.WorkloadIdentity_builder{
			Kind:     types.KindWorkloadIdentity,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: "shared"}.Build(),
			Scope:    scope,
			Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
				Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
					Id: scope + "/_/svc",
				}.Build(),
			}.Build(),
		}.Build()
		_, err = p.workloadIdentity.CreateWorkloadIdentity(ctx, scoped)
		require.NoError(t, err)

		synctest.Wait()

		// Both are independently retrievable from the cache by their qualified name.
		gotUnscoped, err := p.cache.GetWorkloadIdentity(ctx, getReq(unscopedName))
		require.NoError(t, err)
		require.Empty(t, gotUnscoped.GetScope())
		require.Equal(t, "/example", gotUnscoped.GetSpec().GetSpiffe().GetId())

		gotScoped, err := p.cache.GetWorkloadIdentity(ctx, getReq(scopedName))
		require.NoError(t, err)
		require.Equal(t, scope, gotScoped.GetScope())

		// Deleting the scoped identity evicts it from the cache without disturbing
		// the unscoped identity of the same name.
		require.NoError(t, p.workloadIdentity.DeleteWorkloadIdentity(ctx, delReq(scopedName)))
		synctest.Wait()

		_, err = p.cache.GetWorkloadIdentity(ctx, getReq(scopedName))
		require.True(t, trace.IsNotFound(err))

		gotUnscoped, err = p.cache.GetWorkloadIdentity(ctx, getReq(unscopedName))
		require.NoError(t, err)
		require.Empty(t, gotUnscoped.GetScope())
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
		return collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(t.Context(), start, end, sortField, sortDesc))
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
				p.cache.RangeWorkloadIdentities(t.Context(), token, "", sortField, sortDesc),
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

// TestWorkloadIdentityCacheFallback compares reads served by the unhealthy-cache
// fallback against the healthy cache: both apply the collection's scope filter
// to ranges and gets, while sort support legitimately differs (the upstream
// backend only supports name-ascending iteration).
func TestWorkloadIdentityCacheFallback(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		neverOK bool
	}{
		{name: "HealthyCache", neverOK: false},
		{name: "Fallback", neverOK: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				p := newTestPack(t, func(cfg Config) Config {
					cfg = ForAuth(cfg)
					cfg.neverOK = tt.neverOK
					for i, w := range cfg.Watches {
						if w.Kind == types.KindWorkloadIdentity {
							cfg.Watches[i].ScopeFilter = types.ScopeFilterFromProto(
								scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
							)
						}
					}
					return cfg
				})
				t.Cleanup(p.Close)

				for name, scope := range map[string]string{
					"unscoped": "",
					"foo":      "/foo",
					"foo-sub":  "/foo/sub",
					"bar":      "/bar",
				} {
					_, err := p.workloadIdentity.CreateWorkloadIdentity(ctx, workloadidentityv1pb.WorkloadIdentity_builder{
						Kind:     types.KindWorkloadIdentity,
						Version:  types.V1,
						Metadata: headerv1.Metadata_builder{Name: name}.Build(),
						Scope:    scope,
						Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
							Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{Id: scope + "/_/" + name}.Build(),
						}.Build(),
					}.Build())
					require.NoError(t, err)
				}

				synctest.Wait()

				// Ranges apply the collection's scope filter regardless of health.
				got := collectWorkloadIdentities(t, p.cache.RangeWorkloadIdentities(ctx, "", "", "", false))
				var names []string
				for _, wi := range got {
					names = append(names, wi.GetMetadata().GetName())
				}
				slices.Sort(names)
				require.Equal(t, []string{"foo", "foo-sub"}, names)

				// So do gets: in-filter is retrievable, out-of-filter reads as absent.
				gotFoo, err := p.cache.GetWorkloadIdentity(ctx, workloadidentityv1pb.GetWorkloadIdentityRequest_builder{
					Scope: "/foo", Name: "foo",
				}.Build())
				require.NoError(t, err)
				require.Equal(t, "/foo", gotFoo.GetScope())

				_, err = p.cache.GetWorkloadIdentity(ctx, workloadidentityv1pb.GetWorkloadIdentityRequest_builder{
					Scope: "/bar", Name: "bar",
				}.Build())
				require.True(t, trace.IsNotFound(err), "expected NotFound for out-of-filter identity, got %v", err)

				// The healthy cache serves spiffe_id and descending sorts from its
				// indexes; the fallback surfaces the backend's ordering limits.
				rangeErr := func(sortField services.WorkloadIdentitySortField, desc bool) error {
					var err error
					for _, iterErr := range p.cache.RangeWorkloadIdentities(ctx, "", "", sortField, desc) {
						err = iterErr
					}
					return err
				}
				if tt.neverOK {
					require.ErrorContains(t, rangeErr("spiffe_id", false), `unsupported sort, only name field is supported, but got "spiffe_id"`)
					require.ErrorContains(t, rangeErr("name", true), "unsupported sort, only ascending order is supported")
				} else {
					require.NoError(t, rangeErr("spiffe_id", false))
					require.NoError(t, rangeErr("name", true))
				}
			})
		})
	}
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
