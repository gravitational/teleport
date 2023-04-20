/*
Copyright 2021 Gravitational, Inc.

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

package app

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
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
	}{
		"WithHealthyApp": {
			match: true,
		},
		"WithUnhealthyApp": {
			dialErr: errors.New("failed to connect"),
			match:   false,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			match := MatchHealthy(&mockProxyClient{
				remoteSite: &mockRemoteSite{
					dialErr: test.dialErr,
				},
			}, "")

			app, err := types.NewAppV3(
				types.Metadata{
					Name:      "test-app",
					Namespace: defaults.Namespace,
				},
				types.AppSpecV3{
					URI: "https://app.localhost",
				},
			)
			require.NoError(t, err)

			appServer, err := types.NewAppServerV3FromApp(app, "localhost", "123")
			require.NoError(t, err)
			require.Equal(t, test.match, match(context.Background(), appServer))
		})
	}
}

type mockProxyClient struct {
	reversetunnel.Tunnel
	remoteSite *mockRemoteSite
}

func (p *mockProxyClient) GetSite(_ string) (reversetunnel.RemoteSite, error) {
	return p.remoteSite, nil
}

type mockRemoteSite struct {
	reversetunnel.RemoteSite
	dialErr error
}

func (r *mockRemoteSite) Dial(_ reversetunnel.DialParams) (net.Conn, error) {
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
