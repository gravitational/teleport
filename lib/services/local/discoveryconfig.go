/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
func NewDiscoveryConfigService(backend backend.Backend) (*DiscoveryConfigService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*discoveryconfig.DiscoveryConfig]{
		Backend:       backend,
		PageLimit:     defaults.MaxIterationLimit,
		ResourceKind:  types.KindDiscoveryConfig,
		BackendPrefix: discoveryConfigPrefix,
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

// ListDiscoveryConfigss returns a paginated list of DiscoveryConfig resources.
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

// CreateDiscoveryConfigs creates a new DiscoveryConfig resource.
func (s *DiscoveryConfigService) CreateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if err := dc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.svc.CreateResource(ctx, dc); err != nil {
		return nil, trace.Wrap(err)
	}

	return dc, nil
}

// UpdateDiscoveryConfigs updates an existing DiscoveryConfig resource.
func (s *DiscoveryConfigService) UpdateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if err := dc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.svc.UpdateResource(ctx, dc); err != nil {
		return nil, trace.Wrap(err)
	}

	return dc, nil
}

// DeleteDiscoveryConfigs removes the specified DiscoveryConfig resource.
func (s *DiscoveryConfigService) DeleteDiscoveryConfig(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllDiscoveryConfigss removes all DiscoveryConfig resources.
func (s *DiscoveryConfigService) DeleteAllDiscoveryConfigs(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
