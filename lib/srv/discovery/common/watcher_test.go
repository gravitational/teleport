/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
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
	watcher, err := NewWatcher(ctx, WatcherConfig{
		Fetchers: []Fetcher{appFetcher, noAuthFetcher, dbFetcher},
		Interval: time.Hour,
		Clock:    clock,
	})
	require.NoError(t, err)
	go watcher.Start()

	// Watcher should fetch once right away at watcher.Start.
	wantResources := types.ResourcesWithLabels{app1, app2, db}
	assertFetchResources(t, watcher, wantResources)

	// Watcher should fetch again after interval.
	clock.Advance(time.Hour + time.Minute)
	assertFetchResources(t, watcher, wantResources)
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

func (m *mockFetcher) Cloud() string {
	return m.cloud
}
