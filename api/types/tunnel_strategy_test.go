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
