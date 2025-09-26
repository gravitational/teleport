// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package local

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	scopedTokenPrefix = "scoped_token"
)

type ScopedTokenService struct {
	svc *generic.ServiceWrapper[*joiningv1.ScopedToken]
}

var _ services.ScopedTokenService = &ScopedTokenService{}

func NewScopedTokenService(b backend.Backend) (*ScopedTokenService, error) {
	const pageLimit = 100
	svc, err := generic.NewServiceWrapper(generic.ServiceConfig[*joiningv1.ScopedToken]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindScopedToken,
		BackendPrefix: backend.NewKey(scopedTokenPrefix),
		MarshalFunc:   services.MarshalProtoResource[*joiningv1.ScopedToken],
		UnmarshalFunc: services.UnmarshalProtoResource[*joiningv1.ScopedToken],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopedTokenService{
		svc: svc,
	}, nil
}

// CreateScopedToken adds a scoped join token to the auth server
func (s *ScopedTokenService) CreateScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	created, err := s.svc.CreateResource(ctx, token)
	return created, trace.Wrap(err)
}

// UpdateScopedToken
func (s *ScopedTokenService) UpdateScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	updated, err := s.svc.ConditionalUpdateResource(ctx, token)
	return updated, trace.Wrap(err)
}

// UpsertScopedToken
func (s *ScopedTokenService) UpsertScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	upserted, err := s.svc.UpsertResource(ctx, token)
	return upserted, trace.Wrap(err)
}

// GetScopedToken finds and returns token by id
func (s *ScopedTokenService) GetScopedToken(ctx context.Context, name string) (*joiningv1.ScopedToken, error) {
	token, err := s.svc.GetResource(ctx, name)
	return token, trace.Wrap(err)
}

func validateScopedTokenFilters(filters *services.ScopedTokenFilters) error {
	if filters == nil {
		return nil
	}

	if filters.AssignedScope.GetScope() != "" {
		if err := scopes.StrongValidate(filters.AssignedScope.Scope); err != nil {
			return trace.BadParameter("invalid scope for assigned filter: %s", filters.AssignedScope.Scope)
		}
	}

	if filters.ResourceScope.GetScope() != "" {
		if err := scopes.StrongValidate(filters.ResourceScope.Scope); err != nil {
			return trace.BadParameter("invalid scope for resource filter: %s", filters.ResourceScope.Scope)
		}
	}

	return nil
}

func evalScopeFilter(filter *scopesv1.Filter, scope string) bool {
	if filter == nil {
		return true
	}

	switch filter.Mode {
	case scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE:
		return scopes.ResourceScope(scope).IsSubjectToPolicyScope(filter.Scope)
	case scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE:
		return scopes.PolicyScope(scope).AppliesToResourceScope(filter.Scope)
	}

	return true
}

// ListScopedTokens retrieves a paginated list of scoped join tokens.
func (s *ScopedTokenService) ListScopedTokens(ctx context.Context, pageSize int, pageToken string, filters *services.ScopedTokenFilters) ([]*joiningv1.ScopedToken, string, error) {
	if filters == nil {
		tokens, cursor, err := s.svc.ListResources(ctx, pageSize, pageToken)

		return tokens, cursor, trace.Wrap(err)
	}

	if err := validateScopedTokenFilters(filters); err != nil {
		return nil, "", trace.Wrap(err)
	}

	filterFn := func(token *joiningv1.ScopedToken) bool {
		if len(filters.Roles) > 0 {
			roles, err := types.NewTeleportRoles(token.Spec.Roles)
			if err != nil {
				return false
			}

			if !filters.Roles.IncludeAny(roles...) {
				return false
			}
		}

		if !evalScopeFilter(filters.AssignedScope, token.Spec.AssignedScope) {
			return false
		}

		if !evalScopeFilter(filters.ResourceScope, token.Scope) {
			return false
		}

		return true
	}

	tokens, newPageToken, err := s.svc.ListResourcesWithFilter(ctx, pageSize, pageToken, filterFn)
	return tokens, newPageToken, trace.Wrap(err)
}

// DeleteScopedToken deletes scoped join token.
func (s *ScopedTokenService) DeleteScopedToken(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

type scopedTokenParser struct {
	baseParser
}

func newScopedTokenParser() *scopedTokenParser {
	return &scopedTokenParser{
		baseParser: newBaseParser(backend.NewKey(scopedTokenPrefix)),
	}
}

func (p *scopedTokenParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		scopedToken, err := services.UnmarshalProtoResource[*joiningv1.ScopedToken](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(scopedToken), nil
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(scopedTokenPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key)
		}
		return &types.ResourceHeader{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
