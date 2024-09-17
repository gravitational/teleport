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
	autoUpdateConfigPrefix  = "auto_update_config"
	autoUpdateVersionPrefix = "auto_update_version"
)

// AutoUpdateService is responsible for managing auto update configuration and version.
type AutoUpdateService struct {
	config  *generic.ServiceWrapper[*autoupdate.AutoUpdateConfig]
	version *generic.ServiceWrapper[*autoupdate.AutoUpdateVersion]
}

// NewAutoUpdateService returns a new AutoUpdateService.
func NewAutoUpdateService(backend backend.Backend) (*AutoUpdateService, error) {
	config, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*autoupdate.AutoUpdateConfig]{
			Backend:       backend,
			ResourceKind:  types.KindAutoUpdateConfig,
			BackendPrefix: autoUpdateConfigPrefix,
			MarshalFunc:   services.MarshalProtoResource[*autoupdate.AutoUpdateConfig],
			UnmarshalFunc: services.UnmarshalProtoResource[*autoupdate.AutoUpdateConfig],
			ValidateFunc:  update.ValidateAutoUpdateConfig,
			KeyFunc: func(*autoupdate.AutoUpdateConfig) string {
				return types.MetaNameAutoUpdateConfig
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*autoupdate.AutoUpdateVersion]{
			Backend:       backend,
			ResourceKind:  types.KindAutoUpdateVersion,
			BackendPrefix: autoUpdateVersionPrefix,
			MarshalFunc:   services.MarshalProtoResource[*autoupdate.AutoUpdateVersion],
			UnmarshalFunc: services.UnmarshalProtoResource[*autoupdate.AutoUpdateVersion],
			ValidateFunc:  update.ValidateAutoUpdateVersion,
			KeyFunc: func(version *autoupdate.AutoUpdateVersion) string {
				return types.MetaNameAutoUpdateVersion
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AutoUpdateService{
		config:  config,
		version: version,
	}, nil
}

// CreateAutoUpdateConfig creates an auto update configuration singleton.
func (s *AutoUpdateService) CreateAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.CreateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpdateAutoUpdateConfig updates an auto update configuration singleton.
func (s *AutoUpdateService) UpdateAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.UpdateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpsertAutoUpdateConfig sets an auto update configuration.
func (s *AutoUpdateService) UpsertAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.UpsertResource(ctx, c)
	return config, trace.Wrap(err)
}

// GetAutoUpdateConfig gets the auto update configuration from the backend.
func (s *AutoUpdateService) GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.GetResource(ctx, types.MetaNameAutoUpdateConfig)
	return config, trace.Wrap(err)
}

// DeleteAutoUpdateConfig deletes the auto update configuration from the backend.
func (s *AutoUpdateService) DeleteAutoUpdateConfig(ctx context.Context) error {
	return trace.Wrap(s.config.DeleteResource(ctx, types.MetaNameAutoUpdateConfig))
}

// CreateAutoUpdateVersion creates an autoupdate version resource.
func (s *AutoUpdateService) CreateAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.CreateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpdateAutoUpdateVersion updates an autoupdate version resource.
func (s *AutoUpdateService) UpdateAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.UpdateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpsertAutoUpdateVersion sets autoupdate version resource.
func (s *AutoUpdateService) UpsertAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.UpsertResource(ctx, v)
	return version, trace.Wrap(err)
}

// GetAutoUpdateVersion gets the auto update version from the backend.
func (s *AutoUpdateService) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.GetResource(ctx, types.MetaNameAutoUpdateVersion)
	return version, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes the auto update version from the backend.
func (s *AutoUpdateService) DeleteAutoUpdateVersion(ctx context.Context) error {
	return trace.Wrap(s.version.DeleteResource(ctx, types.MetaNameAutoUpdateVersion))
}
