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

package cache

import (
	"context"
	"fmt"
	"iter"
	"testing"
	"testing/synctest"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type hostIDGetterResource interface {
	types.Resource
	GetHostID() string
}

type rangeServersWithTargetNameFuncs[T hostIDGetterResource] struct {
	// newResource creates a new resource with the specified hostID and targetName.
	newResource func(t testing.TB, hostID, targetName string) T
	// create adds the resource to the backend.
	create func(context.Context, services.Presence, T) error
	// delete removes the resource from the backend.
	delete func(context.Context, services.Presence, T) error
	// rangeByName returns a stream of resources from the cache matching the target name filter.
	rangeByName func(*Cache, context.Context, string) iter.Seq2[T, error]
}

type rangeServerSeed struct {
	// hostID is the host ID to use for the seeded resource. If empty, a default will be generated.
	hostID string
	// targetName is the target name to use for the seeded resource (e.g. Database.ServiceName or App.Name).
	targetName string
	// want indicates whether this resource should be included in the test's expected results.
	want bool
}

type rangeServersWithTargetNameTestCase[T hostIDGetterResource] struct {
	// name is the test case name.
	name string
	// neverOK, if true, forces the cache to always be unhealthy, causing the
	// rangeByName function to fallback to the backend for all pages.
	neverOK bool
	// filter is the target name filter to use when ranging resources.
	filter string
	// seeds defines the resources to seed in the backend before running the test.
	seeds []rangeServerSeed
	// onCollected is an optional callback that is called after each resource is collected
	// from the rangeByName function, allowing the test to mutate cache state mid-iteration
	onCollected func(n int, c *Cache)
}

func testRangeServersWithTargetName[T hostIDGetterResource](t *testing.T, funcs rangeServersWithTargetNameFuncs[T]) {
	rangeServerSeeds := func(count int, targetName string, want bool, hostID func(int) string) []rangeServerSeed {
		seeds := make([]rangeServerSeed, count)
		for i := range count {
			seeds[i].targetName = targetName
			seeds[i].want = want
			if hostID != nil {
				seeds[i].hostID = hostID(i)
			} else {
				seeds[i].hostID = fmt.Sprintf("%06d", i)
			}
		}
		return seeds
	}

	tests := []rangeServersWithTargetNameTestCase[T]{
		{
			name:   "HealthyCache_MultipleServersForSameTarget",
			filter: "shared-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target", want: true},
				{targetName: "shared-target", want: true},
				{targetName: "other-target"},
			},
		},
		{
			name:   "HealthyCache_SingleServerForTarget",
			filter: "other-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target"},
				{targetName: "other-target", want: true},
			},
		},
		{
			name:   "HealthyCache_NonExistentTarget",
			filter: "non-existent-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target"},
			},
		},
		{
			name:   "HealthyCache_FirstPagePartialMatchesContinues",
			filter: "target",
			seeds: append(
				[]rangeServerSeed{
					{hostID: "000000", targetName: "target", want: true},
					{hostID: "000001", targetName: "target", want: true},
				},
				append(
					rangeServerSeeds(apidefaults.DefaultChunkSize, "other-target", false, func(i int) string {
						return fmt.Sprintf("%06d", i+2)
					}),
					rangeServerSeed{hostID: "zzzzzz", targetName: "target", want: true},
				)...,
			),
		},
		{
			name:    "Fallback_MultipleServersForSameTarget",
			neverOK: true,
			filter:  "shared-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target", want: true},
				{targetName: "shared-target", want: true},
				{targetName: "other-target"},
			},
		},
		{
			name:    "Fallback_SingleServerForTarget",
			neverOK: true,
			filter:  "other-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target"},
				{targetName: "other-target", want: true},
			},
		},
		{
			name:    "Fallback_NonExistentTarget",
			neverOK: true,
			filter:  "non-existent-target",
			seeds: []rangeServerSeed{
				{targetName: "shared-target"},
			},
		},
		{
			name:    "Fallback_PageWithNoMatchesContinuesToNextPage",
			neverOK: true,
			filter:  "zzz-target",
			seeds: append(
				// Fill a full page with "aaa-target" servers whose hostIDs sort
				// before "zzzzzz", guaranteeing the first backend page has zero
				// matches for "zzz-target".
				rangeServerSeeds(apidefaults.DefaultChunkSize+1, "aaa-target", false, func(i int) string {
					return fmt.Sprintf("%06d", i)
				}),
				rangeServerSeed{hostID: "zzzzzz", targetName: "zzz-target", want: true},
			),
		},
		{
			name:    "Fallback_FirstPagePartialMatchesContinues",
			neverOK: true,
			filter:  "target",
			seeds: append(
				[]rangeServerSeed{
					{hostID: "000000", targetName: "target", want: true},
					{hostID: "000001", targetName: "target", want: true},
				},
				append(
					rangeServerSeeds(apidefaults.DefaultChunkSize, "other-target", false, func(i int) string {
						return fmt.Sprintf("%06d", i+2)
					}),
					rangeServerSeed{hostID: "zzzzzz", targetName: "target", want: true},
				)...,
			),
		},
		{
			name:   "TransientCacheHealth_ContinuesOnFallbackMidIteration",
			filter: "shared-target",
			seeds:  rangeServerSeeds(apidefaults.DefaultChunkSize+1, "shared-target", true, nil),
			onCollected: func(n int, c *Cache) {
				if n == apidefaults.DefaultChunkSize {
					c.ok = false
				}
			},
		},
		{
			name:   "TransientCacheHealth_CacheRecoversAfterFallbackPage",
			filter: "shared-target",
			seeds:  rangeServerSeeds(2*apidefaults.DefaultChunkSize+1, "shared-target", true, nil),
			onCollected: func(n int, c *Cache) {
				switch n {
				case apidefaults.DefaultChunkSize:
					c.ok = false
				case 2 * apidefaults.DefaultChunkSize:
					c.ok = true
				}
			},
		},
	}

	t.Run("ParameterValidation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		c := &Cache{
			Config: Config{
				Tracer: tracing.NoopTracer("test"),
			},
		}

		_, err := stream.Collect(funcs.rangeByName(c, ctx, ""))
		require.ErrorAs(t, err, new(*trace.BadParameterError))
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				syncTestRangeServersWithTargetName(t, funcs, tt)
			})
		})
	}

	t.Run("DeletedResourceNotReturned", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			neverOK bool
		}{
			{name: "HealthyCache", neverOK: false},
			{name: "Fallback", neverOK: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					syncTestDeletedRangeServerWithTargetName(t, funcs, tt.neverOK)
				})
			})
		}
	})
}

