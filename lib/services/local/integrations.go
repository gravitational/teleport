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
	"iter"

	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	integrationsPrefix = "integrations"
)

// IntegrationsService manages Integrations in the Backend.
type IntegrationsService struct {
	svc       *generic.ScopeAwareService[types.Integration]
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
	svc, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[types.Integration]{
		Backend:               b,
		PageLimit:             defaults.MaxIterationLimit,
		ResourceKind:          types.KindIntegration,
		UnscopedBackendPrefix: integrationsUnscopedPrefix(),
		ScopedBackendPrefix:   integrationsScopedPrefix(),
		MarshalFunc:           services.MarshalIntegration,
		UnmarshalFunc:         services.UnmarshalIntegration,
		ValidateFunc:          validateIntegration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integrationsSvc := &IntegrationsService{
		svc:     svc,
		backend: b,
	}
	for _, opt := range opts {
		opt(integrationsSvc)
	}
	return integrationsSvc, nil
}

// ListIntegrations returns a page of Integration resources respecting scope filters.
func (s *IntegrationsService) ListIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) ([]types.Integration, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return s.svc.ListResourcesWithFilter(ctx, int(req.GetLimit()), req.GetNextKey(), func(ig types.Integration) bool {
		return scopes.MatchScope(scopeFilter, ig.GetScope())
	})
}

// RangeIntegrations returns Integration resources within the range [start, end) respecting scope filters.
func (s *IntegrationsService) RangeIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) iter.Seq2[types.Integration, error] {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.Integration](trace.Wrap(err))
	}

	filterFn := func(ig types.Integration) (types.Integration, bool) {
		return ig, scopes.MatchScope(scopeFilter, ig.GetScope())
	}

	return stream.FilterMap(s.svc.Resources(ctx, req.GetNextKey(), ""), filterFn)
}

// GetIntegration returns the specified scoped or unscoped Integration resource.
func (s *IntegrationsService) GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (types.Integration, error) {
	sqn := apiscopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}
	if sqn.Scope != "" {
		if err := sqn.WeakValidate(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return s.svc.GetResource(ctx, sqn)
}

// CreateIntegration creates a new Integration resource.
func (s *IntegrationsService) CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	created, err := s.svc.CreateResource(ctx, ig)
	return created, trace.Wrap(err)
}

// UpdateIntegration updates an existing Integration resource.
func (s *IntegrationsService) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	updated, err := s.svc.UpdateResource(ctx, ig)
	return updated, trace.Wrap(err)
}

// DeleteIntegration removes the specified scoped or unscoped Integration resource.
func (s *IntegrationsService) DeleteIntegration(ctx context.Context, req *integrationpb.DeleteIntegrationRequest) error {
	sqn := apiscopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}
	if sqn.Scope != "" {
		if err := sqn.WeakValidate(); err != nil {
			return trace.Wrap(err)
		}
	}

	if s.cacheMode {
		// No checks are done in cache mode.
		return trace.Wrap(s.svc.DeleteResource(ctx, sqn))
	}

	// First check if the integration exists to return NotFound in case it doesn't.
	if _, err := s.svc.GetResource(ctx, sqn); err != nil {
		return trace.Wrap(err)
	}

	// External Audit Storage and the AWS Identity Center plugin reference integrations by
	// bare name, so only an unscoped integration can be in use by them.
	if sqn.Scope != "" {
		return trace.Wrap(s.svc.DeleteResource(ctx, sqn))
	}

	deleteConditions, err := integrationDeletionConditions(ctx, s.backend, sqn.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	deleteConditions = append(deleteConditions, backend.ConditionalAction{
		Key:       s.svc.UnscopedService.MakeKey(backend.NewKey(sqn.Name)),
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

	if err := integrationReferencedByAWSICPlugin(ctx, bk, name); err != nil {
		return nil, trace.Wrap(err)
	}

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
// by an existing AWS Identity Center plugin.
func integrationReferencedByAWSICPlugin(ctx context.Context, bk backend.Backend, name string) error {
	pluginService := NewPluginsService(bk)
	plugins, err := pluginService.GetPlugins(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, p := range plugins {
		pluginV1, ok := p.(*types.PluginV1)
		if !ok {
			continue
		}
		switch pluginV1.GetType() {
		case types.PluginTypeAWSIdentityCenter:
			if awsIC := pluginV1.Spec.GetAwsIc(); awsIC != nil {
				if awsIC.IntegrationName == name {
					return trace.BadParameter("cannot delete AWS OIDC integration currently referenced by AWS Identity Center integration %q", pluginV1.GetName())
				}
			}
		default:
			continue
		}
	}

	return nil
}

// DeleteAllIntegrations removes all Integration resources, in every scope. This should only be used in a cache.
func (s *IntegrationsService) DeleteAllIntegrations(ctx context.Context) error {
	if !s.cacheMode {
		return trace.BadParameter("Deleting all integrations is not supported, this is a bug")
	}
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

func integrationsUnscopedPrefix() backend.Key {
	return backend.NewKey(integrationsPrefix)
}

func integrationsScopedPrefix() backend.Key {
	return backend.NewKey(scopedPrefix, integrationsPrefix)
}

// validateIntegration runs shared validation for an Integration resource. Scoped integrations are
// limited to the AWS OIDC and AWS Roles Anywhere subkinds, and a scoped AWS OIDC integration must
// use the proxy-served issuer (RFD 0309).
func validateIntegration(ig types.Integration) error {
	if err := services.CheckAndSetDefaults(ig); err != nil {
		return trace.Wrap(err)
	}

	if ig.GetScope() == "" {
		return nil
	}

	if err := apiscopes.StrongValidate(ig.GetScope()); err != nil {
		return trace.Wrap(err)
	}

	switch ig.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		if spec := ig.GetAWSOIDCIntegrationSpec(); spec != nil && spec.IssuerS3URI != "" {
			return trace.BadParameter("scoped AWS OIDC integrations do not support issuer_s3_uri")
		}
	case types.IntegrationSubKindAWSRolesAnywhere:
	default:
		return trace.BadParameter("scoped integrations do not support the %q subkind", ig.GetSubKind())
	}

	// Credentials are not yet scoped, and AWS OIDC and Roles Anywhere should never define
	// credentials.
	if ig.GetCredentials() != nil {
		return trace.BadParameter("scoped integrations do not support credentials")
	}

	return nil
}
