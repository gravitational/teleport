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
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// NewClusterNameWithRandomID creates a ClusterName, supplying a random
// ClusterID if the field is not provided in spec.
func NewClusterNameWithRandomID(spec types.ClusterNameSpecV2) (types.ClusterName, error) {
	if spec.ClusterID == "" {
		spec.ClusterID = uuid.New().String()
	}
	return types.NewClusterName(spec)
}

// UnmarshalClusterName unmarshals the ClusterName resource from JSON.
func UnmarshalClusterName(bytes []byte, opts ...MarshalOption) (types.ClusterName, error) {
	var clusterName types.ClusterNameV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &clusterName); err != nil {
		return nil, trace.BadParameter("%s", err)
	}

	err = clusterName.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		clusterName.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		clusterName.SetExpiry(cfg.Expires)
	}

	return &clusterName, nil
}

// MarshalClusterName marshals the ClusterName resource to JSON.
func MarshalClusterName(clusterName types.ClusterName, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch clusterName := clusterName.(type) {
	case *types.ClusterNameV2:
		if err := clusterName.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, clusterName))
	default:
		return nil, trace.BadParameter("unrecognized cluster name version %T", clusterName)
	}
}
