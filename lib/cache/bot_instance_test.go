// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"slices"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// TestBotInstanceCache tests that CRUD operations on bot instances resources are
// replicated from the backend to the cache. Instances of scoped bots live in a
// scope-namespaced backend range, so create/update/delete events for them must
// round-trip through the cache's event watcher just like unscoped instances.
func TestBotInstanceCache(t *testing.T) {
	t.Parallel()

	for _, botScope := range []string{"", "/scopes/test"} {
		t.Run(fmt.Sprintf("scope=%q", botScope), func(t *testing.T) {
			t.Parallel()

			p := newTestPack(t, ForAuth)
			t.Cleanup(p.Close)

			testResources153(t, p, testFuncs[*machineidv1.BotInstance]{
				newResource: func(key string) (*machineidv1.BotInstance, error) {
					return machineidv1.BotInstance_builder{
						Kind:     types.KindBotInstance,
						Version:  types.V1,
						Scope:    botScope,
						Metadata: &headerv1.Metadata{},
						Spec: machineidv1.BotInstanceSpec_builder{
							BotName:    "bot-1",
							InstanceId: key,
						}.Build(),
						Status: &machineidv1.BotInstanceStatus{},
					}.Build(), nil
				},
				cacheGet: func(ctx context.Context, key string) (*machineidv1.BotInstance, error) {
					return p.cache.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: botScope, BotName: "bot-1", InstanceId: key}.Build())
				},
				cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1.BotInstance, string, error) {
					return p.cache.ListBotInstances(ctx, pageSize, pageToken, nil)
				},
				create: func(ctx context.Context, resource *machineidv1.BotInstance) error {
					_, err := p.botInstanceService.CreateBotInstance(ctx, resource)
					return err
				},
				list: func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1.BotInstance, string, error) {
					return p.botInstanceService.ListBotInstances(ctx, pageSize, pageToken, nil)
				},
				update: func(ctx context.Context, bi *machineidv1.BotInstance) error {
					_, err := p.botInstanceService.PatchBotInstance(ctx, services.PatchBotInstanceOpts{
						Bot:        scopes.QualifiedName{Scope: botScope, Name: "bot-1"},
						InstanceID: bi.GetMetadata().GetName(),
						UpdateFn: func(_ *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
							return bi, nil
						},
					})
					return err
				},
				delete: func(ctx context.Context, key string) error {
					return p.botInstanceService.DeleteBotInstance(ctx, machineidv1.DeleteBotInstanceRequest_builder{BotScope: botScope, BotName: "bot-1", InstanceId: key}.Build())
				},
				deleteAll: func(ctx context.Context) error {
					return p.botInstanceService.DeleteAllBotInstances(ctx)
				},
			})
		})
	}
}

