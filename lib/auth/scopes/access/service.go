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
	ScopedAuthorizer authz.ScopedAuthorizer
	Reader           services.CachedScopedAccessReader
	Writer           services.ScopedAccessWriter
	Logger           *slog.Logger
}

// CheckAndSetDefaults checks the config for missing parameters and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.ScopedAuthorizer == nil {
		return trace.BadParameter("missing ScopedAuthorizer in scoped access grpc service config")
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

	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzContext.CheckerContext.Decision(ctx, req.GetRole().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRole, types.VerbCreate)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to create scoped roles in the requested scope",
			//"user", authzContext.User.GetName(),
			"scope", req.GetRole().GetScope())
		return nil, trace.Wrap(err)
	}

	return s.cfg.Writer.CreateScopedRole(ctx, req)
}

// CreateScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) CreateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error) {
	if err := scopes.AssertFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
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

	if err := authzContext.CheckerContext.Decision(ctx, req.GetAssignment().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbCreate)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to create scoped role assignments in the requested scope",
			//"user", authzContext.User.GetName(),
			"scope", req.GetAssignment().GetScope())
		return nil, trace.Wrap(err)
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

		// XXX: we're kind of side-stepping the question of what, if any, per-role policies should be enforced
		// by currently requiring that all assignments only assign roles from the same scope as part of the
		// backend validation logic. if/when we lift that restriction, we'll need to revisit this logic and
		// decide what, if any, additional access-control checks may be required when an assignment references
		// a role from a different scope. the current thinking is that we will allow assignments to reference
		// roles in parent scopes *but* said assignments will not be able to introduce conflicts in modification
		// of said parent roles. this is consistent with the scopes security model but has the downside of requiring
		// us to change/relax role modification restrictions and possibly introduce a means of automated cleanup of
		// dangling/malformed assignments.
	}

	return s.cfg.Writer.CreateScopedRoleAssignment(ctx, req)
}

// DeleteScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRole, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// load the role so we can determine the resource scope
	grsp, err := s.cfg.Reader.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: req.GetName(),
	})
	if err != nil {
		if trace.IsNotFound(err) {
			// this API deliberately does not distinguish between kinds of concurrent modification
			// in its error kinds.
			return nil, trace.CompareFailed("scoped role %q has been concurrently modified and/or deleted", req.GetName())
		}
		return nil, trace.Wrap(err)
	}

	// if a revision has been asserted, it must match the revision of the resource we are going to use for
	// access-control checks.
	if rev := req.GetRevision(); rev != "" && rev != grsp.GetRole().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role %q has been concurrently modified", req.GetName())
	}

	// evaluate the access to the role based on its scope
	if err := authzContext.CheckerContext.Decision(ctx, grsp.GetRole().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRole, types.VerbDelete)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to delete scoped roles in the requested scope",
			//"user", authzContext.User.GetName(),
			"scope", grsp.GetRole().GetScope(),
			"role", req.GetName(),
			"error", err,
		)
		return nil, trace.Wrap(err)
	}

	// set the revision to the current revision to prevent deletion in the event of concurrent modification
	// that might invalidate the access-control checks we just performed.
	req.Revision = grsp.GetRole().GetMetadata().GetRevision()

	return s.cfg.Writer.DeleteScopedRole(ctx, req)
}

// DeleteScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// load the assignment so we can determine the resource scope
	grsp, err := s.cfg.Reader.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: req.GetName(),
	})
	if err != nil {
		if trace.IsNotFound(err) {
			// this API deliberately does not distinguish between kinds of concurrent modification
			// in its error kinds.
			return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified and/or deleted", req.GetName())
		}
		return nil, trace.Wrap(err)
	}

	// if a revision has been asserted, it must match the revision of the resource we are going to use for
	// access-control checks.
	if rev := req.GetRevision(); rev != "" && rev != grsp.GetAssignment().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified", req.GetName())
	}

	// evaluate the access to the assignment based on its scope
	if err := authzContext.CheckerContext.Decision(ctx, grsp.GetAssignment().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbDelete)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to delete scoped role assignments in the requested scope",
			//"user", authzContext.User.GetName(),
			"scope", grsp.GetAssignment().GetScope(),
			"assignment", req.GetName(),
			"error", err,
		)
		return nil, trace.Wrap(err)
	}

	// set the revision to the current revision to prevent deletion in the event of concurrent modification
	// that might invalidate the access-control checks we just performed.
	req.Revision = grsp.GetAssignment().GetMetadata().GetRevision()

	return s.cfg.Writer.DeleteScopedRoleAssignment(ctx, req)
}

// GetScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRole, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// load the role so we can determine the resource scope
	preAuthzRsp, err := s.cfg.Reader.GetScopedRole(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// evaluate the access to the role based on its scope
	if err := authzContext.CheckerContext.Decision(ctx, preAuthzRsp.GetRole().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRole, types.VerbRead)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to read scoped role",
			//"user", authzContext.User.GetName(),
			"scope", preAuthzRsp.GetRole().GetScope(),
			"role", req.GetName(),
			"error", err,
		)
		return nil, trace.Wrap(err)
	}

	// TODO(fspmarshall/scopes): we likely want to add an exception here to allow users to view the roles that they
	// are assigned. though we may want to have such an exception be an opt-in mode to avoid complicating local administration.

	// return of the pre-authz response is safe because we have now confirmed the user has access to its contents.
	return preAuthzRsp, nil
}

// GetScopedRoleAssignment implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// load the assignment so we can determine the resource scope
	preAuthzRsp, err := s.cfg.Reader.GetScopedRoleAssignment(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// evaluate the access to the assignment based on its scope
	if err := authzContext.CheckerContext.Decision(ctx, preAuthzRsp.GetAssignment().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbRead)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to read scoped role assignment",
			//"user", authzContext.User.GetName(),
			"scope", preAuthzRsp.GetAssignment().GetScope(),
			"assignment", req.GetName(),
			"error", err,
		)
		return nil, trace.Wrap(err)
	}

	// TODO(fspmarshall/scopes): we must add an exception here to allow users to view the assignments that they
	// are assigned. we also need to explicitly support "escaping" the scope pin while doing this in order for users
	// to be able to discover privileges in ther scopes. we will want to have such an exception be an opt-in mode.

	// return of the pre-authz response is safe because we have now confirmed the user has access to its contents.
	return preAuthzRsp, nil
}

// ListScopedRoleAssignments implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// list scoped role assignments with a filter that only passes assignments the user has access to.
	rsp, err := s.cfg.Reader.ListScopedRoleAssignmentsWithFilter(ctx, req, func(assignment *scopedaccessv1.ScopedRoleAssignment) bool {
		err := authzContext.CheckerContext.Decision(ctx, assignment.GetScope(), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRoleAssignment, types.VerbRead, types.VerbList)
		})
		return err == nil
	})
	return rsp, trace.Wrap(err)
}

// ListScopedRoles implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(scopedaccess.KindScopedRole, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// list scoped roles with a filter that only passes roles the user has access to.
	rsp, err := s.cfg.Reader.ListScopedRolesWithFilter(ctx, req, func(role *scopedaccessv1.ScopedRole) bool {
		err := authzContext.CheckerContext.Decision(ctx, role.GetScope(), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRole, types.VerbRead, types.VerbList)
		})
		return err == nil
	})

	return rsp, trace.Wrap(err)
}

// UpdateScopedRole implements [scopedaccessv1.ScopedRoleServiceServer].
func (s *Server) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	if err := scopes.AssertFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeSplit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// XXX: the sanity of this access-control check is dependant on the fact that the backend implementation currently
	// does not permit changing the scope of a role during an update. If that ever changes, this logic would be invalid.
	if err := authzContext.CheckerContext.Decision(ctx, req.GetRole().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(scopedaccess.KindScopedRole, types.VerbUpdate)
	}); err != nil {
		s.cfg.Logger.WarnContext(ctx, "user does not have permission to update scoped roles in the requested scope",
			//"user", authzContext.User.GetName(),
			"scope", req.GetRole().GetScope())
		return nil, trace.Wrap(err)
	}

	return s.cfg.Writer.UpdateScopedRole(ctx, req)
}
