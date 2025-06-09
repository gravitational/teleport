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
	"bytes"
	"encoding/json"

	"github.com/goccy/go-yaml"
	apiyaml "github.com/gravitational/teleport/api/utils/yaml"
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

// MarshalYAML converts a TunnelStrategyV1 to yaml.
func (s *TunnelStrategyV1) MarshalYAML() ([]byte, error) {
	jsonData, err := s.MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	yamlData, err := yaml.JSONToYAML(jsonData)
	return yamlData, trace.Wrap(err)
}

// UnmarshalYAML converts yaml to a TunnelStrategyV1 using a strict policy to
// disallow unknown fields.
func (s *TunnelStrategyV1) UnmarshalYAML(data []byte) error {
	return trace.Wrap(s.unmarshal(data, true))
}

// MarshalJSON converts a TunnelStrategyV1 to json.
func (s *TunnelStrategyV1) MarshalJSON() ([]byte, error) {
	var config any
	switch strategy := s.Strategy.(type) {
	case *TunnelStrategyV1_AgentMesh:
		config = struct {
			Type                     TunnelStrategyType `json:"type"`
			*AgentMeshTunnelStrategy `json:",inline"`
		}{Type: AgentMesh, AgentMeshTunnelStrategy: strategy.AgentMesh}
	case *TunnelStrategyV1_ProxyPeering:
		config = struct {
			Type                        TunnelStrategyType `json:"type"`
			*ProxyPeeringTunnelStrategy `json:",inline"`
		}{Type: ProxyPeering, ProxyPeeringTunnelStrategy: strategy.ProxyPeering}
	default:
		return nil, trace.BadParameter("unknown tunnel strategy: \"%T\"", strategy)
	}
	data, err := json.Marshal(config)
	return data, trace.Wrap(err)
}

// UnmarshalJSON converts json to a TunnelStrategyV1. Unknown fields are allowed
// to prevent rollbacks causing issues decoding this data from the backend.
func (s *TunnelStrategyV1) UnmarshalJSON(data []byte) error {
	return trace.Wrap(s.unmarshal(data, false))
}

func (s *TunnelStrategyV1) unmarshal(data []byte, strict bool) error {
	configType := struct {
		Type TunnelStrategyType `json:"type"`
	}{}
	if err := apiyaml.Unmarshal(data, &configType); err != nil {
		return trace.Wrap(err)
	}
	var opts []yaml.DecodeOption
	if strict {
		opts = append(opts, yaml.Strict())
	}
	decoder := apiyaml.NewDecoder(bytes.NewReader(data), opts...)

	switch configType.Type {
	case AgentMesh:
		config := struct {
			Type                     TunnelStrategyType `json:"type"`
			*AgentMeshTunnelStrategy `json:",inline"`
		}{}
		if err := decoder.Decode(&config); err != nil {
			return trace.Wrap(err)
		}
		s.Strategy = &TunnelStrategyV1_AgentMesh{
			AgentMesh: config.AgentMeshTunnelStrategy,
		}
	case ProxyPeering:
		config := struct {
			Type                        TunnelStrategyType `json:"type"`
			*ProxyPeeringTunnelStrategy `json:",inline"`
		}{}
		if err := decoder.Decode(&config); err != nil {
			return trace.Wrap(err)
		}
		s.Strategy = &TunnelStrategyV1_ProxyPeering{
			ProxyPeering: config.ProxyPeeringTunnelStrategy,
		}
	default:
		return trace.BadParameter("unknown tunnel strategy: \"%s\"", configType.Type)
	}

	return nil
}

// CheckAndSetDefaults validates and sets default values for a tunnel strategy.
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
