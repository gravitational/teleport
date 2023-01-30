/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalDiscoveredServer marshals the DiscoveredServer resource to JSON.
func MarshalDiscoveredServer(discoveredServer types.DiscoveredServer, opts ...MarshalOption) ([]byte, error) {
	if err := discoveredServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch discoveredServer := discoveredServer.(type) {
	case *types.DiscoveredServerV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *discoveredServer
			copy.SetResourceID(0)
			discoveredServer = &copy
		}
		return utils.FastMarshal(discoveredServer)
	default:
		return nil, trace.BadParameter("unrecognized DiscoveredServer version %T", discoveredServer)
	}
}

// UnmarshalDiscoveredServer unmarshals the DiscoveredServer resource from JSON.
func UnmarshalDiscoveredServer(data []byte, opts ...MarshalOption) (types.DiscoveredServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DiscoveredServer data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var s types.DiscoveredServerV1
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("discovered server resource version %q is not supported", h.Version)
}
