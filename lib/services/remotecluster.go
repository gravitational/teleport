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

// UnmarshalRemoteCluster unmarshals the RemoteCluster resource from JSON.
func UnmarshalRemoteCluster(bytes []byte, opts ...MarshalOption) (types.RemoteCluster, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cluster types.RemoteClusterV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if err := utils.FastUnmarshal(bytes, &cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	err = cluster.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		cluster.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		cluster.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		cluster.SetExpiry(cfg.Expires)
	}

	return &cluster, nil
}

// MarshalRemoteCluster marshals the RemoteCluster resource to JSON.
func MarshalRemoteCluster(remoteCluster types.RemoteCluster, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch remoteCluster := remoteCluster.(type) {
	case *types.RemoteClusterV3:
		if err := remoteCluster.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, remoteCluster))
	default:
		return nil, trace.BadParameter("unrecognized remote cluster version %T", remoteCluster)
	}
}
