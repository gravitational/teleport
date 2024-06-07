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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

type accessPointMock struct{}

// GetClusterNetworkingConfig returns a cluster config.
func (a *accessPointMock) GetClusterNetworkingConfig(_ context.Context) (types.ClusterNetworkingConfig, error) {
	return &types.ClusterNetworkingConfigV2{
		Spec: types.ClusterNetworkingConfigSpecV2{
			RoutingStrategy: types.RoutingStrategy_MOST_RECENT,
		},
	}, nil

}

func Test_proxySettings_GetProxySettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfgFn    func() *servicecfg.Config
		assertFn require.BoolAssertionFunc
	}{
		{
			name: "AssistEnabled is true when proxy API key is set",
			cfgFn: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.Proxy.AssistAPIKey = "test-api-key"
				return cfg
			},
			assertFn: require.True,
		},
		{
			name: "AssistEnabled is false when proxy API key is not set",
			cfgFn: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.Proxy.AssistAPIKey = ""
				return cfg
			},
			assertFn: require.False,
		},
		{
			name: "AssistEnabled is true when proxy API key is set - v2 config",
			cfgFn: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.Version = defaults.TeleportConfigVersionV2
				cfg.Proxy.AssistAPIKey = "test-api-key"
				return cfg
			},
			assertFn: require.True,
		},
		{
			name: "AssistEnabled is false when proxy API key is not set - v2 config",
			cfgFn: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.Version = defaults.TeleportConfigVersionV2
				cfg.Proxy.AssistAPIKey = ""
				return cfg
			},
			assertFn: require.False,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &proxySettings{
				cfg:          tt.cfgFn(),
				proxySSHAddr: utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3080"},
				accessPoint:  &accessPointMock{},
			}

			proxySettings, err := p.GetProxySettings(context.Background())
			require.NoError(t, err)
			require.NotNil(t, proxySettings)

			tt.assertFn(t, proxySettings.AssistEnabled)
		})
	}
}
