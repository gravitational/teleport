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
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// TestApps tests that CRUD operations on application resources are
// replicated from the backend to the cache.
func TestApps(t *testing.T) {
	t.Parallel()

	p, err := newPack(t, ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Application]{
		newResource: func(name string) (types.Application, error) {
			return types.NewAppV3(types.Metadata{
				Name: name,
			}, types.AppSpecV3{
				URI: "localhost",
			})
		},
		create:     p.apps.CreateApp,
		list:       p.apps.ListApps,
		Range:      p.apps.Apps,
		cacheGet:   p.cache.GetApp,
		cacheList:  p.cache.ListApps,
		cacheRange: p.cache.Apps,
		update:     p.apps.UpdateApp,
		deleteAll:  p.apps.DeleteAllApps,
	})
}

// TestApplicationServers tests that CRUD operations on app servers are
// replicated from the backend to the cache.
func TestApplicationServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetApplicationServers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.AppServer]{
			newResource: func(name string) (types.AppServer, error) {
				app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "localhost"})
				require.NoError(t, err)
				return types.NewAppServerV3FromApp(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertApplicationServer),
			list: getAllAdapter(func(ctx context.Context) ([]types.AppServer, error) {
				return p.presenceS.GetApplicationServers(ctx, apidefaults.Namespace)
			}),
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.AppServer, error) {
				return p.cache.GetApplicationServers(ctx, apidefaults.Namespace)
			}),
			update: withKeepalive(p.presenceS.UpsertApplicationServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		}, withSkipPaginationTest())
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.AppServer]{
			newResource: func(name string) (types.AppServer, error) {
				app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "localhost"})
				require.NoError(t, err)
				return types.NewAppServerV3FromApp(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertApplicationServer),
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.AppServer, string, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindAppServer,
					StartKey:     pageToken,
					Limit:        int32(pageSize),
				}

				var out []types.AppServer
				resp, err := p.presenceS.ListResources(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, s := range resp.Resources {
					out = append(out, s.(types.AppServer))
				}

				return out, resp.NextKey, nil
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.AppServer, string, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindAppServer,
					Limit:        int32(pageSize),
					StartKey:     pageToken,
				}

				var out []types.AppServer
				resp, err := p.cache.ListResources(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, s := range resp.Resources {
					out = append(out, s.(types.AppServer))
				}

				return out, resp.NextKey, nil

			},
			update: withKeepalive(p.presenceS.UpsertApplicationServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		})
	})

}

func mustCreateAppServer(t testing.TB, hostID, appName string) types.AppServer {
	t.Helper()

	app, err := types.NewAppV3(types.Metadata{
		Name: appName,
	}, types.AppSpecV3{
		URI: "localhost",
	})
	require.NoError(t, err)

	appServer, err := types.NewAppServerV3FromApp(app, "localhost", hostID)
	require.NoError(t, err)
	return appServer
}

var appServerRangeFuncs = rangeServersWithTargetNameFuncs[types.AppServer]{
	newResource: mustCreateAppServer,
	create: func(ctx context.Context, presence services.Presence, s types.AppServer) error {
		_, err := presence.UpsertApplicationServer(ctx, s)
		return err
	},
	delete: func(ctx context.Context, presence services.Presence, s types.AppServer) error {
		return presence.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{
			HostId: s.GetHostID(),
			Name:   s.GetName(),
			Scope:  s.GetScope(),
		}.Build())
	},
	rangeByName: (*Cache).RangeApplicationServersWithName,
}

func TestRangeApplicationServersWithName(t *testing.T) {
	t.Parallel()
	testRangeServersWithTargetName(t, appServerRangeFuncs)
}

// TestListResourcesAppServerScopedPagination verifies that ListResources
// pagination spanning unscoped and scoped app servers returns every server
func TestListResourcesAppServerScopedPagination(t *testing.T) {
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

				key := func(s types.AppServer) string {
					return s.GetScope() + "/" + s.GetHostID() + "/" + s.GetName()
				}

				var expectedKeys []string
				for _, scope := range []string{"", "", "", "/prod", "/prod", "/staging"} {
					server := mustCreateAppServer(t, uuid.New().String(), "graf").(*types.AppServerV3)
					server.Scope = scope
					server.Spec.App.Scope = scope
					_, err := p.presenceS.UpsertApplicationServer(ctx, server)
					require.NoError(t, err)
					expectedKeys = append(expectedKeys, key(server))
				}

				// Wait for the cache to replicate all servers.
				synctest.Wait()

				var actualKeys []string
				startKey := ""
				for {
					resp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
						ResourceType: types.KindAppServer,
						Namespace:    apidefaults.Namespace,
						Limit:        1,
						StartKey:     startKey,
					})
					require.NoError(t, err)
					r := resp.Resources[0]
					server, ok := r.(types.AppServer)
					require.True(t, ok)
					if startKey != "" { // first entry
						if startKey == scopes.ResourceCursorScopedStart() {
							// The unscoped range ended exactly at a page boundary;
							// next entry should be scoped
							require.NotEmpty(t, server.GetScope())
						} else {
							require.Equal(t, startKey, services.GetCursorForAppServer(server))
						}
					}
					actualKeys = append(actualKeys, key(server))

					if resp.NextKey == "" {
						break
					}
					startKey = resp.NextKey
				}

				require.ElementsMatch(t, expectedKeys, actualKeys)
			})
		})
	}
}

func BenchmarkRangeApplicationServersWithName(b *testing.B) {
	benchmarkRangeServersWithTargetName(b, appServerRangeFuncs)
}
