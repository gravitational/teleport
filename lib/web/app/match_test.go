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
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

func TestMatchAll(t *testing.T) {
	falseMatcher := func(_ context.Context, _ types.AppServer) bool { return false }
	trueMatcher := func(_ context.Context, _ types.AppServer) bool { return true }

	require.True(t, MatchAll(trueMatcher, trueMatcher, trueMatcher)(nil, nil))
	require.False(t, MatchAll(trueMatcher, trueMatcher, falseMatcher)(nil, nil))
	require.False(t, MatchAll(falseMatcher, falseMatcher, falseMatcher)(nil, nil))
}

func TestMatchHealthy(t *testing.T) {
	testCases := map[string]struct {
		dialErr error
		match   bool
		app     func() types.AppServer
	}{
		"WithHealthyApp": {
			match: true,
			app:   mustNewAppServer(t, types.OriginDynamic),
		},
		"WithUnhealthyApp": {
			dialErr: errors.New("failed to connect"),
			match:   false,
			app:     mustNewAppServer(t, types.OriginDynamic),
		},
		"WithUnhealthyOktaApp": {
			dialErr: errors.New("failed to connect"),
			match:   true,
			app:     mustNewAppServer(t, types.OriginOkta),
		},
		"WithIntegrationApp": {
			dialErr: errors.New("failed to connect"),
			match:   true,
			app: func() types.AppServer {
				appServer := mustNewAppServer(t, types.OriginDynamic)()
				app := appServer.GetApp().Copy()
				app.Spec.Integration = "my-integration"
				appServer.SetApp(app)

				return appServer
			},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			match := MatchHealthy(&mockProxyClient{
				remoteSite: &mockRemoteSite{
					dialErr: test.dialErr,
				},
			}, "")

			require.Equal(t, test.match, match(context.Background(), test.app()))
		})
	}
}

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

type mockProxyClient struct {
	reversetunnelclient.Tunnel
	remoteSite *mockRemoteSite
}

func (p *mockProxyClient) GetSite(_ string) (reversetunnelclient.RemoteSite, error) {
	return p.remoteSite, nil
}

type mockRemoteSite struct {
	reversetunnelclient.RemoteSite
	dialErr error
}

func (r *mockRemoteSite) Dial(_ reversetunnelclient.DialParams) (net.Conn, error) {
	if r.dialErr != nil {
		return nil, r.dialErr
	}

	return &mockDialConn{}, nil
}

type mockDialConn struct {
	net.Conn
}

func (c *mockDialConn) Close() error {
	return nil
}