// TestBotInstanceCollectionSeedHonorsWatchScopeFilter verifies that the cache's
// initial fetch selects the same set as its scope-filtered event stream, which an
// unfiltered seed would not, leaving permanently stale out-of-scope entries.
func TestBotInstanceCollectionSeedHonorsWatchScopeFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for i, scope := range []string{"", "/foo", "/foo/sub", "/bar"} {
		_, err := p.botInstanceService.CreateBotInstance(ctx, machineidv1.BotInstance_builder{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Scope:    scope,
			Metadata: &headerv1.Metadata{},
			Spec: machineidv1.BotInstanceSpec_builder{
				BotName:    fmt.Sprintf("bot-%d", i),
				InstanceId: uuid.New().String(),
			}.Build(),
			Status: &machineidv1.BotInstanceStatus{},
		}.Build())
		require.NoError(t, err)
	}

	tests := []struct {
		name        string
		scopeFilter *scopesv1.Filter
		wantScopes  []string
	}{
		{
			name:        "mode ALL seeds every scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			wantScopes:  []string{"", "/bar", "/foo", "/foo/sub"},
		},
		{
			name:        "mode UNSCOPED seeds only unscoped",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			wantScopes:  []string{""},
		},
		{
			name:        "mode EXACT seeds one scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/foo"}.Build(),
			wantScopes:  []string{"/foo"},
		},
		{
			name:        "mode DESCENDANTS seeds the scope and below",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
			wantScopes:  []string{"/foo", "/foo/sub"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			collection, err := newBotInstanceCollection(p.botInstanceService, types.WatchKind{
				Kind:        types.KindBotInstance,
				ScopeFilter: types.ScopeFilterFromProto(tc.scopeFilter),
			})
			require.NoError(t, err)

			seeded, err := collection.fetcher(ctx, false)
			require.NoError(t, err)

			gotScopes := make([]string, 0, len(seeded))
			for _, bi := range seeded {
				gotScopes = append(gotScopes, bi.GetScope())
			}
			slices.Sort(gotScopes)
			require.Equal(t, tc.wantScopes, gotScopes)
		})
	}

	t.Run("malformed watch filter is rejected at construction", func(t *testing.T) {
		_, err := newBotInstanceCollection(p.botInstanceService, types.WatchKind{
			Kind: types.KindBotInstance,
			// EXACT requires a scope.
			ScopeFilter: types.ScopeFilterFromProto(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build()),
		})
		require.ErrorContains(t, err, "requires a non-empty scope")
	})

	t.Run("unspecified watch filter is rejected at construction", func(t *testing.T) {
		_, err := newBotInstanceCollection(p.botInstanceService, types.WatchKind{
			Kind: types.KindBotInstance,
		})
		require.ErrorContains(t, err, "explicit scope filter mode")
	})
}

// TestBotInstanceCacheBackendTokenParity verifies that the cache lister and
// the backend lister agree on the ordering of the unified scoped+unscoped
// listing and mint interchangeable page tokens. genericLister falls back to
// the backend lister with the caller's token verbatim when the cache is
// unhealthy, so a token minted by one side must resume correctly against the
// other.
func TestBotInstanceCacheBackendTokenParity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		instances := []struct {
			botScope string
			botName  string
		}{
			{botScope: "", botName: "bot-a"},
			{botScope: "", botName: "bot-b"},
			{botScope: "/bar", botName: "bot-c"},
			{botScope: "/foo", botName: "bot-d"},
		}
		for _, i := range instances {
			_, err := p.botInstanceService.CreateBotInstance(ctx, machineidv1.BotInstance_builder{
				Kind:     types.KindBotInstance,
				Version:  types.V1,
				Scope:    i.botScope,
				Metadata: &headerv1.Metadata{},
				Spec: machineidv1.BotInstanceSpec_builder{
					BotName:    i.botName,
					InstanceId: uuid.New().String(),
				}.Build(),
				Status: &machineidv1.BotInstanceStatus{},
			}.Build())
			require.NoError(t, err)
		}

		synctest.Wait()

		names := func(instances []*machineidv1.BotInstance) []string {
			out := make([]string, 0, len(instances))
			for _, b := range instances {
				out = append(out, b.GetSpec().GetBotName())
			}
			return out
		}

		// Both listings order unscoped instances first, then scoped grouped by
		// scope.
		wantOrder := []string{"bot-a", "bot-b", "bot-c", "bot-d"}
		fromCache, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Equal(t, wantOrder, names(fromCache))
		fromBackend, _, err := p.botInstanceService.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Equal(t, wantOrder, names(fromBackend))

		// Page tokens are interchangeable whether the next item is unscoped
		// (pageSize 1) or scoped (pageSize 3).
		for _, pageSize := range []int{1, 3} {
			cachePage, cacheToken, err := p.cache.ListBotInstances(ctx, pageSize, "", nil)
			require.NoError(t, err)
			require.Equal(t, wantOrder[:pageSize], names(cachePage))
			backendPage, backendToken, err := p.botInstanceService.ListBotInstances(ctx, pageSize, "", nil)
			require.NoError(t, err)
			require.Equal(t, wantOrder[:pageSize], names(backendPage))
			require.Equal(t, backendToken, cacheToken)

			rest, _, err := p.botInstanceService.ListBotInstances(ctx, 0, cacheToken, nil)
			require.NoError(t, err)
			require.Equal(t, wantOrder[pageSize:], names(rest))
			rest, _, err = p.cache.ListBotInstances(ctx, 0, backendToken, nil)
			require.NoError(t, err)
			require.Equal(t, wantOrder[pageSize:], names(rest))
		}
	})
}

