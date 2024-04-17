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
	easSvc           *ExternalAuditStorageService
	backend          backend.Backend
	deleteAllEnabled bool
}

// IntegrationsServiceOption is a functional option for the IntegrationsService.
type IntegrationsServiceOption func(*IntegrationsService)

// WithDeleteAllIntegrationsEnabled configure the IntegrationsService to support the DeleteAllIntegrations
// method, which does not include protections against deleting integrations referenced by other components and
// should only be used in e.g. a local cache.
func WithDeleteAllIntegrationsEnabled(deleteAllEnabled bool) func(*IntegrationsService) {
	return func(svc *IntegrationsService) {
		svc.deleteAllEnabled = deleteAllEnabled
	}
}

// WithExternalAuditStorageService configure the IntegrationsService to use the given
// ExternalAuditStorageService.
func WithExternalAuditStorageService(easSvc *ExternalAuditStorageService) func(*IntegrationsService) {
	return func(svc *IntegrationsService) {
		svc.easSvc = easSvc
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
	if integrationsSvc.easSvc == nil {
		integrationsSvc.easSvc, err = NewExternalAuditStorageServiceFallible(backend, WithIntegrationsService(integrationsSvc))
		if err != nil {
			return nil, trace.Wrap(err)
		}
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
	ig, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	awsOIDCSpec := ig.GetAWSOIDCIntegrationSpec()
	if awsOIDCSpec == nil {
		// Not an AWS OIDC integration, go ahead and delete it.
		return trace.Wrap(s.svc.DeleteResource(ctx, name))
	}

	// Avoid deleting AWS OIDC integrations referenced by any External Audit Storage integration.
	// It's necessary to do this under the same lock that is used to create and promote EAS configurations to
	// avoid a race condition where an AWS OIDC configuration could be deleted immediately after a new EAS
	// configuration is added.
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalAuditStorageLockName,
			TTL:      externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		if draft, err := s.easSvc.GetDraftExternalAuditStorage(ctx); err == nil {
			if draft.Spec.IntegrationName == name {
				return trace.BadParameter("cannot delete AWS OIDC integration currently referenced by draft External Audit Storage integration")
			}
		} else if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if cluster, err := s.easSvc.GetClusterExternalAuditStorage(ctx); err == nil {
			if cluster.Spec.IntegrationName == name {
				return trace.BadParameter("cannot delete AWS OIDC integration currently referenced by cluster External Audit Storage integration")
			}
		} else if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.svc.DeleteResource(ctx, name))
	})
	return trace.Wrap(err)
}

// DeleteAllIntegrations removes all Integration resources.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	if !s.deleteAllEnabled {
		return trace.BadParameter("Deleting all integrations is not supported, this is a bug")
	}
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
