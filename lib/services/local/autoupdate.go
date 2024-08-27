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
	autoupdateConfigPrefix  = "autoupdate_config"
	autoupdateVersionPrefix = "autoupdate_version"
)

// AutoupdateService is responsible for managing autoupdate configuration and version management.
type AutoupdateService struct {
	config  *generic.ServiceWrapper[*autoupdate.AutoupdateConfig]
	version *generic.ServiceWrapper[*autoupdate.AutoupdateVersion]
}

// NewAutoupdateService returns a new AutoupdateService.
func NewAutoupdateService(backend backend.Backend) (*AutoupdateService, error) {
	config, err := generic.NewServiceWrapper(
		backend,
		types.KindAutoupdateConfig,
		autoupdateConfigPrefix,
		services.MarshalProtoResource[*autoupdate.AutoupdateConfig],
		services.UnmarshalProtoResource[*autoupdate.AutoupdateConfig],
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := generic.NewServiceWrapper(
		backend,
		types.KindAutoupdateVersion,
		autoupdateVersionPrefix,
		services.MarshalProtoResource[*autoupdate.AutoupdateVersion],
		services.UnmarshalProtoResource[*autoupdate.AutoupdateVersion],
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AutoupdateService{
		config:  config,
		version: version,
	}, nil
}

// CreateAutoupdateConfig creates autoupdate configuration singleton.
func (s *AutoupdateService) CreateAutoupdateConfig(
	ctx context.Context,
	c *autoupdate.AutoupdateConfig,
) (*autoupdate.AutoupdateConfig, error) {
	if err := update.ValidateAutoupdateConfig(c); err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := s.config.CreateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpdateAutoupdateConfig update autoupdate configuration singleton.
func (s *AutoupdateService) UpdateAutoupdateConfig(
	ctx context.Context,
	c *autoupdate.AutoupdateConfig,
) (*autoupdate.AutoupdateConfig, error) {
	if err := update.ValidateAutoupdateConfig(c); err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := s.config.UpdateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpsertAutoupdateConfig sets autoupdate configuration.
func (s *AutoupdateService) UpsertAutoupdateConfig(
	ctx context.Context,
	c *autoupdate.AutoupdateConfig,
) (*autoupdate.AutoupdateConfig, error) {
	if err := update.ValidateAutoupdateConfig(c); err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := s.config.UpsertResource(ctx, c)
	return config, trace.Wrap(err)
}

// GetAutoupdateConfig gets the autoupdate configuration from the backend.
func (s *AutoupdateService) GetAutoupdateConfig(ctx context.Context) (*autoupdate.AutoupdateConfig, error) {
	config, err := s.config.GetResource(ctx, types.MetaNameAutoupdateConfig)
	return config, trace.Wrap(err)
}

// DeleteAutoupdateConfig deletes types.AutoupdateConfig from the backend.
func (s *AutoupdateService) DeleteAutoupdateConfig(ctx context.Context) error {
	return trace.Wrap(s.config.DeleteResource(ctx, types.MetaNameAutoupdateConfig))
}

// CreateAutoupdateVersion creates autoupdate version resource.
func (s *AutoupdateService) CreateAutoupdateVersion(
	ctx context.Context,
	v *autoupdate.AutoupdateVersion,
) (*autoupdate.AutoupdateVersion, error) {
	if err := update.ValidateAutoupdateVersion(v); err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := s.version.CreateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpdateAutoupdateVersion updates autoupdate version resource.
func (s *AutoupdateService) UpdateAutoupdateVersion(
	ctx context.Context,
	v *autoupdate.AutoupdateVersion,
) (*autoupdate.AutoupdateVersion, error) {
	if err := update.ValidateAutoupdateVersion(v); err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := s.version.UpdateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpsertAutoupdateVersion sets autoupdate version resource.
func (s *AutoupdateService) UpsertAutoupdateVersion(
	ctx context.Context,
	v *autoupdate.AutoupdateVersion,
) (*autoupdate.AutoupdateVersion, error) {
	if err := update.ValidateAutoupdateVersion(v); err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := s.version.UpsertResource(ctx, v)
	return version, trace.Wrap(err)
}

// GetAutoupdateVersion gets the autoupdate version from the backend.
func (s *AutoupdateService) GetAutoupdateVersion(ctx context.Context) (*autoupdate.AutoupdateVersion, error) {
	version, err := s.version.GetResource(ctx, types.MetaNameAutoupdateVersion)
	return version, trace.Wrap(err)
}

// DeleteAutoupdateVersion deletes types.AutoupdateVersion from the backend.
func (s *AutoupdateService) DeleteAutoupdateVersion(ctx context.Context) error {
	return trace.Wrap(s.version.DeleteResource(ctx, types.MetaNameAutoupdateVersion))
}
