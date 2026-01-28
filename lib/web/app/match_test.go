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

package app

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
)

func mustNewAppServer(t *testing.T, origin string) func() types.AppServer {
	t.Helper()
	return func() types.AppServer {
		app, err := types.NewAppV3(
			types.Metadata{
				Name:      "test-app",
				Namespace: defaults.Namespace,
				Labels: map[string]string{
					types.OriginLabel: origin,
				},
			},
			types.AppSpecV3{
				URI: "https://app.localhost",
			},
		)
		require.NoError(t, err)

		appServer, err := types.NewAppServerV3FromApp(app, "localhost", "123")
		require.NoError(t, err)

		return appServer
	}
}

func TestResolveByName(t *testing.T) {
	apps := []*types.AppV3{
		createMCPApp(t, "example-1", nil /* labels */),
		createMCPApp(t, "example-2", nil /* labels */),
		createMCPApp(t, "example-3", nil /* labels */),
	}

	for name, tc := range map[string]struct {
		appName         string
		appServers      []types.AppServer
		assertError     require.ErrorAssertionFunc
		assertAppServer require.ValueAssertionFunc
	}{
		"match": {
			appName: apps[0].Metadata.Name,
			appServers: []types.AppServer{
				createAppServerWithApp(t, apps[0]),
				createAppServerWithApp(t, apps[1]),
				createAppServerWithApp(t, apps[2]),
			},
			assertError:     require.NoError,
			assertAppServer: expectAppServerWithApp(apps[0].Metadata.Name),
		},
		"no match": {
			appName: "example-x",
			appServers: []types.AppServer{
				createAppServerWithApp(t, apps[0]),
				createAppServerWithApp(t, apps[1]),
				createAppServerWithApp(t, apps[2]),
			},
			assertError:     require.Error,
			assertAppServer: require.Nil,
		},
		"multi servers match": {
			appName: apps[0].Metadata.Name,
			appServers: []types.AppServer{
				// Create multiple app servers for the first app.
				createAppServerWithApp(t, apps[0]),
				createAppServerWithApp(t, apps[0]),
				createAppServerWithApp(t, apps[0]),
				createAppServerWithApp(t, apps[1]),
				createAppServerWithApp(t, apps[2]),
			},
			assertError:     require.NoError,
			assertAppServer: expectAppServerWithApp(apps[0].Metadata.Name),
		},
		"no servers, no match": {
			appName:         apps[0].Metadata.Name,
			appServers:      []types.AppServer{},
			assertError:     require.Error,
			assertAppServer: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			clock := clockwork.NewFakeClock()

			bk, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)

			type client struct {
				types.Events
			}

			appService := local.NewAppService(bk)
			for _, app := range apps {
				require.NoError(t, appService.CreateApp(t.Context(), app))
			}

			w, err := services.NewAppServersWatcher(ctx, services.AppServersWatcherConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component:      "test",
					MaxRetryPeriod: 200 * time.Millisecond,
					Client: &client{
						Events: local.NewEventsService(bk),
					},
				},
				AppServersGetter: &mockAppServersGetter{servers: tc.appServers},
			})
			require.NoError(t, err)

			res, err := ResolveByName(t.Context(), &mockCluster{watcher: w}, tc.appName)
			tc.assertError(t, err)
			tc.assertAppServer(t, res)
		})
	}
}

func expectAppServerWithApp(name string) require.ValueAssertionFunc {
	return func(t require.TestingT, i1 any, i2 ...any) {
		require.IsType(t, &types.AppServerV3{}, i1)
		appServer, _ := i1.(types.AppServer)
		require.Equal(t, name, appServer.GetApp().GetName())
	}
}

type mockAppServersGetter struct {
	servers []types.AppServer
}

func (m *mockAppServersGetter) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	return m.servers, nil
}

type mockClusterGetter struct {
	reversetunnelclient.ClusterGetter
	cluster *mockCluster
}

func (p *mockClusterGetter) Cluster(context.Context, string) (reversetunnelclient.Cluster, error) {
	return p.cluster, nil
}

type mockCluster struct {
	reversetunnelclient.Cluster
	watcher     *services.GenericWatcher[types.AppServer, readonly.AppServer]
	dialErr     error
	accessPoint *mockAuthClient
}

func (r *mockCluster) GetName() string {
	return "mockCluster"
}

func (r *mockCluster) AppServerWatcher() (*services.GenericWatcher[types.AppServer, readonly.AppServer], error) {
	return r.watcher, nil
}

func (r *mockCluster) Dial(_ reversetunnelclient.DialParams) (net.Conn, error) {
	if r.dialErr != nil {
		return nil, r.dialErr
	}
	return &mockDialConn{}, nil
}

func (r *mockCluster) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return r.accessPoint, nil
}

type mockDialConn struct {
	net.Conn
}

func (c *mockDialConn) Close() error {
	return nil
}
