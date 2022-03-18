/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestTunnelStrategyMarshalling(t *testing.T) {
	tests := []struct {
		json     string
		yaml     string
		strategy *TunnelStrategyV1
	}{
		{
			json: `{"type":"agent_mesh"}`,
			yaml: "type: agent_mesh\n",
			strategy: &TunnelStrategyV1{Strategy: &TunnelStrategyV1_AgentMesh{
				AgentMesh: &AgentMeshTunnelStrategy{},
			}},
		},
		{
			json: `{"type":"proxy_peering"}`,
			yaml: "type: proxy_peering\n",
			strategy: &TunnelStrategyV1{Strategy: &TunnelStrategyV1_ProxyPeering{
				ProxyPeering: &ProxyPeeringTunnelStrategy{},
			}}},
		{
			json: `{"agent_connection_count":2,"type":"proxy_peering"}`,
			yaml: "agent_connection_count: 2\ntype: proxy_peering\n",
			strategy: &TunnelStrategyV1{Strategy: &TunnelStrategyV1_ProxyPeering{
				ProxyPeering: &ProxyPeeringTunnelStrategy{
					AgentConnectionCount: 2,
				},
			}},
		},
	}

	for _, tc := range tests {
		data, err := yaml.Marshal(tc.strategy)
		require.NoError(t, err)
		require.Equal(t, []byte(tc.yaml), data)

		data, err = json.Marshal(tc.strategy)
		require.NoError(t, err)
		require.Equal(t, []byte(tc.json), data)

		strategy := &TunnelStrategyV1{}
		err = json.Unmarshal([]byte(tc.json), strategy)
		require.NoError(t, err)
		require.Equal(t, tc.strategy, strategy)

		strategy = &TunnelStrategyV1{}
		err = yaml.Unmarshal([]byte(tc.yaml), strategy)
		require.NoError(t, err)
		require.Equal(t, tc.strategy, strategy)
	}
}
