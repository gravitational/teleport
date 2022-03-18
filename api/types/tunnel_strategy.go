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

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

const (
	tunnelStrategyTypeParam     = "type"
	defaultAgentConnectionCount = 1
)

// DefaultAgentMeshTunnelStrategy sets default values for a agent mesh
// tunnel strategy.
func DefaultAgentMeshTunnelStrategy() *AgentMeshTunnelStrategy {
	return &AgentMeshTunnelStrategy{}
}

// DefaultProxyPeeringTunnelStrategy sets default values for a proxy peering
// tunnel strategy.
func DefaultProxyPeeringTunnelStrategy() *ProxyPeeringTunnelStrategy {
	return &ProxyPeeringTunnelStrategy{
		AgentConnectionCount: defaultAgentConnectionCount,
	}
}

// DefaultTunnelStrategy is the default tunnel strategy used when one is not
// specified.
func DefaultTunnelStrategy() TunnelStrategy {
	return &TunnelStrategyV1_AgentMesh{
		AgentMesh: DefaultAgentMeshTunnelStrategy(),
	}
}

// TunnelStrategy defines methods to be implemented by any TunnelStrategy.
type TunnelStrategy interface {
	isTunnelStrategyV1_Strategy
	CheckAndSetDefaults() error
}

// tunnelStrategyConfig represents a unparsed tunnel strategy configuration.
type tunnelStrategyConfig struct {
	Type   TunnelStrategyType     `yaml:"type" json:"type"`
	Params map[string]interface{} `yaml:",inline" json:"-"`
}

// newTunnelStrategyConfig creates a new tunnelStrategyConfig instance.
func newTunnelStrategyConfig() *tunnelStrategyConfig {
	return &tunnelStrategyConfig{
		Params: make(map[string]interface{}),
	}
}

// setFromMap sets a TunnelStrategyConfig from a map.
func (c *tunnelStrategyConfig) setFromMap(m map[string]interface{}) error {
	rawStrategy, ok := m[tunnelStrategyTypeParam]
	if !ok {
		return trace.BadParameter("missing type parameter")
	}
	strategyType, ok := rawStrategy.(string)
	if !ok {
		return trace.BadParameter("invalid type parameter")
	}
	c.Type = TunnelStrategyType(strategyType)

	for k, v := range m {
		if k == tunnelStrategyTypeParam {
			continue
		}
		c.Params[k] = v
	}
	return nil
}

// getMapCopy returns a TunnelStrategyConfig as a map.
func (c *tunnelStrategyConfig) getMapCopy() map[string]interface{} {
	mCopy := make(map[string]interface{})
	for k, v := range c.Params {
		mCopy[k] = v
	}
	mCopy[tunnelStrategyTypeParam] = c.Type
	return mCopy
}

// MarshalYAML converts a TunnelStrategyV1 to yaml.
func (s *TunnelStrategyV1) MarshalYAML() (interface{}, error) {
	var config *tunnelStrategyConfig
	err := s.marshal(func(c *tunnelStrategyConfig) error {
		config = c
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config.getMapCopy(), nil
}

// UnmarshalYAML converts yaml to a TunnelStrategyV1 using a strict policy to
// disallow unknown fields.
func (s *TunnelStrategyV1) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := s.unmarshal(utils.StrictObjectToStruct, func(c *tunnelStrategyConfig) error {
		return trace.Wrap(unmarshal(c))
	})
	return trace.Wrap(err)
}

// MarshalJSON converts a TunnelStrategyV1 to json.
func (s *TunnelStrategyV1) MarshalJSON() ([]byte, error) {
	var data []byte
	err := s.marshal(func(c *tunnelStrategyConfig) error {
		var err error
		data, err = json.Marshal(c.getMapCopy())
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalJSON converts json to a TunnelStrategyV1. Unknown fields are allowed
// to prevent rollbacks causing issues decoding this data from the backend.
func (s *TunnelStrategyV1) UnmarshalJSON(data []byte) error {
	err := s.unmarshal(utils.ObjectToStruct, func(c *tunnelStrategyConfig) error {
		params := make(map[string]interface{})
		err := json.Unmarshal(data, &params)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(c.setFromMap(params))
	})
	return trace.Wrap(err)
}

// marshal converts a TunnelStrategyV1 to a TunnelStrategyConfig before calling
// the given marshal function.
func (s *TunnelStrategyV1) marshal(marshal func(*tunnelStrategyConfig) error) error {
	config := newTunnelStrategyConfig()
	switch strategy := s.Strategy.(type) {
	case *TunnelStrategyV1_AgentMesh:
		config.Type = AgentMesh
		err := utils.ObjectToStruct(strategy.AgentMesh, &config.Params)
		if err != nil {
			return trace.Wrap(err)
		}
	case *TunnelStrategyV1_ProxyPeering:
		config.Type = ProxyPeering
		err := utils.ObjectToStruct(strategy.ProxyPeering, &config.Params)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown tunnel strategy: \"%s\"", config.Type)
	}

	return trace.Wrap(marshal(config))
}

// objectToStructFunc is a function that converts one struct to another.
type objectToStructFunc func(interface{}, interface{}) error

func (s *TunnelStrategyV1) unmarshal(ots objectToStructFunc, unmarshal func(*tunnelStrategyConfig) error) error {
	config := newTunnelStrategyConfig()
	err := unmarshal(config)
	if err != nil {
		return trace.Wrap(err)
	}

	switch config.Type {
	case AgentMesh:
		strategy := &TunnelStrategyV1_AgentMesh{
			AgentMesh: &AgentMeshTunnelStrategy{},
		}

		err = ots(&config.Params, strategy.AgentMesh)
		if err != nil {
			return trace.Wrap(err)
		}
		s.Strategy = strategy
	case ProxyPeering:
		strategy := &TunnelStrategyV1_ProxyPeering{
			ProxyPeering: &ProxyPeeringTunnelStrategy{},
		}

		err = ots(&config.Params, strategy.ProxyPeering)
		if err != nil {
			return trace.Wrap(err)
		}
		s.Strategy = strategy
	default:
		return trace.BadParameter("unknown tunnel strategy: \"%s\"", config.Type)
	}

	return nil
}

func (s *TunnelStrategyV1) CheckAndSetDefaults() error {
	if s.Strategy == nil {
		s.Strategy = DefaultTunnelStrategy()
	}

	switch strategy := s.Strategy.(type) {
	case TunnelStrategy:
		err := strategy.CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown tunnel strategy: %T", strategy)
	}

	return nil
}

// CheckAndSetDefaults validates an agent mesh tunnel strategy.
func (s *TunnelStrategyV1_AgentMesh) CheckAndSetDefaults() error {
	if s.AgentMesh == nil {
		s.AgentMesh = DefaultAgentMeshTunnelStrategy()
	}

	return nil
}

// CheckAndSetDefaults validates a proxy peering tunnel strategy.
func (s *TunnelStrategyV1_ProxyPeering) CheckAndSetDefaults() error {
	if s.ProxyPeering == nil {
		s.ProxyPeering = DefaultProxyPeeringTunnelStrategy()
	}
	if s.ProxyPeering.AgentConnectionCount == 0 {
		s.ProxyPeering.AgentConnectionCount = defaultAgentConnectionCount
	}

	return nil
}
