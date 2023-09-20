/*
Copyright 2021 Gravitational, Inc.

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

// UnmarshalClusterNetworkingConfig unmarshals the ClusterNetworkingConfig resource from JSON.
func UnmarshalClusterNetworkingConfig(bytes []byte, opts ...MarshalOption) (types.ClusterNetworkingConfig, error) {
	var netConfig types.ClusterNetworkingConfigV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &netConfig); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = netConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		netConfig.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		netConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		netConfig.SetExpiry(cfg.Expires)
	}
	return &netConfig, nil
}

// MarshalClusterNetworkingConfig marshals the ClusterNetworkingConfig resource to JSON.
func MarshalClusterNetworkingConfig(netConfig types.ClusterNetworkingConfig, opts ...MarshalOption) ([]byte, error) {
	if err := netConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch netConfig := netConfig.(type) {
	case *types.ClusterNetworkingConfigV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *netConfig
			copy.SetResourceID(0)
			netConfig = &copy
		}
		return utils.FastMarshal(netConfig)
	default:
		return nil, trace.BadParameter("unrecognized cluster networking config version %T", netConfig)
	}
}
