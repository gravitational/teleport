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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	autoUpdateConfigPrefix  = "autoupdate_config"
	autoUpdateVersionPrefix = "autoupdate_version"
)

// ClusterAutoUpdateService is responsible for managing autoupdate configuration and version management.
type ClusterAutoUpdateService struct {
	config  *generic.ServiceWrapper[*autoupdate.ClusterAutoUpdateConfig]
	version *generic.ServiceWrapper[*autoupdate.AutoUpdateVersion]
}

// NewClusterAutoUpdateService returns a new AutoUpdateService.
func NewClusterAutoUpdateService(backend backend.Backend) (*ClusterAutoUpdateService, error) {
	config, err := generic.NewServiceWrapper(
		backend,
		types.KindClusterAutoUpdateConfig,
		autoUpdateConfigPrefix,
		services.MarshalProtoResource[*autoupdate.ClusterAutoUpdateConfig],
		services.UnmarshalProtoResource[*autoupdate.ClusterAutoUpdateConfig],
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := generic.NewServiceWrapper(
		backend,
		types.KindAutoUpdateVersion,
		autoUpdateVersionPrefix,
		services.MarshalProtoResource[*autoupdate.AutoUpdateVersion],
		services.UnmarshalProtoResource[*autoupdate.AutoUpdateVersion],
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClusterAutoUpdateService{
		config:  config,
		version: version,
	}, nil
}

// UpsertClusterAutoUpdateConfig sets cluster autoupdate configuration.
func (s *ClusterAutoUpdateService) UpsertClusterAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.ClusterAutoUpdateConfig,
) (*autoupdate.ClusterAutoUpdateConfig, error) {
	if err := update.ValidateClusterAutoUpdateConfig(c); err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := s.config.UpsertResource(ctx, c)
	return config, trace.Wrap(err)
}

// GetClusterAutoUpdateConfig gets the autoupdate configuration from the backend.
func (s *ClusterAutoUpdateService) GetClusterAutoUpdateConfig(ctx context.Context) (*autoupdate.ClusterAutoUpdateConfig, error) {
	config, err := s.config.GetResource(ctx, types.MetaNameClusterAutoUpdateConfig)
	return config, trace.Wrap(err)
}

// DeleteClusterAutoUpdateConfig deletes types.ClusterAutoUpdateConfig from the backend.
func (s *ClusterAutoUpdateService) DeleteClusterAutoUpdateConfig(ctx context.Context) error {
	return trace.Wrap(s.config.DeleteResource(ctx, types.MetaNameClusterAutoUpdateConfig))
}

// UpsertAutoUpdateVersion sets cluster autoupdate version resource.
func (s *ClusterAutoUpdateService) UpsertAutoUpdateVersion(ctx context.Context, v *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error) {
	if err := update.ValidateAutoUpdateVersion(v); err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := s.version.UpsertResource(ctx, v)
	return version, trace.Wrap(err)
}

// GetAutoUpdateVersion gets the autoupdate version from the backend.
func (s *ClusterAutoUpdateService) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.GetResource(ctx, types.MetaNameAutoUpdateVersion)
	return version, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes types.AutoUpdateVersion from the backend.
func (s *ClusterAutoUpdateService) DeleteAutoUpdateVersion(ctx context.Context) error {
	return trace.Wrap(s.config.DeleteResource(ctx, types.MetaNameAutoUpdateVersion))
}
