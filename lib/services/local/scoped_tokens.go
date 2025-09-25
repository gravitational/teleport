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

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	scopedTokenPrefix = "scoped_token"
)

// ScopedTokenService exposes backend functionality for working with scoped token resources.
type ScopedTokenService struct {
	svc *generic.ServiceWrapper[*joiningv1.ScopedToken]
}

// NewScopedTokenService creates a new ScopedTokenService.
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

// CreateScopedToken adds a scoped token to the auth server.
func (s *ScopedTokenService) CreateScopedToken(ctx context.Context, req *joiningv1.CreateScopedTokenRequest) (*joiningv1.CreateScopedTokenResponse, error) {
	if err := joining.StrongValidateToken(req.GetToken()); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.svc.CreateResource(ctx, req.GetToken())
	return &joiningv1.CreateScopedTokenResponse{
		Token: created,
	}, trace.Wrap(err)
}

// GetScopedToken finds and returns a scoped token by name.
func (s *ScopedTokenService) GetScopedToken(ctx context.Context, req *joiningv1.GetScopedTokenRequest) (*joiningv1.GetScopedTokenResponse, error) {
	token, err := s.svc.GetResource(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := joining.WeakValidateToken(token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &joiningv1.GetScopedTokenResponse{Token: token}, nil
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
func (s *ScopedTokenService) ListScopedTokens(ctx context.Context, req *joiningv1.ListScopedTokensRequest) (*joiningv1.ListScopedTokensResponse, error) {
	// we only want to return filters if at least one of the filters
	// has been defined, otherwise we should return nil so that the
	// backend can choose to perform a simple list operation instead
	// of a list with filter
	switch {
	case req.ResourceScope != nil:
	case req.AssignedScope != nil:
	case len(req.Roles) > 0:
	case len(req.Labels) > 0:
	default:
		tokens, cursor, err := s.svc.ListResources(ctx, int(req.GetLimit()), req.GetCursor())

		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &joiningv1.ListScopedTokensResponse{
			Tokens: tokens,
			Cursor: cursor,
		}, nil
	}

	if req.GetAssignedScope().GetScope() != "" {
		if err := scopes.WeakValidate(req.GetAssignedScope().GetScope()); err != nil {
			return nil, trace.BadParameter("invalid scope for assigned filter: %s", req.GetAssignedScope().GetScope())
		}

	}

	if req.GetResourceScope().GetScope() != "" {
		if err := scopes.WeakValidate(req.GetResourceScope().GetScope()); err != nil {
			return nil, trace.BadParameter("invalid scope for resource filter: %s", req.GetResourceScope().GetScope())
		}
	}

	filterRoles, err := types.NewTeleportRoles(req.GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filterFn := func(token *joiningv1.ScopedToken) bool {
		if len(req.GetRoles()) > 0 {
			roles, err := types.NewTeleportRoles(token.Spec.Roles)
			if err != nil {
				return false
			}

			if !filterRoles.IncludeAny(roles...) {
				return false
			}
		}

		if !evalScopeFilter(req.GetAssignedScope(), token.Spec.AssignedScope) {
			return false
		}

		if !evalScopeFilter(req.GetResourceScope(), token.Scope) {
			return false
		}

		for k, v := range req.GetLabels() {
			if token.GetMetadata().GetLabels()[k] != v {
				return false
			}
		}

		if err := joining.WeakValidateToken(token); err != nil {
			return false
		}

		return true
	}

	tokens, cursor, err := s.svc.ListResourcesWithFilter(ctx, int(req.GetLimit()), req.GetCursor(), filterFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &joiningv1.ListScopedTokensResponse{
		Tokens: tokens,
		Cursor: cursor,
	}, nil
}

// DeleteScopedToken deletes a scoped token by name.
func (s *ScopedTokenService) DeleteScopedToken(ctx context.Context, req *joiningv1.DeleteScopedTokenRequest) (*joiningv1.DeleteScopedTokenResponse, error) {
	return nil, trace.Wrap(s.svc.DeleteResource(ctx, req.GetName()))
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
				Name: name,
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
