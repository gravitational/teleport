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
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestMatchAll(t *testing.T) {
	falseMatcher := func(_ types.AppServer) bool { return false }
	trueMatcher := func(_ types.AppServer) bool { return true }

	require.True(t, MatchAll(trueMatcher, trueMatcher, trueMatcher)(nil))
	require.False(t, MatchAll(trueMatcher, trueMatcher, falseMatcher)(nil))
	require.False(t, MatchAll(falseMatcher, falseMatcher, falseMatcher)(nil))
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
			identity := &tlsca.Identity{RouteToApp: tlsca.RouteToApp{ClusterName: ""}}
			match := MatchHealthy(&mockProxyClient{
				remoteSite: &mockRemoteSite{
					dialErr: test.dialErr,
				},
			}, identity)

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
			require.Equal(t, test.match, match(appServer))
		})
	}
}

func TestMatchWithRetry(t *testing.T) {
	ctx := context.Background()

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

	tests := map[string]struct {
		getter       Getter
		matchResult  bool
		retryOptions MatchRetryOptions
		errAssert    require.ErrorAssertionFunc
	}{
		"WithMatchingAppServers": {
			getter: &mockGetter{
				getAppServersResult: []types.AppServer{appServer},
			},
			matchResult:  true,
			retryOptions: MatchRetryOptions{Interval: time.Second, MaxTime: time.Second},
			errAssert:    require.NoError,
		},
		"WithoutAppServers": {
			getter: &mockGetter{
				getAppServersResult: []types.AppServer{},
			},
			matchResult: true,
			// Retries 3 times before timeout.
			retryOptions: MatchRetryOptions{Interval: 10 * time.Millisecond, MaxTime: 30 * time.Millisecond},
			errAssert: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.IsType(t, err, trace.LimitExceeded(""))
			},
		},
		"WithAppServersNoMatching": {
			getter: &mockGetter{
				getAppServersResult: []types.AppServer{appServer},
			},
			matchResult: false,
			// Retries 4 times before timeout.
			retryOptions: MatchRetryOptions{Interval: 10 * time.Millisecond, MaxTime: 40 * time.Millisecond},
			errAssert: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.IsType(t, err, trace.LimitExceeded(""))
			},
		},
		"FalingToFetchAppServers": {
			getter: &mockGetter{
				getAppServersErr: trace.ConnectionProblem(nil, ""),
			},
			matchResult: false,
			// Retries 5 times before timeout.
			retryOptions: MatchRetryOptions{Interval: 10 * time.Millisecond, MaxTime: 50 * time.Millisecond},
			errAssert: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.IsType(t, err, trace.LimitExceeded(""))
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var err error

			// Ensures the match won't execeed the defined max time. We add 5
			// milliseconds to make the test less time sensitive.
			maxTime := test.retryOptions.MaxTime + 5*time.Millisecond
			require.Eventually(t, func() bool {
				_, err = MatchWithRetry(
					ctx,
					test.getter,
					func(_ types.AppServer) bool { return test.matchResult },
					test.retryOptions,
				)

				return true
			}, maxTime, time.Millisecond)

			test.errAssert(t, err)
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
	return nil, r.dialErr
}

type mockGetter struct {
	getAppServersResult  []types.AppServer
	getAppServersErr     error
	getClusterNameResult types.ClusterName
	getClusterNameErr    error
}

func (m *mockGetter) GetApplicationServers(context.Context, string) ([]types.AppServer, error) {
	return m.getAppServersResult, m.getAppServersErr
}

func (m *mockGetter) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return m.getClusterNameResult, m.getClusterNameErr
}
