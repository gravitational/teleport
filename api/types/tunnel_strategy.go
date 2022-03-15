package types

import (
	"bytes"
	"encoding/json"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

const (
	defaultAgentConnectionCount = 1
)

// DefaultAgentMeshTunnelStrategy sets default values for a agent mesh
// tunnel strategy.
func DefaultAgentMeshTunnelStrategy() *AgentMeshTunnelStrategy {
	return &AgentMeshTunnelStrategy{Type: AgentMesh}
}

// DefaultProxyPeeringTunnelStrategy sets default values for a proxy peering
// tunnel strategy.
func DefaultProxyPeeringTunnelStrategy() *ProxyPeeringTunnelStrategy {
	return &ProxyPeeringTunnelStrategy{
		Type:                 ProxyPeering,
		AgentConnectionCount: defaultAgentConnectionCount,
	}
}

// DefaultTunnelStrategy is the default tunnel strategy used when one is not
// specified.
func DefaultTunnelStrategy() TunnelStrategy {
	return &ClusterNetworkingConfigSpecV2_AgentMesh{
		AgentMesh: DefaultAgentMeshTunnelStrategy(),
	}
}

// TunnelStrategy defines methods to be implemented by any TunnelStrategy.
type TunnelStrategy interface {
	isClusterNetworkingConfigSpecV2_TunnelStrategy
	CheckAndSetDefaults() error
}

// TunnelStrategyConfig represents a unparsed tunnel strategy configuration.
type TunnelStrategyConfig struct {
	Type   TunnelStrategyType     `yaml:"type" json:"type"`
	Params map[string]interface{} `yaml:",inline" json:"-"`
}

// Parse parses a TunnelStrategyConfig into a TunnelStrategy.
// The default tunnel strategy is used if the TunnelStrategyConfig is empty.
func (c *TunnelStrategyConfig) Parse() (TunnelStrategy, error) {
	var strategy TunnelStrategy

	if c.Type == "" && len(c.Params) == 0 {
		return DefaultTunnelStrategy(), nil
	}

	bytes, err := json.Marshal(c.Params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal %v", c)
	}

	switch c.Type {
	case AgentMesh:
		agentMesh := &ClusterNetworkingConfigSpecV2_AgentMesh{
			AgentMesh: DefaultAgentMeshTunnelStrategy(),
		}

		if err = UnmarshalStrict(bytes, agentMesh.AgentMesh); err != nil {
			return nil, trace.Wrap(err)
		}
		strategy = agentMesh
	case ProxyPeering:
		proxyPeering := &ClusterNetworkingConfigSpecV2_ProxyPeering{
			ProxyPeering: DefaultProxyPeeringTunnelStrategy(),
		}
		if err = UnmarshalStrict(bytes, proxyPeering.ProxyPeering); err != nil {
			return nil, trace.Wrap(err)
		}
		strategy = proxyPeering
	default:
		return nil, trace.BadParameter("unknown tunnel strategy: \"%s\"", c.Type)
	}

	err = strategy.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return strategy, nil
}

// CheckAndSetDefaults validates an agent mesh tunnel strategy.
func (s *ClusterNetworkingConfigSpecV2_AgentMesh) CheckAndSetDefaults() error {
	if s.AgentMesh == nil {
		s.AgentMesh = DefaultAgentMeshTunnelStrategy()
	}
	if s.AgentMesh.Type != AgentMesh {
		return trace.BadParameter("tunnel strategy must be %s: got %s", AgentMesh, s.AgentMesh.Type)
	}

	return nil
}

// MarshalYAML defines how a proxy listener mode should be marshalled to a string
func (s *AgentMeshTunnelStrategy) MarshalYAML() (interface{}, error) {
	params, err := tunnelStrategyMarshalYAML(s)
	return params, trace.Wrap(err)
}

// CheckAndSetDefaults validates a proxy peering tunnel strategy.
func (s *ClusterNetworkingConfigSpecV2_ProxyPeering) CheckAndSetDefaults() error {
	if s.ProxyPeering == nil {
		s.ProxyPeering = DefaultProxyPeeringTunnelStrategy()
	}

	if s.ProxyPeering.Type != ProxyPeering {
		return trace.BadParameter("tunnel strategy must be %s: got %s", ProxyPeering, s.ProxyPeering.Type)
	}
	if s.ProxyPeering.AgentConnectionCount == 0 {
		s.ProxyPeering.AgentConnectionCount = defaultAgentConnectionCount
	}

	return nil
}

// MarshalYAML defines how a proxy listener mode should be marshalled to a string
func (s *ProxyPeeringTunnelStrategy) MarshalYAML() (interface{}, error) {
	params, err := tunnelStrategyMarshalYAML(s)
	return params, trace.Wrap(err)
}

// tunnelStratgeyMarshalYAML returns a map ready for YAML marshalling
// removing XXX_ fields from protobuf structs.
func tunnelStrategyMarshalYAML(strategy interface{}) (map[string]interface{}, error) {
	params := make(map[string]interface{})

	// We must use json conversion first to remove xxx_ fields.
	err := utils.ObjectToStruct(strategy, &params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return params, nil
}

// UnmarshalStrict is a strict json unmarshaller.
func UnmarshalStrict(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return trace.Wrap(dec.Decode(v))
}
