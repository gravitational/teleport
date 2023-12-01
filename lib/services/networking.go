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
			copy.SetRevision("")
			netConfig = &copy
		}
		return utils.FastMarshal(netConfig)
	default:
		return nil, trace.BadParameter("unrecognized cluster networking config version %T", netConfig)
	}
}
