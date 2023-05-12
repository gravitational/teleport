/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type accessPointMock struct{}

// GetClusterNetworkingConfig returns a cluster config.
func (a *accessPointMock) GetClusterNetworkingConfig(_ context.Context, _ ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
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
		cfgFn    func() *Config
		assertFn require.BoolAssertionFunc
	}{
		{
			name: "AssistEnabled is true when proxy API key is set",
			cfgFn: func() *Config {
				cfg := MakeDefaultConfig()
				cfg.Proxy.AssistAPIKey = "test-api-key"
				return cfg
			},
			assertFn: require.True,
		},
		{
			name: "AssistEnabled is false when proxy API key is not set",
			cfgFn: func() *Config {
				cfg := MakeDefaultConfig()
				cfg.Proxy.AssistAPIKey = ""
				return cfg
			},
			assertFn: require.False,
		},
		{
			name: "AssistEnabled is true when proxy API key is set - v2 config",
			cfgFn: func() *Config {
				cfg := MakeDefaultConfig()
				cfg.Version = defaults.TeleportConfigVersionV2
				cfg.Proxy.AssistAPIKey = "test-api-key"
				return cfg
			},
			assertFn: require.True,
		},
		{
			name: "AssistEnabled is false when proxy API key is not set - v2 config",
			cfgFn: func() *Config {
				cfg := MakeDefaultConfig()
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