type botInstanceSpec struct {
	scope, botName, id string
	hostname, version  string
	activeAt           int64
}

func buildBotInstance(s botInstanceSpec) *machineidv1.BotInstance {
	status := &machineidv1.BotInstanceStatus{}
	if s.hostname != "" || s.version != "" || s.activeAt != 0 {
		status = machineidv1.BotInstanceStatus_builder{
			LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
				machineidv1.BotInstanceStatusHeartbeat_builder{
					Hostname:   s.hostname,
					Version:    s.version,
					RecordedAt: &timestamppb.Timestamp{Seconds: s.activeAt},
				}.Build(),
			},
		}.Build()
	}
	return machineidv1.BotInstance_builder{
		Kind:     types.KindBotInstance,
		Version:  types.V1,
		Scope:    s.scope,
		Metadata: &headerv1.Metadata{},
		Spec: machineidv1.BotInstanceSpec_builder{
			BotName:    s.botName,
			InstanceId: s.id,
		}.Build(),
		Status: status,
	}.Build()
}

// TestBotInstanceCacheList exercises Cache.ListBotInstances across filtering and
// sorting.
func TestBotInstanceCacheList(t *testing.T) {
	t.Parallel()

	botInstanceIDs := func(instances []*machineidv1.BotInstance) []string {
		out := make([]string, 0, len(instances))
		for _, b := range instances {
			out = append(out, b.GetSpec().GetInstanceId())
		}
		return out
	}

	sortInstances := []botInstanceSpec{
		{botName: "alpha", id: "i-a", activeAt: 200, version: "2.0.0", hostname: "host-2"},
		{botName: "beta", id: "i-b", activeAt: 100, version: "2.0.0-rc1", hostname: "host-3"},
		{scope: "/east", botName: "alpha", id: "i-c", activeAt: 300, version: "1.0.0", hostname: "host-1"},
	}

	evenFilter := func(b *machineidv1.BotInstance) bool {
		id := b.GetSpec().GetInstanceId()
		return (id[len(id)-1]-'0')%2 == 0
	}

	// Instance IDs name their scope, so expectations read as selected scopes.
	scopeFilterInstances := []botInstanceSpec{
		{botName: "u", id: "u"},
		{scope: "/foo", botName: "f", id: "foo"},
		{scope: "/foo/sub", botName: "fs", id: "foo-sub"},
		{scope: "/bar", botName: "b", id: "bar"},
	}

	filterFnInstances := []botInstanceSpec{
		{botName: "bot", id: "i-0"},
		{botName: "bot", id: "i-1"},
		{botName: "bot", id: "i-2"},
		{botName: "bot", id: "i-3"},
		{scope: "/s", botName: "bot", id: "i-4"},
		{scope: "/s", botName: "bot", id: "i-5"},
	}

	tests := []struct {
		name      string
		instances []botInstanceSpec
		opts      *services.ListBotInstancesRequestOptions
		want      []string
		wantErr   string
	}{
		{
			name: "no filter spans all scopes, unscoped first",
			instances: []botInstanceSpec{
				{botName: "bot-a", id: "u1"},
				{scope: "/foo", botName: "bot-b", id: "s1"},
				{botName: "bot-c", id: "u2"},
			},
			want: []string{"u1", "u2", "s1"},
		},
		{
			name: "bot name filter matches only the unscoped bot of that name",
			instances: []botInstanceSpec{
				{botName: "web", id: "web-a"},
				{botName: "web", id: "web-b"},
				{scope: "/prod", botName: "web", id: "web-c"},
				{botName: "db", id: "db-a"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterBotName: "web"},
			want: []string{"web-a", "web-b"},
		},
		{
			name: "bot name and scope filter matches the scoped bot",
			instances: []botInstanceSpec{
				{botName: "web", id: "web-u"},
				{scope: "/prod", botName: "web", id: "web-p1"},
				{scope: "/prod", botName: "web", id: "web-p2"},
				{scope: "/prod", botName: "api", id: "api-p"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterBotName: "web", FilterBotScope: "/prod"},
			want: []string{"web-p1", "web-p2"},
		},
		{
			// A bot scope only qualifies a bot name, so it is rejected on its own
			// rather than listing every instance in the scope. Standalone scope
			// selection is ScopeFilter, covered below.
			name: "bot scope without a bot name filter is rejected",
			instances: []botInstanceSpec{
				{botName: "web", id: "web-u"},
				{scope: "/prod", botName: "web", id: "web-p1"},
			},
			opts:    &services.ListBotInstancesRequestOptions{FilterBotScope: "/prod"},
			wantErr: "bot scope filter requires a bot name filter",
		},
		{
			name:      "scope filter mode ALL spans all scopes",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			},
			want: []string{"u", "bar", "foo", "foo-sub"},
		},
		{
			// Unscoped is orthogonal to every scoped value, not the root.
			name:      "scope filter mode UNSCOPED matches only unscoped instances",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			},
			want: []string{"u"},
		},
		{
			name:      "scope filter mode EXACT matches one scope",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/foo"}.Build(),
			},
			want: []string{"foo"},
		},
		{
			name:      "scope filter mode DESCENDANTS includes the scope and below",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
			},
			want: []string{"foo", "foo-sub"},
		},
		{
			name:      "scope filter with a bot filter is rejected",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				FilterBotName:  "f",
				FilterBotScope: "/foo",
				ScopeFilter:    scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			},
			wantErr: "scope filter cannot be combined with a bot name filter",
		},
		{
			// Rejected even though it would be harmless as a predicate.
			name:      "scope filter mode ALL with a bot filter is rejected",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				FilterBotName:  "f",
				FilterBotScope: "/foo",
				ScopeFilter:    scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			},
			wantErr: "scope filter cannot be combined with a bot name filter",
		},
		{
			name:      "malformed scope filter is rejected",
			instances: scopeFilterInstances,
			// EXACT requires a scope.
			opts: &services.ListBotInstancesRequestOptions{
				ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build(),
			},
			wantErr: "requires a non-empty scope",
		},
		{
			name: "search term matches across scopes",
			instances: []botInstanceSpec{
				{botName: "runner", id: "r1", hostname: "host-match"},
				{scope: "/foo", botName: "runner", id: "r2", hostname: "host-match"},
				{botName: "runner", id: "r3", hostname: "host-other"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "host-match"},
			want: []string{"r1", "r2"},
		},
		{
			name: "predicate query matches across scopes",
			instances: []botInstanceSpec{
				{botName: "q", id: "q1", hostname: "host-1"},
				{scope: "/foo", botName: "q", id: "q2", hostname: "host-1"},
				{botName: "q", id: "q3", hostname: "host-2"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterQuery: `status.latest_heartbeat.hostname == "host-1"`},
			want: []string{"q1", "q2"},
		},
		{
			name:      "sort by bot_name ascending groups unscoped before scoped",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "bot_name"},
			want:      []string{"i-a", "i-b", "i-c"},
		},
		{
			name:      "sort by bot_name descending",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "bot_name", SortDesc: true},
			want:      []string{"i-c", "i-b", "i-a"},
		},
		{
			name:      "sort by active_at ascending is global across scopes",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "active_at_latest"},
			want:      []string{"i-b", "i-a", "i-c"},
		},
		{
			name:      "sort by active_at descending",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "active_at_latest", SortDesc: true},
			want:      []string{"i-c", "i-a", "i-b"},
		},
		{
			name:      "sort by version ascending is global across scopes",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "version_latest"},
			want:      []string{"i-c", "i-b", "i-a"},
		},
		{
			name:      "sort by version descending",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "version_latest", SortDesc: true},
			want:      []string{"i-a", "i-b", "i-c"},
		},
		{
			name:      "sort by host_name ascending is global across scopes",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "host_name_latest"},
			want:      []string{"i-c", "i-a", "i-b"},
		},
		{
			name:      "sort by host_name descending",
			instances: sortInstances,
			opts:      &services.ListBotInstancesRequestOptions{SortField: "host_name_latest", SortDesc: true},
			want:      []string{"i-b", "i-a", "i-c"},
		},
		{
			name:      "unsupported sort field is rejected",
			instances: []botInstanceSpec{{botName: "bot", id: "x1"}},
			opts:      &services.ListBotInstancesRequestOptions{SortField: "nope"},
			wantErr:   `unsupported sort "nope"`,
		},
		{
			name:      "filter fn spans scopes",
			instances: filterFnInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterFn: evenFilter},
			want:      []string{"i-0", "i-2", "i-4"},
		},
		{
			name:      "filter fn combined with bot name filter",
			instances: filterFnInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterBotName: "bot", FilterFn: evenFilter},
			want:      []string{"i-0", "i-2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				p := newTestPack(t, ForAuth)
				t.Cleanup(p.Close)

				for _, s := range tc.instances {
					_, err := p.botInstanceService.CreateBotInstance(ctx, buildBotInstance(s))
					require.NoError(t, err)
				}

				synctest.Wait()

				got, _, err := p.cache.ListBotInstances(ctx, 0, "", tc.opts)
				if tc.wantErr != "" {
					assert.ErrorContains(t, err, tc.wantErr)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.want, botInstanceIDs(got))

				// Also assert that pagination produces the same results (in
				// particular, we're looking for cursor keys not lining up
				// leading to skipped entries)
				paged, err := stream.Collect(
					clientutils.ResourcesWithPageSize(
						ctx,
						func(ctx context.Context, pageSize int, token string) ([]*machineidv1.BotInstance, string, error) {
							return p.cache.ListBotInstances(ctx, pageSize, token, tc.opts)
						},
						2,
					),
				)
				require.NoError(t, err)
				assert.Equal(t, tc.want, botInstanceIDs(paged))
			})
		})
	}
}