// syncTestRangeServersWithTargetName must be run within [synctest.Test].
func syncTestRangeServersWithTargetName[T hostIDGetterResource](
	t *testing.T,
	funcs rangeServersWithTargetNameFuncs[T],
	tt rangeServersWithTargetNameTestCase[T],
) {
	ctx := t.Context()

	p := newTestPack(t, func(cfg Config) Config {
		cfg = ForAuth(cfg)
		cfg.neverOK = tt.neverOK
		return cfg
	})
	t.Cleanup(p.Close)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.eventsC:
				// Discard events to avoid blocking the test.
			}
		}
	}()

	var expected []T
	for i, seed := range tt.seeds {
		hostID := seed.hostID
		if hostID == "" {
			hostID = fmt.Sprintf("%06d", i)
		}
		item := funcs.newResource(t, hostID, seed.targetName)
		require.NoError(t, funcs.create(ctx, p.presenceS, item))
		if seed.want {
			expected = append(expected, item)
		}
	}

	// Wait for cache to sync.
	synctest.Wait()

	var collected []T
	for item, err := range funcs.rangeByName(p.cache, ctx, tt.filter) {
		require.NoError(t, err)
		collected = append(collected, item)
		if tt.onCollected != nil {
			tt.onCollected(len(collected), p.cache)
		}
	}

	if len(expected) == 0 {
		require.Empty(t, collected)
		return
	}
	require.ElementsMatch(t, expected, collected)
}

