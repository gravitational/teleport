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
	svc       generic.Service[types.Integration]
	backend   backend.Backend
	cacheMode bool
}

// IntegrationsServiceOption is a functional option for the IntegrationsService.
type IntegrationsServiceOption func(*IntegrationsService)

// WithIntegrationsServiceCacheMode configures the IntegrationsService to skip certain checks against deleting
// integrations referenced by other components and should only be used in e.g. a local cache.
func WithIntegrationsServiceCacheMode(cacheMode bool) func(*IntegrationsService) {
	return func(svc *IntegrationsService) {
		svc.cacheMode = cacheMode
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

	created, err := s.svc.CreateResource(ctx, ig)
	return created, trace.Wrap(err)
}

// UpdateIntegrations updates an existing Integration resource.
func (s *IntegrationsService) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	if err := ig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.svc.UpdateResource(ctx, ig)
	return updated, trace.Wrap(err)
}

// DeleteIntegrations removes the specified Integration resource.
func (s *IntegrationsService) DeleteIntegration(ctx context.Context, name string) error {
	if s.cacheMode {
		// No checks are done in cache mode.
		return trace.Wrap(s.svc.DeleteResource(ctx, name))
	}

	// Check that this integration is not referenced by an EAS integration under the externalAuditStorageLock
	// so that no new EAS integrations can be concurrently created.
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.backend,
			LockNameComponents: []string{externalAuditStorageLockName},
			TTL:                externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		if err := notReferencedByEAS(ctx, s.backend, name); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.svc.DeleteResource(ctx, name))
	})
	return trace.Wrap(err)
}

// notReferencedByEAS checks that integration [name] is not referenced by any EAS (External Audit Storage)
// integration. It should be called under the externalAuditStorageLock only.
func notReferencedByEAS(ctx context.Context, bk backend.Backend, name string) error {
	for _, key := range []backend.Key{draftExternalAuditStorageBackendKey, clusterExternalAuditStorageBackendKey} {
		eas, err := getExternalAuditStorage(ctx, bk, key)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			// If this EAS configuration currently doesn't exist, continue.
			continue
		}
		if eas.Spec.IntegrationName == name {
			return trace.BadParameter("cannot delete AWS OIDC integration currently referenced by External Audit Storage integration")
		}
	}
	return nil
}

// DeleteAllIntegrations removes all Integration resources. This should only be used in a cache.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	if !s.cacheMode {
		return trace.BadParameter("Deleting all integrations is not supported, this is a bug")
	}
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
