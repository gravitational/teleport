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
		strategy isClusterNetworkingConfigSpecV2_TunnelStrategy
	}{
		{
			json: `{"tunnel_strategy":{"type":"agent_mesh"}}`,
			yaml: "tunnel_strategy:\n  type: agent_mesh\n",
			strategy: &ClusterNetworkingConfigSpecV2_AgentMesh{
				AgentMesh: &AgentMeshTunnelStrategy{Type: AgentMesh},
			},
		},
		{
			json: `{"tunnel_strategy":{"type":"proxy_peering"}}`,
			yaml: "tunnel_strategy:\n  type: proxy_peering\n",
			strategy: &ClusterNetworkingConfigSpecV2_ProxyPeering{
				ProxyPeering: &ProxyPeeringTunnelStrategy{Type: ProxyPeering},
			},
		},
		{
			json: `{"tunnel_strategy":{"type":"proxy_peering","agent_connection_count":2}}`,
			yaml: "tunnel_strategy:\n  agent_connection_count: 2\n  type: proxy_peering\n",
			strategy: &ClusterNetworkingConfigSpecV2_ProxyPeering{
				ProxyPeering: &ProxyPeeringTunnelStrategy{
					Type:                 ProxyPeering,
					AgentConnectionCount: 2,
				},
			},
		},
	}

	for _, tc := range tests {
		data, err := yaml.Marshal(tc.strategy)
		require.NoError(t, err)
		s := string(data)
		_ = s
		require.Equal(t, []byte(tc.yaml), data)

		data, err = json.Marshal(tc.strategy)
		require.NoError(t, err)
		s = string(data)
		_ = s
		require.Equal(t, []byte(tc.json), data)

		var actual interface{}
		switch tc.strategy.(type) {
		case *ClusterNetworkingConfigSpecV2_AgentMesh:
			actual = &ClusterNetworkingConfigSpecV2_AgentMesh{}
		case *ClusterNetworkingConfigSpecV2_ProxyPeering:
			actual = &ClusterNetworkingConfigSpecV2_ProxyPeering{}
		default:
			require.FailNow(t, "unexpected type: %T", tc.strategy)
		}
		err = json.Unmarshal([]byte(tc.json), actual)
		require.NoError(t, err)
		require.Equal(t, tc.strategy, actual)

		err = yaml.Unmarshal([]byte(tc.yaml), actual)
		require.NoError(t, err)
		require.Equal(t, tc.strategy, actual)
	}
}