// syncTestDeletedRangeServerWithTargetName must be run within [synctest.Test].
func syncTestDeletedRangeServerWithTargetName[T hostIDGetterResource](
	t *testing.T,
	funcs rangeServersWithTargetNameFuncs[T],
	neverOK bool,
) {
	ctx := t.Context()

	p := newTestPack(t, func(cfg Config) Config {
		cfg = ForAuth(cfg)
		cfg.neverOK = neverOK
		return cfg
	})
	t.Cleanup(p.Close)

	item := funcs.newResource(t, "000000", "target")
	require.NoError(t, funcs.create(ctx, p.presenceS, item))

	// Wait for cache to sync.
	synctest.Wait()

	collected, err := stream.Collect(funcs.rangeByName(p.cache, ctx, "target"))
	require.NoError(t, err)
	require.Equal(t, []T{item}, collected)

	require.NoError(t, funcs.delete(ctx, p.presenceS, item))

	// Wait for cache to sync.
	synctest.Wait()

	collected, err = stream.Collect(funcs.rangeByName(p.cache, ctx, "target"))
	require.NoError(t, err)
	require.Empty(t, collected)
}

func benchmarkRangeServersWithTargetName[T hostIDGetterResource](b *testing.B, funcs rangeServersWithTargetNameFuncs[T]) {
	b.Helper()
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	const count = 2 * apidefaults.DefaultChunkSize
	const targetName = "shared-target"

	tests := []struct {
		name           string
		neverOK        bool
		firstMatchOnly bool
	}{
		{name: "RangeByName_FirstMatch", firstMatchOnly: true},
		{name: "RangeByName_FirstMatch_Fallback", firstMatchOnly: true, neverOK: true},
		{name: "RangeByName_AllMatches"},
		{name: "RangeByName_AllMatches_Fallback", neverOK: true},
	}

	for _, tt := range tests {
		b.Run(fmt.Sprintf("%s_%dServers", tt.name, count), func(b *testing.B) {
			ctx := b.Context()
			b.ReportAllocs()

			bk, err := memory.New(memory.Config{Context: ctx, Mirror: true})
			require.NoError(b, err)
			b.Cleanup(func() { _ = bk.Close() })

			// Populate the backend before creating the cache.
			// New() will block until the cache is ready.
			var kindName string
			presenceS := local.NewPresenceService(bk)
			firstMatchIndex := apidefaults.DefaultChunkSize - 1
			firstMatchHostID := fmt.Sprintf("%06d", firstMatchIndex)
			lastMatchIndex := count - 1
			lastMatchHostID := fmt.Sprintf("%06d", lastMatchIndex)

			// Add bulk resources with the first match near the end of the first
			// backend page and the last match at the end of the second page to
			// ensure the full range is processed.
			for i := range count {
				hostID := fmt.Sprintf("%06d", i)
				name := fmt.Sprintf("target-%d", i+1)
				if i == firstMatchIndex {
					name = targetName
				}
				if i == lastMatchIndex {
					name = targetName
				}

				s := funcs.newResource(b, hostID, name)
				require.NoError(b, funcs.create(ctx, presenceS, s))
				if kindName == "" {
					kindName = s.GetKind()
				}
			}

			require.NotEmpty(b, kindName)
			watch := types.WatchKind{Kind: kindName}
			c, err := New(Config{
				Context:  ctx,
				Presence: presenceS,
				Events:   local.NewEventsService(bk),
				Watches:  []types.WatchKind{watch},
				neverOK:  tt.neverOK,
			})
			require.NoError(b, err)
			b.Cleanup(func() { c.Close() })

			for b.Loop() {
				if tt.firstMatchOnly {
					var matched bool
					for server, err := range funcs.rangeByName(c, ctx, targetName) {
						require.NoError(b, err)
						require.Equal(b, firstMatchHostID, server.GetHostID())
						matched = true
						break
					}
					require.True(b, matched)
					continue
				}

				servers, err := stream.Collect(funcs.rangeByName(c, ctx, targetName))
				require.NoError(b, err)
				require.Len(b, servers, 2)
				require.ElementsMatch(b,
					[]string{firstMatchHostID, lastMatchHostID},
					[]string{servers[0].GetHostID(), servers[1].GetHostID()},
				)
			}
		})
	}
}
