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
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// Config contains the parameters for [New].
type Config struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
}

// Server is the [scopedaccessv1.UnimplementedScopedAccessServiceServer] returned by [New].
type Server struct {
	scopedaccessv1.UnimplementedScopedAccessServiceServer

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

// CreateScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRole(ctx context.Context, req *scopedaccessv1.CreateScopedRoleRequest) (*scopedaccessv1.CreateScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped roles", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).CreateScopedRole(ctx, req)
}

// CreateScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped role assignments", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).CreateScopedRoleAssignment(ctx, req)
}

// DeleteScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to delete a scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped roles", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).DeleteScopedRole(ctx, req)
}

// DeleteScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to delete a scoped role assignment", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped role assignment", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).DeleteScopedRoleAssignment(ctx, req)
}

// GetScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to get scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped roles", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).GetScopedRole(ctx, req)
}

// GetScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to get scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped role assignments", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to list scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped role assignments", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).ListScopedRoleAssignments(ctx, req)
}

// ListScopedRoles implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to list scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped roles", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).ListScopedRoles(ctx, req)
}

// UpdateScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission update scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission update scoped roles", authzContext.User.GetName())
	}

	return (scopedaccessv1.UnimplementedScopedAccessServiceServer{}).UpdateScopedRole(ctx, req)
}