// TestBotInstanceCacheFallback tests that requests fallback to the upstream when the cache is unhealthy.
// TestBotInstanceCacheFallback compares reads served by the unhealthy-cache
// fallback against the healthy cache: both apply the collection's scope filter
// to lists and gets, while sort support legitimately differs (the upstream
// backend only supports bot_name-ascending iteration).
func TestBotInstanceCacheFallback(t *testing.T) {
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
						if w.Kind == types.KindBotInstance {
							cfg.Watches[i].ScopeFilter = types.ScopeFilterFromProto(
								scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
							)
						}
					}
					return cfg
				})
				t.Cleanup(p.Close)

				for i, scope := range []string{"", "/foo", "/foo/sub", "/bar"} {
					_, err := p.botInstanceService.CreateBotInstance(ctx, machineidv1.BotInstance_builder{
						Kind:     types.KindBotInstance,
						Version:  types.V1,
						Scope:    scope,
						Metadata: &headerv1.Metadata{},
						Spec: machineidv1.BotInstanceSpec_builder{
							BotName:    fmt.Sprintf("bot-%d", i),
							InstanceId: fmt.Sprintf("instance-%d", i),
						}.Build(),
						Status: &machineidv1.BotInstanceStatus{},
					}.Build())
					require.NoError(t, err)
				}

				synctest.Wait()

				// Lists apply the collection's scope filter regardless of health.
				results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
				require.NoError(t, err)
				gotScopes := make([]string, 0, len(results))
				for _, bi := range results {
					gotScopes = append(gotScopes, bi.GetScope())
				}
				slices.Sort(gotScopes)
				require.Equal(t, []string{"/foo", "/foo/sub"}, gotScopes)

				// So do gets: in-filter is retrievable, out-of-filter reads as absent.
				gotFoo, err := p.cache.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{
					BotName: "bot-1", InstanceId: "instance-1", BotScope: "/foo",
				}.Build())
				require.NoError(t, err)
				require.Equal(t, "/foo", gotFoo.GetScope())

				_, err = p.cache.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{
					BotName: "bot-3", InstanceId: "instance-3", BotScope: "/bar",
				}.Build())
				require.True(t, trace.IsNotFound(err), "expected NotFound for out-of-filter instance, got %v", err)

				// The healthy cache serves every sort from its indexes; the fallback
				// surfaces the backend's ordering limits.
				listErr := func(sortField string, desc bool) error {
					_, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
						SortField: sortField,
						SortDesc:  desc,
					})
					return err
				}
				require.NoError(t, listErr("bot_name", false))
				if tt.neverOK {
					require.ErrorContains(t, listErr("bot_name", true), "unsupported sort, only ascending order is supported")
					require.ErrorContains(t, listErr("active_at_latest", false), `unsupported sort, only bot_name field is supported, but got "active_at_latest"`)
				} else {
					require.NoError(t, listErr("bot_name", true))
					require.NoError(t, listErr("active_at_latest", false))
				}
			})
		})
	}
}

