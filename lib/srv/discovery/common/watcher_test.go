/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestWatcher(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app1, err := types.NewAppV3(types.Metadata{Name: "app1"}, types.AppSpecV3{Cloud: types.CloudAWS})
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{Name: "app2"}, types.AppSpecV3{Cloud: types.CloudAWS})
	require.NoError(t, err)

	db, err := types.NewDatabaseV3(types.Metadata{Name: "db"}, types.DatabaseSpecV3{Protocol: "mysql", URI: "db.mysql.database.azure.com:1234"})
	require.NoError(t, err)

	appFetcher := &mockFetcher{
		resources:    types.ResourcesWithLabels{app1, app2},
		resourceType: types.KindApp,
		cloud:        types.CloudAWS,
	}
	dbFetcher := &mockFetcher{
		resources:    types.ResourcesWithLabels{db},
		resourceType: types.KindDatabase,
		cloud:        types.CloudAzure,
	}
	noAuthFetcher := &mockFetcher{
		noAuth:       true,
		resourceType: types.KindKubeServer,
		cloud:        types.CloudGCP,
	}

	clock := clockwork.NewFakeClock()
	fetchIterations := atomic.Uint32{}
	watcher, err := NewWatcher(ctx, WatcherConfig{
		FetchersFn: StaticFetchers([]Fetcher{appFetcher, noAuthFetcher, dbFetcher}),
		Interval:   time.Hour,
		Clock:      clock,
		Origin:     types.OriginCloud,
		PreFetchHookFn: func() {
			fetchIterations.Add(1)
		},
	})
	require.NoError(t, err)
	go watcher.Start()

	// Watcher should fetch once right away at watcher.Start.
	wantResources := types.ResourcesWithLabels{app1, app2, db}
	assertFetchResources(t, watcher, wantResources)

	// Watcher should fetch again after interval.
	clock.Advance(time.Hour + time.Minute)
	assertFetchResources(t, watcher, wantResources)

	require.Equal(t, uint32(2), fetchIterations.Load())
}

func TestWatcherWithDynamicFetchers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app1, err := types.NewAppV3(types.Metadata{Name: "app1"}, types.AppSpecV3{Cloud: types.CloudAWS})
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{Name: "app2"}, types.AppSpecV3{Cloud: types.CloudAWS})
	require.NoError(t, err)

	appFetcher := &mockFetcher{
		resources:    types.ResourcesWithLabels{app1, app2},
		resourceType: types.KindApp,
		cloud:        types.CloudAWS,
	}

	noAuthFetcher := &mockFetcher{
		noAuth:       true,
		resourceType: types.KindKubeServer,
		cloud:        types.CloudGCP,
	}

	// Start with two watchers.
	fetchers := []Fetcher{appFetcher, noAuthFetcher}
	fetchersFn := func() []Fetcher {
		return fetchers
	}

	clock := clockwork.NewFakeClock()
	watcher, err := NewWatcher(ctx, WatcherConfig{
		FetchersFn: fetchersFn,
		Interval:   time.Hour,
		Clock:      clock,
		Origin:     types.OriginCloud,
	})
	require.NoError(t, err)
	go watcher.Start()

	// Watcher should fetch once right away at watcher.Start.
	assertFetchResources(t, watcher, types.ResourcesWithLabels{app1, app2})

	// Add an extra fetcher during runtime.
	db, err := types.NewDatabaseV3(types.Metadata{Name: "db"}, types.DatabaseSpecV3{Protocol: "mysql", URI: "db.mysql.database.azure.com:1234"})
	require.NoError(t, err)
	dbFetcher := &mockFetcher{
		resources:    types.ResourcesWithLabels{db},
		resourceType: types.KindDatabase,
		cloud:        types.CloudAzure,
	}
	fetchers = append(fetchers, dbFetcher)

	// During next iteration, the new fetcher must be used and a 3rd resource must appear.
	clock.Advance(time.Hour + time.Minute)
	assertFetchResources(t, watcher, types.ResourcesWithLabels{app1, app2, db})
}

func assertFetchResources(t *testing.T, watcher *Watcher, wantResources types.ResourcesWithLabels) {
	select {
	case fetchResources := <-watcher.ResourcesC():
		require.ElementsMatch(t, wantResources, fetchResources)
	case <-time.After(time.Second):
		require.Fail(t, "timeout waiting for resources")
	}
}

type mockFetcher struct {
	resources    types.ResourcesWithLabels
	resourceType string
	cloud        string
	noAuth       bool
}

func (m *mockFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	if m.noAuth {
		return nil, trace.AccessDenied("access denied")
	}
	return m.resources, nil
}

func (m *mockFetcher) ResourceType() string {
	return m.resourceType
}

func (m *mockFetcher) FetcherType() string {
	return "empty"
}

func (m *mockFetcher) IntegrationName() string {
	return ""
}

func (m *mockFetcher) GetDiscoveryConfigName() string {
	return ""
}
func (m *mockFetcher) Cloud() string {
	return m.cloud
}
