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
	svc              generic.Service[types.Integration]
	backend          backend.Backend
	preDeleteChecks  []PreDeleteCheck
	deleteAllEnabled bool
}

// IntegrationsServiceOption is a functional option for the IntegrationsService.
type IntegrationsServiceOption func(*IntegrationsService)

// PreDeleteCheck is a check that should run before any integration is deleted to make sure it is safe to
// delete.
//
// If the check returns any error, the integration will not be deleted.
//
// The check should probably hold a lock around both the check and the integration deletion to avoid race
// conditions. If the check does not return an error, [release] will be invoked by the caller to release the
// lock.
type PreDeleteCheck func(ctx context.Context, bk backend.Backend, integrationName string) (release func(), err error)

// WithPreDeleteChecks is a functional option for the IntegrationsService to use the given [preDeleteChecks].
func WithPreDeleteChecks(preDeleteChecks ...PreDeleteCheck) func(*IntegrationsService) {
	return func(svc *IntegrationsService) {
		svc.preDeleteChecks = append(svc.preDeleteChecks, preDeleteChecks...)
	}
}

// WithDeleteAllIntegrationsEnabled configure the IntegrationsService to support the DeleteAllIntegrations
// method, which does not include protections against deleting integrations referenced by other components and
// should only be used in e.g. a local cache.
func WithDeleteAllIntegrationsEnabled(deleteAllEnabled bool) func(*IntegrationsService) {
	return func(svc *IntegrationsService) {
		svc.deleteAllEnabled = deleteAllEnabled
	}
}

// NewIntegrationsService creates a new IntegrationsService.
func NewIntegrationsService(backend backend.Backend, opts ...IntegrationsServiceOption) (*IntegrationsService, error) {
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
	integrationsSvc := &IntegrationsService{
		svc:     *svc,
		backend: backend,
	}
	for _, opt := range opts {
		opt(integrationsSvc)
	}
	return integrationsSvc, nil
}

// ListIntegrations returns a paginated list of Integration resources.
func (s *IntegrationsService) ListIntegrations(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
	igs, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return igs, nextKey, nil
}

// GetIntegration returns the specified Integration resource.
func (s *IntegrationsService) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	ig, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// CreateIntegration creates a new Integration resource.
func (s *IntegrationsService) CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	if err := services.CheckAndSetDefaults(ig); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.svc.CreateResource(ctx, ig)
	return created, trace.Wrap(err)
}

// UpdateIntegration updates an existing Integration resource.
func (s *IntegrationsService) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	if err := services.CheckAndSetDefaults(ig); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.svc.UpdateResource(ctx, ig)
	return updated, trace.Wrap(err)
}

// DeleteIntegration removes the specified Integration resource.
func (s *IntegrationsService) DeleteIntegration(ctx context.Context, name string) error {
	for _, preDeleteCheck := range s.preDeleteChecks {
		release, err := preDeleteCheck(ctx, s.backend, name)
		if err != nil {
			return trace.Wrap(err, "running pre-delete check for integration %q", name)
		}
		defer release()
	}
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllIntegrations removes all Integration resources. This should only be used in a cache.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	if !s.deleteAllEnabled {
		return trace.BadParameter("Deleting all integrations is not supported, this is a bug")
	}
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
