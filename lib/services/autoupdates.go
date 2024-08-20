/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ClusterAutoUpdate stores the cluster autoupdate configuration in the backend.
type ClusterAutoUpdate interface {
	// UpsertClusterAutoUpdateConfig sets cluster autoupdate configuration.
	UpsertClusterAutoUpdateConfig(ctx context.Context, c types.ClusterAutoUpdateConfig) error
	// GetClusterAutoUpdateConfig gets the autoupdate configuration from the backend.
	GetClusterAutoUpdateConfig(ctx context.Context, opts ...MarshalOption) (types.ClusterAutoUpdateConfig, error)
	// DeleteClusterAutoUpdateConfig deletes types.ClusterAutoUpdateConfig from the backend.
	DeleteClusterAutoUpdateConfig(ctx context.Context) error

	// UpsertAutoUpdateVersion sets autoupdate version.
	UpsertAutoUpdateVersion(ctx context.Context, c types.AutoUpdateVersion) error
	// GetAutoUpdateVersion gets the autoupdate version from the backend.
	GetAutoUpdateVersion(ctx context.Context, opts ...MarshalOption) (types.AutoUpdateVersion, error)
	// DeleteAutoUpdateVersion deletes types.AutoUpdateVersion from the backend.
	DeleteAutoUpdateVersion(ctx context.Context) error
}

// NewClusterAutoUpdateConfig creates a ClusterAutoUpdateConfig with default values.
func NewClusterAutoUpdateConfig(spec types.ClusterAutoUpdateConfigSpecV1) (types.ClusterAutoUpdateConfig, error) {
	return types.NewClusterAutoUpdateConfig(spec)
}

// NewAutoUpdateVersion creates a AutoUpdateVersion with default values.
func NewAutoUpdateVersion(spec types.AutoupdateVersionSpecV1) (types.AutoUpdateVersion, error) {
	return types.NewAutoUpdateVersion(spec)
}

// UnmarshalClusterAutoUpdateConfig unmarshals the ClusterAutoUpdateConfig resource from JSON.
func UnmarshalClusterAutoUpdateConfig(bytes []byte, opts ...MarshalOption) (types.ClusterAutoUpdateConfig, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	var clusterAutoUpdateConfig types.ClusterAutoUpdateConfigV1
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &clusterAutoUpdateConfig); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = clusterAutoUpdateConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		clusterAutoUpdateConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		clusterAutoUpdateConfig.SetExpiry(cfg.Expires)
	}

	return &clusterAutoUpdateConfig, nil
}

// UnmarshalAutoUpdateVersion unmarshals the AutoUpdateVersion resource from JSON.
func UnmarshalAutoUpdateVersion(bytes []byte, opts ...MarshalOption) (types.AutoUpdateVersion, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	var autoUpdateVersion types.AutoUpdateVersionV1
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &autoUpdateVersion); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = autoUpdateVersion.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		autoUpdateVersion.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		autoUpdateVersion.SetExpiry(cfg.Expires)
	}

	return &autoUpdateVersion, nil
}

// MarshalClusterAutoUpdateConfig marshals the ClusterAutoUpdateConfig resource to JSON.
func MarshalClusterAutoUpdateConfig(config types.ClusterAutoUpdateConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch autoUpdateConfig := config.(type) {
	case *types.ClusterAutoUpdateConfigV1:
		if err := autoUpdateConfig.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, autoUpdateConfig))
	default:
		return nil, trace.BadParameter("unrecognized cluster autoupdate config version %T", autoUpdateConfig)
	}
}

// MarshalAutoUpdateVersion marshals the AutoUpdateVersion resource to JSON.
func MarshalAutoUpdateVersion(config types.AutoUpdateVersion, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch autoUpdateVersion := config.(type) {
	case *types.AutoUpdateVersionV1:
		if err := autoUpdateVersion.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, autoUpdateVersion))
	default:
		return nil, trace.BadParameter("unrecognized autoupdate version %T", autoUpdateVersion)
	}
}
