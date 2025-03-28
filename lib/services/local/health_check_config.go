/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const healthCheckConfigPrefix = "health_check_config"

// HealthCheckConfigService manages [healthcheckconfigv1.HealthCheckConfig] resources in
// the backend.
type HealthCheckConfigService struct {
	svc *generic.ServiceWrapper[*healthcheckconfigv1.HealthCheckConfig]
}

// NewHealthCheckConfigService creates a new HealthCheckConfigService.
func NewHealthCheckConfigService(b backend.Backend) (*HealthCheckConfigService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*healthcheckconfigv1.HealthCheckConfig]{
			Backend:       b,
			ResourceKind:  types.KindHealthCheckConfig,
			BackendPrefix: backend.NewKey(healthCheckConfigPrefix),
			MarshalFunc:   services.MarshalHealthCheckConfig,
			UnmarshalFunc: services.UnmarshalHealthCheckConfig,
			ValidateFunc:  services.ValidateHealthCheckConfig,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &HealthCheckConfigService{
		svc: service,
	}, nil
}

// CreateHealthCheckConfig creates a new HealthCheckConfig resource.
func (s *HealthCheckConfigService) CreateHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	created, err := s.svc.CreateResource(ctx, config)
	return created, trace.Wrap(err)
}

// GetHealthCheckConfig returns the specified HealthCheckConfig resource.
func (s *HealthCheckConfigService) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	item, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// ListHealthCheckConfigs returns a paginated list of HealthCheckConfig resources.
func (s *HealthCheckConfigService) ListHealthCheckConfigs(ctx context.Context, pageSize int, pageToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	items, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return items, nextKey, nil
}

// UpdateHealthCheckConfig updates an existing HealthCheckConfig resource.
func (s *HealthCheckConfigService) UpdateHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	updated, err := s.svc.ConditionalUpdateResource(ctx, config)
	return updated, trace.Wrap(err)
}

// UpsertHealthCheckConfig upserts an existing HealthCheckConfig resource.
func (s *HealthCheckConfigService) UpsertHealthCheckConfig(ctx context.Context, config *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	upserted, err := s.svc.UpsertResource(ctx, config)
	return upserted, trace.Wrap(err)
}

// DeleteHealthCheckConfig removes the specified HealthCheckConfig resource.
func (s *HealthCheckConfigService) DeleteHealthCheckConfig(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllHealthCheckConfigs removes all HealthCheckConfig resources.
func (s *HealthCheckConfigService) DeleteAllHealthCheckConfigs(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
