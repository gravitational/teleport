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

package joining

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// Config contains the parameters for [New].
type Config struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
	Backend    services.ScopedTokenService
}

// Server is the [scopedjoiningv1.ScopedJoiningServiceServer] returned by [New].
type Server struct {
	scopedjoiningv1.UnsafeScopedJoiningServiceServer

	authorizer authz.Authorizer
	logger     *slog.Logger
	backend    services.ScopedTokenService
}

// New returns the auth server implementation for the scoped provisioning
// service, including the gRPC interface, authz enforcement, and business logic.
func New(c Config) (*Server, error) {
	if c.Authorizer == nil {
		return nil, trace.BadParameter("missing Authorizer")
	}

	if c.Backend == nil {
		return nil, trace.BadParameter("missing Backend")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "scopes")
	}

	return &Server{
		authorizer: c.Authorizer,
		logger:     c.Logger,
		backend:    c.Backend,
	}, nil
}

func validateScopedToken(token *scopedjoiningv1.ScopedToken) error {
	spec := token.GetSpec()
	if spec == nil {
		return trace.BadParameter("scoped token must not be nil")
	}

	if token.GetScope() == "" {
		return trace.BadParameter("scoped token must have a scope assigned")
	}

	if err := scopes.StrongValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if spec.AssignedScope != "" {
		if err := scopes.StrongValidate(spec.AssignedScope); err != nil {
			return trace.Wrap(err, "validating scoped token assigned scope")
		}
		if !scopes.ResourceScope(spec.AssignedScope).IsSubjectToPolicyScope(token.GetScope()) {
			return trace.BadParameter("scoped token assigned scope must be descendant of its resource scope")
		}
	}

	if len(spec.Roles) == 0 {
		return trace.BadParameter("at least one role must be assigned to a token")
	}

	return nil
}

// CreateScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) CreateScopedToken(ctx context.Context, req *scopedjoiningv1.CreateScopedTokenRequest) (*scopedjoiningv1.CreateScopedTokenResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped tokens", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped tokens", authzContext.User.GetName())
	}

	token := req.GetToken()
	if err := validateScopedToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err = s.backend.CreateScopedToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedjoiningv1.CreateScopedTokenResponse{
		Token: token,
	}, nil
}

// DeleteScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) DeleteScopedToken(ctx context.Context, req *scopedjoiningv1.DeleteScopedTokenRequest) (*scopedjoiningv1.DeleteScopedTokenResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to delete scoped tokens", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete scoped tokens", authzContext.User.GetName())
	}

	if err := s.backend.DeleteScopedToken(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedjoiningv1.DeleteScopedTokenResponse{}, nil
}

// GetScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) GetScopedToken(ctx context.Context, req *scopedjoiningv1.GetScopedTokenRequest) (*scopedjoiningv1.GetScopedTokenResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to get scoped tokens", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped tokens", authzContext.User.GetName())
	}

	token, err := s.backend.GetScopedToken(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedjoiningv1.GetScopedTokenResponse{
		Token: token,
	}, nil
}

func getScopedTokenFiltersFromReq(req *scopedjoiningv1.ListScopedTokensRequest) (*services.ScopedTokenFilters, error) {
	roles, err := types.NewTeleportRoles(req.Roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filters := &services.ScopedTokenFilters{
		AssignedScope: req.AssignedScope,
		ResourceScope: req.ResourceScope,
		Roles:         roles,
	}

	switch {
	case filters.AssignedScope != nil:
	case filters.ResourceScope != nil:
	case len(filters.Roles) > 0:
	default:
		filters = nil
	}

	return filters, nil
}

// ListScopedTokens implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) ListScopedTokens(ctx context.Context, req *scopedjoiningv1.ListScopedTokensRequest) (*scopedjoiningv1.ListScopedTokensResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to list scoped tokens", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped tokens", authzContext.User.GetName())
	}

	filters, err := getScopedTokenFiltersFromReq(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokens, cursor, err := s.backend.ListScopedTokens(ctx, int(req.Limit), req.Cursor, filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedjoiningv1.ListScopedTokensResponse{
		Tokens: tokens,
		Cursor: cursor,
	}, nil
}

// UpdateScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) UpdateScopedToken(ctx context.Context, req *scopedjoiningv1.UpdateScopedTokenRequest) (*scopedjoiningv1.UpdateScopedTokenResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to update scoped tokens", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to update scoped tokens", authzContext.User.GetName())
	}

	token := req.GetToken()
	if err := validateScopedToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err = s.backend.UpdateScopedToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedjoiningv1.UpdateScopedTokenResponse{
		Token: token,
	}, nil
}
