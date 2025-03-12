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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	discoveryConfigPrefix = "discovery_config"
)

// DiscoveryConfigService manages DiscoveryConfigs in the Backend.
type DiscoveryConfigService struct {
	svc generic.Service[*discoveryconfig.DiscoveryConfig]
}

// NewDiscoveryConfigService creates a new DiscoveryConfigService.
func NewDiscoveryConfigService(b backend.Backend) (*DiscoveryConfigService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*discoveryconfig.DiscoveryConfig]{
		Backend:       b,
		PageLimit:     defaults.MaxIterationLimit,
		ResourceKind:  types.KindDiscoveryConfig,
		BackendPrefix: backend.NewKey(discoveryConfigPrefix),
		MarshalFunc:   services.MarshalDiscoveryConfig,
		UnmarshalFunc: services.UnmarshalDiscoveryConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DiscoveryConfigService{
		svc: *svc,
	}, nil
}

// ListDiscoveryConfigs returns a paginated list of DiscoveryConfig resources.
func (s *DiscoveryConfigService) ListDiscoveryConfigs(ctx context.Context, pageSize int, pageToken string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	dcs, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return dcs, nextKey, nil
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
func (s *DiscoveryConfigService) GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	dc, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return dc, nil
}

// CreateDiscoveryConfig creates a new DiscoveryConfig resource.
func (s *DiscoveryConfigService) CreateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if err := dc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.svc.CreateResource(ctx, dc)
	return created, trace.Wrap(err)
}

// UpdateDiscoveryConfig updates an existing DiscoveryConfig resource.
func (s *DiscoveryConfigService) UpdateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if err := dc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.svc.UpdateResource(ctx, dc)
	return updated, trace.Wrap(err)
}

// UpsertDiscoveryConfig upserts a DiscoveryConfig resource.
func (s *DiscoveryConfigService) UpsertDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if err := dc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.svc.UpsertResource(ctx, dc)
	return upserted, trace.Wrap(err)
}

// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
func (s *DiscoveryConfigService) DeleteDiscoveryConfig(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllDiscoveryConfigs removes all DiscoveryConfig resources.
func (s *DiscoveryConfigService) DeleteAllDiscoveryConfigs(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
