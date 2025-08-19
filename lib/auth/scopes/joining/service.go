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

package provisioning

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// Config contains the parameters for [New].
type Config struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
}

// Server is the [scopedjoiningv1.ScopedJoiningServiceServer] returned by [New].
type Server struct {
	scopedjoiningv1.UnsafeScopedJoiningServiceServer

	authorizer authz.Authorizer
	logger     *slog.Logger
}

// New returns the auth server implementation for the scoped provisioning
// service, including the gRPC interface, authz enforcement, and business logic.
func New(c Config) (*Server, error) {
	if c.Authorizer == nil {
		return nil, trace.BadParameter("missing Authorizer")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "scopes")
	}

	return &Server{
		authorizer: c.Authorizer,
		logger:     c.Logger,
	}, nil
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

	return (scopedjoiningv1.UnimplementedScopedJoiningServiceServer{}).CreateScopedToken(ctx, req)
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

	return (scopedjoiningv1.UnimplementedScopedJoiningServiceServer{}).DeleteScopedToken(ctx, req)
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

	return (scopedjoiningv1.UnimplementedScopedJoiningServiceServer{}).GetScopedToken(ctx, req)
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

	return (scopedjoiningv1.UnimplementedScopedJoiningServiceServer{}).ListScopedTokens(ctx, req)
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

	return (scopedjoiningv1.UnimplementedScopedJoiningServiceServer{}).UpdateScopedToken(ctx, req)
}
