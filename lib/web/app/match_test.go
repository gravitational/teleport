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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
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

type mockClusterGetter struct {
	reversetunnelclient.ClusterGetter
	cluster *mockCluster
}

func (p *mockClusterGetter) Cluster(context.Context, string) (reversetunnelclient.Cluster, error) {
	return p.cluster, nil
}

type mockCluster struct {
	reversetunnelclient.Cluster
	dialErr error
}

func (r *mockCluster) Dial(_ reversetunnelclient.DialParams) (net.Conn, error) {
	if r.dialErr != nil {
		return nil, r.dialErr
	}

	return &mockDialConn{}, nil
}

func (r *mockCluster) GetName() string {
	return "mockCluster"
}

type mockDialConn struct {
	net.Conn
}

func (c *mockDialConn) Close() error {
	return nil
}
