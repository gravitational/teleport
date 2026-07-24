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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
)

func TestMatchAppServerForRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc string

		appName  string
		appAddr  string
		appScope string

		name string
		addr string

		wantMatch bool
	}{
		{
			desc:      "all match",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "foo.example.com",
			wantMatch: true,
		},
		{
			desc:      "fallback no name (match)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "foo.example.com",
			wantMatch: true,
		},
		{
			desc:      "fallback no name (mismatch)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "bar.example.com",
			wantMatch: false,
		},
		{
			desc:      "different name",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "bar",
			addr:      "foo.example.com",
			wantMatch: false,
		},
		{
			desc:      "different addr",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "bar.example.com",
			wantMatch: false,
		},
		{
			desc:      "name only (match)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "",
			wantMatch: true,
		},
		{
			desc:      "name only (mismatch)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "bar",
			addr:      "",
			wantMatch: false,
		},
		{
			desc:      "neither name nor addr matches nothing",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "",
			wantMatch: false,
		},
		// scoped app matching - matches based on its computed app name and scope as the subdomain.
		{
			desc:      "scoped app matches its hash under a different proxy",
			appName:   "grafana",
			appScope:  "/staging/west",
			appAddr:   scopedapp.ScopedAppPublicAddr("/staging/west", "grafana", "teleport.example.com"),
			name:      "grafana",
			addr:      scopedapp.ScopedAppPublicAddr("/staging/west", "grafana", "teleportalt.example.com"),
			wantMatch: true,
		},
		{
			desc:      "scoped app does not match a different scope's hash",
			appName:   "grafana",
			appScope:  "/staging/west",
			appAddr:   scopedapp.ScopedAppPublicAddr("/staging/west", "grafana", "teleport.example.com"),
			name:      "grafana",
			addr:      scopedapp.ScopedAppPublicAddr("/prod", "grafana", "teleportalt.example.com"),
			wantMatch: false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			appServer, err := types.NewAppServerV3(
				types.Metadata{Name: test.appName},
				types.AppServerSpecV3{
					HostID: "test-host-id",
					App: &types.AppV3{
						Metadata: types.Metadata{Name: test.appName},
						Scope:    test.appScope,
						Spec: types.AppSpecV3{
							PublicAddr: test.appAddr,
							URI:        "http://localhost:12345",
						},
					},
				},
			)
			require.NoError(t, err)

			require.Equal(
				t,
				test.wantMatch,
				MatchAppServerForRoute(test.name, test.addr)(appServer),
			)
		})
	}
}

func TestHostIsProxyOrSubdomain(t *testing.T) {
	t.Parallel()
	const proxy = "teleport.example.com"
	for _, test := range []struct {
		desc  string
		host  string
		proxy string
		want  bool
	}{
		{
			desc:  "exact match",
			host:  proxy,
			proxy: proxy,
			want:  true,
		},
		{
			desc:  "subdomain",
			host:  "app.teleport.example.com",
			proxy: proxy,
			want:  true,
		},
		{
			desc:  "multi-label subdomain",
			host:  "a.b.teleport.example.com",
			proxy: proxy,
			want:  true,
		},
		{
			// The bug this guards against: a raw suffix check would accept this
			// because it ends in "teleport.example.com" with no label boundary.
			desc:  "no label boundary",
			host:  "evilteleport.example.com",
			proxy: proxy,
			want:  false,
		},
		{
			desc:  "proxy name as a lower-level domain",
			host:  "teleport.example.com.evil.com",
			proxy: proxy,
			want:  false,
		},
		{
			desc:  "unrelated domain",
			host:  "app.somewebsite.com",
			proxy: proxy,
			want:  false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.want, hostIsProxyOrSubdomain(test.host, test.proxy))
		})
	}
}

