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
	autoUpdateConfigPrefix    = "auto_update_config"
	autoUpdateVersionPrefix   = "auto_update_version"
	autoUpdateAgentPlanPrefix = "auto_update_agent_plan"
)

// AutoUpdateService is responsible for managing AutoUpdateConfig and AutoUpdateVersion singleton resources.
type AutoUpdateService struct {
	config  *generic.ServiceWrapper[*autoupdate.AutoUpdateConfig]
	version *generic.ServiceWrapper[*autoupdate.AutoUpdateVersion]
	plan    *generic.ServiceWrapper[*autoupdate.AutoUpdateAgentPlan]
}

// NewAutoUpdateService returns a new AutoUpdateService.
func NewAutoUpdateService(b backend.Backend) (*AutoUpdateService, error) {
	config, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*autoupdate.AutoUpdateConfig]{
			Backend:       b,
			ResourceKind:  types.KindAutoUpdateConfig,
			BackendPrefix: backend.NewKey(autoUpdateConfigPrefix),
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
			Backend:       b,
			ResourceKind:  types.KindAutoUpdateVersion,
			BackendPrefix: backend.NewKey(autoUpdateVersionPrefix),
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
	plan, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*autoupdate.AutoUpdateAgentPlan]{
			Backend:       b,
			ResourceKind:  types.KindAutoUpdateAgentPlan,
			BackendPrefix: backend.NewKey(autoUpdateAgentPlanPrefix),
			MarshalFunc:   services.MarshalProtoResource[*autoupdate.AutoUpdateAgentPlan],
			UnmarshalFunc: services.UnmarshalProtoResource[*autoupdate.AutoUpdateAgentPlan],
			ValidateFunc:  update.ValidateAutoUpdateAgentPlan,
			KeyFunc: func(version *autoupdate.AutoUpdateAgentPlan) string {
				return types.MetaNameAutoUpdateAgentPlan
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AutoUpdateService{
		config:  config,
		version: version,
		plan:    plan,
	}, nil
}

// CreateAutoUpdateConfig creates the AutoUpdateConfig singleton resource.
func (s *AutoUpdateService) CreateAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.CreateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpdateAutoUpdateConfig updates the AutoUpdateConfig singleton resource.
func (s *AutoUpdateService) UpdateAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.UpdateResource(ctx, c)
	return config, trace.Wrap(err)
}

// UpsertAutoUpdateConfig sets the AutoUpdateConfig singleton resource.
func (s *AutoUpdateService) UpsertAutoUpdateConfig(
	ctx context.Context,
	c *autoupdate.AutoUpdateConfig,
) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.UpsertResource(ctx, c)
	return config, trace.Wrap(err)
}

// GetAutoUpdateConfig gets the AutoUpdateConfig singleton resource.
func (s *AutoUpdateService) GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error) {
	config, err := s.config.GetResource(ctx, types.MetaNameAutoUpdateConfig)
	return config, trace.Wrap(err)
}

// DeleteAutoUpdateConfig deletes the AutoUpdateConfig singleton resource.
func (s *AutoUpdateService) DeleteAutoUpdateConfig(ctx context.Context) error {
	return trace.Wrap(s.config.DeleteResource(ctx, types.MetaNameAutoUpdateConfig))
}

// CreateAutoUpdateVersion creates the AutoUpdateVersion singleton resource.
func (s *AutoUpdateService) CreateAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.CreateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpdateAutoUpdateVersion updates the AutoUpdateVersion singleton resource.
func (s *AutoUpdateService) UpdateAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.UpdateResource(ctx, v)
	return version, trace.Wrap(err)
}

// UpsertAutoUpdateVersion sets the AutoUpdateVersion singleton resource.
func (s *AutoUpdateService) UpsertAutoUpdateVersion(
	ctx context.Context,
	v *autoupdate.AutoUpdateVersion,
) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.UpsertResource(ctx, v)
	return version, trace.Wrap(err)
}

// GetAutoUpdateVersion gets the AutoUpdateVersion singleton resource.
func (s *AutoUpdateService) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	version, err := s.version.GetResource(ctx, types.MetaNameAutoUpdateVersion)
	return version, trace.Wrap(err)
}

// DeleteAutoUpdateVersion deletes the AutoUpdateVersion singleton resource.
func (s *AutoUpdateService) DeleteAutoUpdateVersion(ctx context.Context) error {
	return trace.Wrap(s.version.DeleteResource(ctx, types.MetaNameAutoUpdateVersion))
}

// CreateAutoUpdateAgentPlan creates the AutoUpdateAgentPlan singleton resource.
func (s *AutoUpdateService) CreateAutoUpdateAgentPlan(
	ctx context.Context,
	v *autoupdate.AutoUpdateAgentPlan,
) (*autoupdate.AutoUpdateAgentPlan, error) {
	plan, err := s.plan.CreateResource(ctx, v)
	return plan, trace.Wrap(err)
}

// UpdateAutoUpdateAgentPlan updates the AutoUpdateAgentPlan singleton resource.
func (s *AutoUpdateService) UpdateAutoUpdateAgentPlan(
	ctx context.Context,
	v *autoupdate.AutoUpdateAgentPlan,
) (*autoupdate.AutoUpdateAgentPlan, error) {
	plan, err := s.plan.UpdateResource(ctx, v)
	return plan, trace.Wrap(err)
}

// UpsertAutoUpdateAgentPlan sets the AutoUpdateAgentPlan singleton resource.
func (s *AutoUpdateService) UpsertAutoUpdateAgentPlan(
	ctx context.Context,
	v *autoupdate.AutoUpdateAgentPlan,
) (*autoupdate.AutoUpdateAgentPlan, error) {
	plan, err := s.plan.UpsertResource(ctx, v)
	return plan, trace.Wrap(err)
}

// GetAutoUpdateAgentPlan gets the AutoUpdateAgentPlan singleton resource.
func (s *AutoUpdateService) GetAutoUpdateAgentPlan(ctx context.Context) (*autoupdate.AutoUpdateAgentPlan, error) {
	plan, err := s.plan.GetResource(ctx, types.MetaNameAutoUpdateAgentPlan)
	return plan, trace.Wrap(err)
}

// DeleteAutoUpdateAgentPlan deletes the AutoUpdateAgentPlan singleton resource.
func (s *AutoUpdateService) DeleteAutoUpdateAgentPlan(ctx context.Context) error {
	return trace.Wrap(s.plan.DeleteResource(ctx, types.MetaNameAutoUpdateAgentPlan))
}
