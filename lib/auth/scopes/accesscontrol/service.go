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

package accesscontrol

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopedrolev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedrole/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// Config contains the parameters for [New].
type Config struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
}

// Server is the [scopedrolev1.ScopedRoleServiceServer] returned by [New].
type Server struct {
	scopedrolev1.UnsafeScopedRoleServiceServer

	authorizer authz.Authorizer
	logger     *slog.Logger
}

// New returns the auth server implementation for the scoped access control
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

// CreateScopedRole implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRole(ctx context.Context, req *scopedrolev1.CreateScopedRoleRequest) (*scopedrolev1.CreateScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped roles", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).CreateScopedRole(ctx, req)
}

// CreateScopedRoleAssignment implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRoleAssignment(ctx context.Context, req *scopedrolev1.CreateScopedRoleAssignmentRequest) (*scopedrolev1.CreateScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped role assignments", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).CreateScopedRoleAssignment(ctx, req)
}

// DeleteScopedRole implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRole(ctx context.Context, req *scopedrolev1.DeleteScopedRoleRequest) (*scopedrolev1.DeleteScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to delete a scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped roles", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).DeleteScopedRole(ctx, req)
}

// DeleteScopedRoleAssignment implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRoleAssignment(ctx context.Context, req *scopedrolev1.DeleteScopedRoleAssignmentRequest) (*scopedrolev1.DeleteScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to delete a scoped role assignment", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped role assignment", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).DeleteScopedRoleAssignment(ctx, req)
}

// GetScopedRole implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) GetScopedRole(ctx context.Context, req *scopedrolev1.GetScopedRoleRequest) (*scopedrolev1.GetScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to get scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped roles", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).GetScopedRole(ctx, req)
}

// GetScopedRoleAssignment implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) GetScopedRoleAssignment(ctx context.Context, req *scopedrolev1.GetScopedRoleAssignmentRequest) (*scopedrolev1.GetScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to get scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped role assignments", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoleAssignments(ctx context.Context, req *scopedrolev1.ListScopedRoleAssignmentsRequest) (*scopedrolev1.ListScopedRoleAssignmentsResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to list scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped role assignments", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).ListScopedRoleAssignments(ctx, req)
}

// ListScopedRoles implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoles(ctx context.Context, req *scopedrolev1.ListScopedRolesRequest) (*scopedrolev1.ListScopedRolesResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to list scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped roles", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).ListScopedRoles(ctx, req)
}

// UpdateScopedRole implements [scopedrolev1.ScopedRoleServiceServer].
func (s *Server) UpdateScopedRole(ctx context.Context, req *scopedrolev1.UpdateScopedRoleRequest) (*scopedrolev1.UpdateScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission update scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission update scoped roles", authzContext.User.GetName())
	}

	return (scopedrolev1.UnimplementedScopedRoleServiceServer{}).UpdateScopedRole(ctx, req)
}
