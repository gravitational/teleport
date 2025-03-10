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
func NewIntegrationsService(b backend.Backend, opts ...IntegrationsServiceOption) (*IntegrationsService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.Integration]{
		Backend:       b,
		PageLimit:     defaults.MaxIterationLimit,
		ResourceKind:  types.KindIntegration,
		BackendPrefix: backend.NewKey(integrationsPrefix),
		MarshalFunc:   services.MarshalIntegration,
		UnmarshalFunc: services.UnmarshalIntegration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integrationsSvc := &IntegrationsService{
		svc:     *svc,
		backend: b,
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
	if s.cacheMode {
		// No checks are done in cache mode.
		return trace.Wrap(s.svc.DeleteResource(ctx, name))
	}

	// First check if the integration exists to return NotFound in case it doesn't.
	if _, err := s.svc.GetResource(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	deleteConditions, err := integrationDeletionConditions(ctx, s.backend, name)
	if err != nil {
		return trace.Wrap(err)
	}
	deleteConditions = append(deleteConditions, backend.ConditionalAction{
		Key:       s.svc.MakeKey(backend.NewKey(name)),
		Condition: backend.Exists(),
		Action:    backend.Delete(),
	})
	_, err = s.backend.AtomicWrite(ctx, deleteConditions)
	return trace.Wrap(err)
}

// integrationDeletionConditions returns a BadParameter error if the integration is referenced by another
// Teleport service. If it does not find any direct reference, the backend.ConditionalAction is returned
// with the current state of reference, which should be added to AtomicWrite to ensure that the current
// reference state remains unchanged until the integration is completely deleted.
// Service may have zero or multiple ConditionalActions returned.
func integrationDeletionConditions(ctx context.Context, bk backend.Backend, name string) ([]backend.ConditionalAction, error) {
	var deleteConditionalActions []backend.ConditionalAction
	easDeleteConditions, err := integrationReferencedByEAS(ctx, bk, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deleteConditionalActions = append(deleteConditionalActions, easDeleteConditions...)

	awsIcDeleteCondition, err := integrationReferencedByAWSICPlugin(ctx, bk, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deleteConditionalActions = append(deleteConditionalActions, awsIcDeleteCondition...)

	return deleteConditionalActions, nil
}

// integrationReferencedByEAS returns a slice of ConditionalActions to use with a backend.AtomicWrite to ensure that
// integration [name] is not referenced by any EAS (External Audit Storage) integration.
func integrationReferencedByEAS(ctx context.Context, bk backend.Backend, name string) ([]backend.ConditionalAction, error) {
	var conditionalActions []backend.ConditionalAction
	for _, key := range []backend.Key{draftExternalAuditStorageBackendKey, clusterExternalAuditStorageBackendKey} {
		condition := backend.ConditionalAction{
			Key:    key,
			Action: backend.Nop(),
			// Condition: will be set below based on existence of key.
		}

		eas, err := getExternalAuditStorage(ctx, bk, key)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			// If this EAS configuration currently doesn't exist, make sure it still doesn't exist when
			// deleting the AWS integration.
			condition.Condition = backend.NotExists()
		} else {
			if eas.Spec.IntegrationName == name {
				return nil, trace.BadParameter("cannot delete AWS OIDC integration currently referenced by External Audit Storage integration")
			}
			// If this EAS configuration currently doesn't reference the AWS integration being deleted, make
			// sure it hasn't changed when deleting the AWS integration.
			condition.Condition = backend.Revision(eas.GetRevision())
		}

		conditionalActions = append(conditionalActions, condition)
	}
	return conditionalActions, nil
}

// integrationReferencedByAWSICPlugin returns an error if the integration name is referenced
// by an existing AWS Identity Center plugin. In case the AWS Identity Center plugin exists
// but does not reference this integration, a conditional action is returned with a revision
// of the plugin to ensure that plugin hasn't changed during deletion of the AWS OIDC integration.
func integrationReferencedByAWSICPlugin(ctx context.Context, bk backend.Backend, name string) ([]backend.ConditionalAction, error) {
	var conditionalActions []backend.ConditionalAction
	pluginService := NewPluginsService(bk)
	plugins, err := pluginService.GetPlugins(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, p := range plugins {
		pluginV1, ok := p.(*types.PluginV1)
		if !ok {
			continue
		}
		if pluginV1.GetType() != types.PluginType(types.PluginTypeAWSIdentityCenter) {
			continue
		}
		if awsIC := pluginV1.Spec.GetAwsIc(); awsIC != nil {
			switch awsIC.IntegrationName {
			case name:
				return nil, trace.BadParameter("cannot delete AWS OIDC integration currently referenced by AWS Identity Center integration %q", pluginV1.GetName())
			default:
				conditionalActions = append(conditionalActions, backend.ConditionalAction{
					Key:       backend.NewKey(pluginsPrefix, name),
					Action:    backend.Nop(),
					Condition: backend.Revision(pluginV1.GetRevision()),
				})
				return conditionalActions, nil
			}
		}
	}

	return conditionalActions, nil
}

// DeleteAllIntegrations removes all Integration resources. This should only be used in a cache.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	if !s.cacheMode {
		return trace.BadParameter("Deleting all integrations is not supported, this is a bug")
	}
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
