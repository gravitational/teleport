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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// AppService manages application resources in the backend.
type AppService struct {
	backend.Backend
	logger *slog.Logger
	svc    *generic.ScopeAwareService[types.Application]
}

// NewAppService creates a new AppService.
func NewAppService(b backend.Backend) (*AppService, error) {
	svc, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[types.Application]{
		Backend:               b,
		ResourceKind:          types.KindApp,
		UnscopedBackendPrefix: appsUnscopedPrefix(),
		ScopedBackendPrefix:   appsScopedPrefix(),
		MarshalFunc: func(app types.Application, option ...services.MarshalOption) ([]byte, error) {
			return services.MarshalApp(app, option...)
		},
		UnmarshalFunc: func(bytes []byte, option ...services.MarshalOption) (types.Application, error) {
			app, err := services.UnmarshalApp(bytes, option...)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return app, nil
		},
		ValidateFunc: validateApp,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AppService{
		Backend: b,
		logger:  slog.With(teleport.ComponentKey, "AppService"),
		svc:     svc,
	}, nil
}

// ListApplications returns a page of registered applications respecting scope filters.
func (s *AppService) ListApplications(ctx context.Context, req *presencev1.ListApplicationsRequest) ([]types.Application, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}

	filterFn := func(app types.Application) bool {
		return scopes.MatchScope(scopeFilter, app.GetScope())
	}

	return s.svc.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), filterFn)
}

// RangeApplications returns applications within the range [start, end).
func (s *AppService) RangeApplications(ctx context.Context, req *presencev1.ListApplicationsRequest) iter.Seq2[types.Application, error] {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.Application](trace.Wrap(err))
	}

	filterFn := func(app types.Application) (types.Application, bool) {
		return app, scopes.MatchScope(scopeFilter, app.GetScope())
	}

	return stream.FilterMap(s.svc.Resources(ctx, req.GetPageToken(), ""), filterFn)
}

// GetApplication returns the specified application resource.
func (s *AppService) GetApplication(ctx context.Context, req *presencev1.GetApplicationRequest) (types.Application, error) {
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

// DeleteApplication removes the specified application resource.
func (s *AppService) DeleteApplication(ctx context.Context, req *presencev1.DeleteApplicationRequest) error {
	return s.svc.DeleteResource(ctx, apiscopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}

// CreateApp creates a new application resource.
func (s *AppService) CreateApp(ctx context.Context, app types.Application) error {
	_, err := s.svc.CreateResource(ctx, app)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("app %q already exists", apiscopes.QualifiedName{Scope: app.GetScope(), Name: app.GetName()}.String())
	}
	return trace.Wrap(err)
}

// UpdateApp updates an existing application resource.
func (s *AppService) UpdateApp(ctx context.Context, app types.Application) error {
	_, err := s.svc.UpdateResource(ctx, app)
	if trace.IsNotFound(err) {
		return trace.NotFound("app %q doesn't exist", apiscopes.QualifiedName{Scope: app.GetScope(), Name: app.GetName()}.String())
	}
	return trace.Wrap(err)
}

// DeleteAllApps removes all application resources.
func (s *AppService) DeleteAllApps(ctx context.Context) error {
	return s.svc.DeleteAllResources(ctx)
}

// AppendPutAppActions adds conditional actions to an atomic write to create or
// update an application resource.
func (s *AppService) AppendPutAppActions(
	actions []backend.ConditionalAction,
	app types.Application,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	if err := validateApp(app); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.svc.MakeBackendItem(app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(actions, backend.ConditionalAction{
		Key:       item.Key,
		Condition: condition,
		Action:    backend.Put(item),
	}), nil
}

// AppendDeleteAppActions adds conditional actions to an atomic write to delete
// an application resource.
func (s *AppService) AppendDeleteAppActions(
	actions []backend.ConditionalAction,
	name string,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	svc, err := s.svc.WithScopePrefix("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(actions, backend.ConditionalAction{
		Key:       svc.MakeKey(backend.NewKey(name)),
		Condition: condition,
		Action:    backend.Delete(),
	}), nil
}

// validateApp runs shared validation for an application resource.
// Scoped apps currently reject dynamic labels, MCP, LLM, and integrations
func validateApp(app types.Application) error {
	if err := services.CheckAndSetDefaults(app); err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateAppIntegration(app); err != nil {
		return trace.Wrap(err)
	}

	if app.GetScope() == "" {
		return nil
	}

	if err := apiscopes.StrongValidate(app.GetScope()); err != nil {
		return trace.Wrap(err)
	}

	if app.IsAzureCloud() || app.IsGCP() {
		return trace.BadParameter("scoped applications do not support Azure or GCP cloud apps")
	}

	if len(app.GetRequiredAppNames()) > 0 {
		return trace.BadParameter("scoped applications do not support required app names")
	}

	if len(app.GetDynamicLabels()) > 0 {
		return trace.BadParameter("scoped applications do not support dynamic labels")
	}

	if app.IsMCP() {
		return trace.BadParameter("scoped applications do not support stdio MCP servers")
	}

	if app.IsLLM() {
		return trace.BadParameter("scoped applications do not support LLM apps")
	}

	return nil
}

func appsUnscopedPrefix() backend.Key {
	return backend.NewKey(appPrefix)
}

func appsScopedPrefix() backend.Key {
	return backend.NewKey(scopedPrefix, appPrefix)
}

const (
	appPrefix = "applications"
)