func TestKeyForVersionIndex(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name      string
		mutatorFn func(*machineidv1.BotInstance)
		key       string
	}{
		{
			name:      "zero heartbeats",
			mutatorFn: func(b *machineidv1.BotInstance) {},
			key:       "000000.000000.000000-~/bot-instance-1",
		},
		{
			name: "invalid version",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					InitialHeartbeat: machineidv1.BotInstanceStatusHeartbeat_builder{
						Version: "a.b.c",
					}.Build(),
				}.Build())
			},
			key: "000000.000000.000000-~/bot-instance-1",
		},
		{
			name: "initial heartbeat",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					InitialHeartbeat: machineidv1.BotInstanceStatusHeartbeat_builder{
						Version: "1.0.0",
					}.Build(),
				}.Build())
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "latest heartbeat",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						machineidv1.BotInstanceStatusHeartbeat_builder{
							Version: "1.0.0",
						}.Build(),
					},
				}.Build())
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "with release",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					InitialHeartbeat: machineidv1.BotInstanceStatusHeartbeat_builder{
						Version: "1.0.0-dev",
					}.Build(),
				}.Build())
			},
			key: "000001.000000.000000-dev/bot-instance-1",
		},
		{
			name: "with build",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					InitialHeartbeat: machineidv1.BotInstanceStatusHeartbeat_builder{
						Version: "1.0.0+build1",
					}.Build(),
				}.Build())
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "with release and build",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.SetStatus(machineidv1.BotInstanceStatus_builder{
					InitialHeartbeat: machineidv1.BotInstanceStatusHeartbeat_builder{
						Version: "1.0.0-dev+build1",
					}.Build(),
				}.Build())
			},
			key: "000001.000000.000000-dev/bot-instance-1",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			instance := machineidv1.BotInstance_builder{
				Kind:    types.KindBotInstance,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "bot-instance-1",
				}.Build(),
				Spec:   &machineidv1.BotInstanceSpec{},
				Status: &machineidv1.BotInstanceStatus{},
			}.Build()
			tc.mutatorFn(instance)

			versionKey := keyForBotInstanceVersionIndex(instance)

			assert.Equal(t, tc.key, versionKey)
		})
	}
}