func TestExtractHostname(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc         string
		fqdn         string
		wantHostname string
		wantErr      bool
	}{
		{
			desc:         "plain hostname",
			fqdn:         "blah.teleport.example.com",
			wantHostname: "blah.teleport.example.com",
		},
		{
			desc:         "numeric port is stripped",
			fqdn:         "blah.teleport.example.com:8443",
			wantHostname: "blah.teleport.example.com",
		},
		{
			// App names may contain underscores and start with a digit
			desc:         "underscore in app label",
			fqdn:         "my_app.teleport.example.com",
			wantHostname: "my_app.teleport.example.com",
		},
		{
			desc:         "label starting with a digit",
			fqdn:         "1stapp.teleport.example.com",
			wantHostname: "1stapp.teleport.example.com",
		},
		{
			desc:         "all-digit label",
			fqdn:         "123.teleport.example.com",
			wantHostname: "123.teleport.example.com",
		},
		{
			desc:    "suffix disguised as a port",
			fqdn:    "bloo.example.com:443@malicious.com",
			wantErr: true,
		},
		{
			desc:    "empty port",
			fqdn:    "bloo.example.com:",
			wantErr: true,
		},
		{
			desc:    "empty host",
			fqdn:    ":443",
			wantErr: true,
		},
		{
			desc:    "too many colons",
			fqdn:    "a:b:c",
			wantErr: true,
		},
		{
			desc:    "empty string",
			fqdn:    "",
			wantErr: true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			host, err := extractHostname(test.fqdn)
			if test.wantErr {
				require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantHostname, host)
		})
	}
}

// emptyCluster is a minimal reversetunnelclient.Cluster without an app-server watcher.
type emptyCluster struct {
	reversetunnelclient.Cluster
	name string
}

func (c emptyCluster) GetName() string { return c.name }

func (c emptyCluster) AppServerWatcher() (*services.GenericWatcher[types.AppServer, readonly.AppServer], error) {
	return nil, trace.NotFound("no app server watcher in test")
}

// TestResolveFQDN_ProxyWithPort verifies that a request under a proxy whose DNS
// name includes a port does not get rejected as BadParameter.
func TestResolveFQDN_ProxyWithPort(t *testing.T) {
	t.Parallel()

	getter := &reversetunnelclient.FakeServer{
		FakeClusters: []reversetunnelclient.Cluster{emptyCluster{name: "local"}},
	}
	proxyDNSNames := []string{"proxy.example.com:3080"}

	_, _, err := ResolveFQDN(
		t.Context(),
		getter,
		"local",
		proxyDNSNames,
		"myapp.proxy.example.com",
		nil,
	)

	require.Error(t, err)
	require.False(t, trace.IsBadParameter(err),
		"proxy with port must not be rejected, got %v", err)
}

func TestPickAppServer(t *testing.T) {
	t.Parallel()

	mustMakeAppServer := func(name string) types.AppServer {
		s, err := types.NewAppServerV3(
			types.Metadata{Name: name},
			types.AppServerSpecV3{
				HostID: "host-" + name,
				App: &types.AppV3{
					Metadata: types.Metadata{Name: name},
					Spec:     types.AppSpecV3{PublicAddr: "dup.example.com", URI: "http://localhost:1"},
				},
			},
		)
		require.NoError(t, err)
		return s
	}

	app1, app2 := mustMakeAppServer("dup-app-1"), mustMakeAppServer("dup-app-2")
	servers := []types.AppServer{app1, app2}
	onlyApp1 := func(a types.Application) bool { return a.GetName() == "dup-app-1" }
	none := func(types.Application) bool { return false }

	t.Run("prefers the accessible app", func(t *testing.T) {
		for range 100 {
			require.Equal(t, "dup-app-1", pickAppServer(servers, onlyApp1).GetApp().GetName())
		}
	})

	t.Run("nil filter picks among all (legacy behavior)", func(t *testing.T) {
		got := pickAppServer(servers, nil)
		require.Contains(t, []string{"dup-app-1", "dup-app-2"}, got.GetApp().GetName())
	})

	t.Run("no accessible match falls back to all", func(t *testing.T) {
		got := pickAppServer(servers, none)
		require.NotNil(t, got)
		require.Contains(t, []string{"dup-app-1", "dup-app-2"}, got.GetApp().GetName())
	})
}
