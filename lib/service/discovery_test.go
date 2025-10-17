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

package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/discovery"
	"github.com/gravitational/teleport/lib/utils"
)

func TestTeleportProcessIntegrationsOnly(t *testing.T) {
	for _, tt := range []struct {
		name              string
		inputFeatureCloud bool
		inputAuthEnabled  bool
		integrationOnly   bool
	}{
		{
			name:              "self-hosted",
			inputFeatureCloud: false,
			inputAuthEnabled:  false,
			integrationOnly:   false,
		},
		{
			name:              "cloud but discovery service is not running side-by-side with Auth",
			inputFeatureCloud: false,
			inputAuthEnabled:  true,
			integrationOnly:   false,
		},
		{
			name:              "cloud and discovery service is not running side-by-side with Auth",
			inputFeatureCloud: false,
			inputAuthEnabled:  true,
			integrationOnly:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := TeleportProcess{
				Config: &servicecfg.Config{
					Auth: servicecfg.AuthConfig{
						Enabled: tt.inputAuthEnabled,
					},
				},
			}

			modulestest.SetTestModules(t, modulestest.Modules{TestFeatures: modules.Features{
				Cloud: tt.inputFeatureCloud,
			}})

			require.Equal(t, tt.integrationOnly, p.integrationOnlyCredentials())
		})
	}
}

func TestTeleportProcess_initDiscoveryService(t *testing.T) {

	tests := []struct {
		name      string
		cfg       servicecfg.AccessGraphConfig
		rsp       *clusterconfigpb.AccessGraphConfig
		err       error
		want      discovery.AccessGraphConfig
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "local access graph",
			cfg: servicecfg.AccessGraphConfig{
				Enabled:  true,
				Addr:     "localhost:5000",
				Insecure: true,
			},
			rsp: nil,
			err: nil,
			want: discovery.AccessGraphConfig{
				Enabled:  true,
				Addr:     "localhost:5000",
				Insecure: true,
			},
			assertErr: require.NoError,
		},
		{
			name: "access graph disabled locally but enabled in auth",
			cfg: servicecfg.AccessGraphConfig{
				Enabled: false,
			},
			rsp: &clusterconfigpb.AccessGraphConfig{
				Enabled:  true,
				Address:  "localhost:5000",
				Insecure: true,
			},
			err: nil,
			want: discovery.AccessGraphConfig{
				Enabled:  true,
				Addr:     "localhost:5000",
				Insecure: true,
			},
			assertErr: require.NoError,
		},
		{
			name: "access graph disabled locally and auth doesn't implement GetClusterAccessGraphConfig",
			cfg: servicecfg.AccessGraphConfig{
				Enabled: false,
			},
			rsp: nil,
			err: trace.NotImplemented("err"),
			want: discovery.AccessGraphConfig{
				Enabled: false,
			},
			assertErr: require.NoError,
		},
		{
			name: "access graph disabled locally and auth call fails",
			cfg: servicecfg.AccessGraphConfig{
				Enabled: false,
			},
			rsp: nil,
			err: trace.BadParameter("err"),
			want: discovery.AccessGraphConfig{
				Enabled: false,
			},
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessGraphCfg, err := buildAccessGraphFromTAGOrFallbackToAuth(
				context.Background(),
				&servicecfg.Config{
					AccessGraph: tt.cfg,
				},
				&fakeClient{
					rsp: tt.rsp,
					err: tt.err,
				},
				slog.Default(),
			)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, accessGraphCfg)
		})
	}
}
func TestProcessPublicProxyAddr(t *testing.T) {
	proxyServerWithPublicAddr := func(addr string) *types.ServerV2 {
		return &types.ServerV2{
			Spec: types.ServerSpecV2{
				PublicAddrs: []string{addr},
			},
		}
	}

	tests := []struct {
		name        string
		config      *servicecfg.Config
		proxyGetter proxiesGetter
		wantAddr    string
		errCheck    require.ErrorAssertionFunc
	}{
		{
			name: "proxy server was set in config",
			config: &servicecfg.Config{
				ProxyServer: utils.NetAddr{Addr: "proxy.example.com:3080"},
			},
			wantAddr: "proxy.example.com:3080",
			errCheck: require.NoError,
		},
		{
			name: "proxy is running alongside discovery service",
			config: &servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
					PublicAddrs: []utils.NetAddr{
						{Addr: "public.proxy.com:443"},
					},
				},
			},
			wantAddr: "public.proxy.com:443",
			errCheck: require.NoError,
		},
		{
			name:   "discovery service is running alongside auth, (no proxy server defined and no proxy service enabled)",
			config: &servicecfg.Config{},
			proxyGetter: &mockProxyGetter{
				servers: []types.Server{proxyServerWithPublicAddr("proxy.example:8080")},
			},
			wantAddr: "proxy.example:8080",
			errCheck: require.NoError,
		},
		{
			name:   "no proxy available",
			config: &servicecfg.Config{},
			proxyGetter: &mockProxyGetter{
				servers: []types.Server{},
			},
			errCheck: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			process := &TeleportProcess{
				Config: tt.config,
			}
			addr, err := process.publicProxyAddr(tt.proxyGetter)
			tt.errCheck(t, err)
			require.Equal(t, tt.wantAddr, addr)
		})
	}
}

type mockProxyGetter struct {
	servers []types.Server
}

func (f *mockProxyGetter) GetProxies() ([]types.Server, error) {
	return f.servers, nil
}

type fakeClient struct {
	authclient.ClientI
	rsp *clusterconfigpb.AccessGraphConfig
	err error
}

func (f *fakeClient) GetClusterAccessGraphConfig(ctx context.Context) (*clusterconfigpb.AccessGraphConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rsp, nil
}
