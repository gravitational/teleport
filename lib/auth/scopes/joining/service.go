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
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
	if token.GetMetadata().GetName() == "" {
		if token.Metadata == nil {
			token.Metadata = &headerv1.Metadata{}
		}
		name, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err, "generating token value")
		}
		token.Metadata.Name = name
	}

	res, err := s.backend.CreateScopedToken(ctx, req)
	return res, trace.Wrap(err)
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

	res, err := s.backend.DeleteScopedToken(ctx, req)
	return res, trace.Wrap(err)
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

	res, err := s.backend.GetScopedToken(ctx, req)
	return res, trace.Wrap(err)
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

	res, err := s.backend.ListScopedTokens(ctx, req)
	return res, trace.Wrap(err)
}

// UpdateScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) UpdateScopedToken(ctx context.Context, req *scopedjoiningv1.UpdateScopedTokenRequest) (*scopedjoiningv1.UpdateScopedTokenResponse, error) {
	return nil, trace.NotImplemented("scoped tokens must be recreated, they cannot be updated")
}
