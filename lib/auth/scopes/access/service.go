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

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
)

// Config contains the parameters for [New].
type Config struct {
	Authorizer authz.Authorizer
	Reader     services.ScopedAccessReader
	Writer     services.ScopedAccessWriter
	Logger     *slog.Logger
}

// CheckAndSetDefaults checks the config for missing parameters and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer in scoped access grpc service config")
	}

	if c.Reader == nil {
		return trace.BadParameter("missing Reader in scoped access grpc service config")
	}

	if c.Writer == nil {
		return trace.BadParameter("missing Writer in scoped access grpc service config")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "scopes")
	}

	return nil
}

// Server is the [scopedaccessv1.UnimplementedScopedAccessServiceServer] returned by [New].
type Server struct {
	scopedaccessv1.UnimplementedScopedAccessServiceServer
	cfg Config
}

// New returns the auth server implementation for the scoped access control
// service, including the gRPC interface, authz enforcement, and business logic.
func New(cfg Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Server{
		cfg: cfg,
	}, nil
}

// CreateScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRole(ctx context.Context, req *scopedaccessv1.CreateScopedRoleRequest) (*scopedaccessv1.CreateScopedRoleResponse, error) {
	if err := scopes.AssertFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to create scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped roles", authzContext.User.GetName())
	}

	return s.cfg.Writer.CreateScopedRole(ctx, req)
}

// CreateScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error) {
	if err := scopes.AssertFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to create scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to create scoped role assignments", authzContext.User.GetName())
	}

	if req.GetAssignment() == nil {
		return nil, trace.BadParameter("missing assignment in request")
	}

	if assignment := req.GetAssignment(); assignment.GetMetadata() == nil {
		assignment.Metadata = &headerv1.Metadata{}
	}

	if req.GetAssignment().GetMetadata().GetName() == "" {
		req.GetAssignment().GetMetadata().Name = uuid.New().String()
	}

	if err := scopedaccess.StrongValidateAssignment(req.GetAssignment()); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetRoleRevisions() == nil {
		req.RoleRevisions = make(map[string]string)
	}

	for _, subAssignment := range req.GetAssignment().GetSpec().GetAssignments() {
		rsp, err := s.cfg.Reader.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: subAssignment.GetRole(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if revision, ok := req.GetRoleRevisions()[subAssignment.GetRole()]; ok {
			if revision != rsp.GetRole().GetMetadata().GetRevision() {
				return nil, trace.CompareFailed("role %q revision %q does not match expected revision %q",
					subAssignment.GetRole(), rsp.GetRole().GetMetadata().GetRevision(), revision)
			}
		} else {
			// If the revision is not specified, use the current revision of the role.
			req.RoleRevisions[subAssignment.GetRole()] = rsp.GetRole().GetMetadata().GetRevision()
		}

		// TODO(fspmarshall): implement validation of role assignment access-controls at this step.
	}

	return s.cfg.Writer.CreateScopedRoleAssignment(ctx, req)
}

// DeleteScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to delete a scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped roles", authzContext.User.GetName())
	}

	return s.cfg.Writer.DeleteScopedRole(ctx, req)
}

// DeleteScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to delete a scoped role assignment", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to delete a scoped role assignment", authzContext.User.GetName())
	}

	return s.cfg.Writer.DeleteScopedRoleAssignment(ctx, req)
}

// GetScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to get scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped roles", authzContext.User.GetName())
	}

	return s.cfg.Reader.GetScopedRole(ctx, req)
}

// GetScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to get scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to get scoped role assignments", authzContext.User.GetName())
	}

	return s.cfg.Reader.GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to list scoped role assignments", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped role assignments", authzContext.User.GetName())
	}

	return s.cfg.Reader.ListScopedRoleAssignments(ctx, req)
}

// ListScopedRoles implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to list scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to list scoped roles", authzContext.User.GetName())
	}

	return s.cfg.Reader.ListScopedRoles(ctx, req)
}

// UpdateScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	if err := scopes.AssertFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authzContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission update scoped roles", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission update scoped roles", authzContext.User.GetName())
	}

	return s.cfg.Writer.UpdateScopedRole(ctx, req)
}
