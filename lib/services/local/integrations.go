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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	integrationsPrefix = "integrations"
)

// IntegrationsService manages Integrations in the Backend.
type IntegrationsService struct {
	svc generic.Service[types.Integration]
}

// NewIntegrationsService creates a new IntegrationsService.
func NewIntegrationsService(backend backend.Backend) (*IntegrationsService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.Integration]{
		Backend:       backend,
		PageLimit:     defaults.MaxIterationLimit,
		ResourceKind:  types.KindIntegration,
		BackendPrefix: integrationsPrefix,
		MarshalFunc:   services.MarshalIntegration,
		UnmarshalFunc: services.UnmarshalIntegration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &IntegrationsService{
		svc: *svc,
	}, nil
}

// ListIntegrationss returns a paginated list of Integration resources.
func (s *IntegrationsService) ListIntegrations(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
	igs, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return igs, nextKey, nil
}

// GetIntegrations returns the specified Integration resource.
func (s *IntegrationsService) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	ig, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// CreateIntegrations creates a new Integration resource.
func (s *IntegrationsService) CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	if err := ig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.svc.CreateResource(ctx, ig); err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// UpdateIntegrations updates an existing Integration resource.
func (s *IntegrationsService) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	if err := ig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.svc.UpdateResource(ctx, ig); err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// DeleteIntegrations removes the specified Integration resource.
func (s *IntegrationsService) DeleteIntegration(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllIntegrationss removes all Integration resources.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
