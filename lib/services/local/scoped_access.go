/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"crypto/rand"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// scoped role and assignment state is modeled with the following key ranges:
//
//  - `/scoped_role/role/<role-name>`             (the location of the role resource, stored at author-chosen name)
//  - `/scoped_role/assignment/<assignment-name>` (the assignment resource itself, always stored at a random UUID)
//  - `/scoped_role/user_lock/<username>`         (a value that is randomized each time associated user's assignments are modified)
//  - `/scoped_role/role_lock/<role-name>`        (a value that is randomized each time associated role's assignments are modified)
//
// These four key ranges allow for efficient management of roles and assignmments atomically. Assignments are stored homogenously,
// but the provided lock values make it easy for backend operations to assert that the assignments related to a given user/role
// are not concurrently changed, indepdnent of the total number of assignments or the number of roles they effect (each assignment
// may assign multiple roles). Cleanup of role locks is the responsibility of the DeleteScopedRole operation, and cleanup of user locks
// is the responsibility of the DeleteScopedRoleAssignment operation.
//
// NOTE: this model does not provide means of making one assignment invalidate another (e.g. in the case of OIDC assignments,
// for which only one should be valid at a time), and does not invalidate assignments on user deletion.

const (
	scopedRolePrefix              = "scoped_role"
	scopedRoleRoleComponent       = "role"
	scopedRoleAssignmentComponent = "assignment"
	userAssignmentLockComponent   = "user_lock"
	roleAssignmentLockComponent   = "role_lock"
)

const (
	scopedAccessListPrefix       = "scoped_access_list"
	scopedAccessListMemberPrefix = "scoped_access_list_member"
)

// ScopedAccessService manages backend state for the ScopedRole and ScopedRoleAssignment types.
type ScopedAccessService struct {
	bk     backend.Backend
	logger *slog.Logger
}

// NewScopedAccessService creates a new ScopedAccessService for the specified backend.
func NewScopedAccessService(bk backend.Backend) *ScopedAccessService {
	return &ScopedAccessService{
		bk:     bk,
		logger: slog.With(teleport.ComponentKey, "scopedrole"),
	}
}

func (s *ScopedAccessService) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped role name in get request")
	}

	item, err := s.bk.Get(ctx, scopedRoleKey(req.GetName()))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped role %q not found", req.GetName())
		}
		return nil, trace.Wrap(err)
	}

	role, err := scopedRoleFromItem(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scopedaccess.WeakValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.GetScopedRoleResponse{
		Role: role,
	}, nil
}

// ListScopedRoles returns a paginated list of scoped roles.
// NOTE: this method is only used by local auth caches, and doesn't implement sorting, filtering, or pagination.
func (s *ScopedAccessService) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	if req.GetResourceScope() != nil {
		return nil, trace.NotImplemented("filtering by resource scope is not implemented for direct backend scoped role reads")
	}

	if req.GetAssignableScope() != nil {
		return nil, trace.NotImplemented("filtering by assignable scope is not implemented for direct backend scoped role reads")
	}

	if req.GetPageToken() != "" {
		return nil, trace.NotImplemented("pagination is not implemented for direct backend scoped role reads")
	}

	var out []*scopedaccessv1.ScopedRole
	for role, err := range s.StreamScopedRoles(ctx) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, role)
	}

	return &scopedaccessv1.ListScopedRolesResponse{
		Roles: out,
	}, nil
}

// StreamScopedRoles returns a stream of all scoped roles in the backend. Malformed roles are skipped. Returned roles
// have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoles(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedRole] {
	return func(yield func(*scopedaccessv1.ScopedRole, error) bool) {
		startKey := scopedRoleKey("")
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   backend.RangeEnd(startKey),
		}

		for item, err := range s.bk.Items(ctx, params) {
			if err != nil {
				// backend errors terminate the stream
				yield(nil, trace.Wrap(err))
				return
			}

			role, err := scopedRoleFromItem(&item)
			if err != nil {
				// per-role errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped role due to unmarshal error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if err := scopedaccess.WeakValidateRole(role); err != nil {
				// per-role errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped role due to validation error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if !yield(role, nil) {
				return
			}
		}
	}
}

func (s *ScopedAccessService) CreateScopedRole(ctx context.Context, req *scopedaccessv1.CreateScopedRoleRequest) (*scopedaccessv1.CreateScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in create request")
	}

	if err := scopedaccess.StrongValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := scopedRoleToItem(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// we make efforts elsewhere to ensure that roles cannot be deleted s.t. they leave behind dangling assignments,
	// but it is best to be absolutely certain about that.
	lockItem, err := s.bk.Get(ctx, roleAssignmentLockKey(role.GetMetadata().GetName()))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		lockItem = nil
	}

	lockCondition := backend.NotExists()
	if lockItem != nil {
		lockCondition = backend.Revision(lockItem.Revision)
		for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, subAssignment := range assignment.GetSpec().GetAssignments() {
				if subAssignment.GetRole() != role.GetMetadata().GetName() {
					continue
				}

				// an assignment already exists referencing this role, we need to check if that is because
				// a role with this name exists, or because the assignment is dangling.
				_, err = s.bk.Get(ctx, scopedRoleKey(role.GetMetadata().GetName()))
				if err != nil {
					if !trace.IsNotFound(err) {
						return nil, trace.Wrap(err)
					}
					// this is a dangling assignment, we need to return an error
					return nil, trace.CompareFailed("cannot create scoped role %q while extant assignment %q references it", role.GetMetadata().GetName(), assignment.GetMetadata().GetName())
				}
				return nil, trace.CompareFailed("scoped role %q already exists", role.GetMetadata().GetName())
			}
		}
	}

	revision, err := s.bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
		{
			Key:       roleAssignmentLockKey(role.GetMetadata().GetName()),
			Condition: lockCondition,
			Action:    backend.Nop(), // assignments update the lock, roles just assert that it is unchanged
		},
	})
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped role %q or an associated assignment already exist", role.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, revision),
	}, nil
}

func (s *ScopedAccessService) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in update request")
	}

	if err := scopedaccess.StrongValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	extant, err := s.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: role.GetMetadata().GetName(),
	})
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return nil, trace.CompareFailed("scoped role %q was deleted", role.GetMetadata().GetName())
	}

	if role.GetMetadata().GetRevision() != "" && role.GetMetadata().GetRevision() != extant.GetRole().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role %q has been concurrently modified", role.GetMetadata().GetName())
	}

	// disallow change of resource scope via update. use of scopes.Compare directly is generally discouraged,
	// but that is due to ease of misuse, which isn't really a concern for a simple equivalence check.
	if scopes.Compare(role.GetScope(), extant.GetRole().GetScope()) != scopes.Equivalent {
		// XXX: the current implementation of our access-control logic relies upon this invarient being enforced. if we ever
		// relax this restriction here we *must* first modify the outer access-control logic to understand the concept of
		// scope changing and correctly validate the transition.
		return nil, trace.BadParameter("cannot modify the resource scope of scoped role %q (%q -> %q)", role.GetMetadata().GetName(), extant.GetRole().GetScope(), role.GetScope())
	}

	// acquire the assignment lock and verify that the update doesn't validate any extant assignments
	lockItem, err := s.bk.Get(ctx, roleAssignmentLockKey(role.GetMetadata().GetName()))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		lockItem = nil
	}

	lockCondition := backend.NotExists()
	if lockItem != nil {
		lockCondition = backend.Revision(lockItem.Revision)
		for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, subAssignment := range assignment.GetSpec().GetAssignments() {
				if subAssignment.GetRole() != role.GetMetadata().GetName() {
					continue
				}

				if !scopedaccess.RoleIsAssignableAtScope(extant.GetRole(), subAssignment.GetScope()) {
					// theoretically, we prevent broken assignments. in practice, its best to
					// assume they may exist and to not allow them to prevent an otherwsie
					// valid update. We will still force all broken assignments to be
					// removed at the time of role deletion.
					continue
				}

				if !scopedaccess.RoleIsAssignableAtScope(role, subAssignment.GetScope()) {
					return nil, trace.BadParameter("update of scoped role %q would invalidate assignment %q which assigns it to user %q at scope %q", role.GetMetadata().GetName(), assignment.GetMetadata().GetName(), assignment.GetSpec().GetUser(), subAssignment.GetScope())
				}
			}
		}
	}

	item, err := scopedRoleToItem(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.Revision(item.Revision),
			Action:    backend.Put(item),
		},
		{
			Key:       roleAssignmentLockKey(role.GetMetadata().GetName()),
			Condition: lockCondition,
			Action:    backend.Nop(),
		},
	})
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped role %q or an associated assignment was concurrently modified", role.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.UpdateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, revision),
	}, nil
}

func (s *ScopedAccessService) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	roleName := req.GetName()
	if roleName == "" {
		return nil, trace.BadParameter("missing scoped role name in delete request")
	}

	lockItem, err := s.bk.Get(ctx, roleAssignmentLockKey(roleName))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		lockItem = nil
	}

	lockCondition := backend.NotExists()
	if lockItem != nil {
		lockCondition = backend.Revision(lockItem.Revision)
	}

	// now that we have a lock condition established, we can read all assignments with a "happens after" relationship
	// to the current lock value and verify that no assignments target this role.
	for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, subAssignment := range assignment.GetSpec().GetAssignments() {
			if subAssignment.GetRole() == roleName {
				return nil, trace.CompareFailed("cannot delete scoped role %q while assignment %q assigns it to a user", roleName, assignment.GetMetadata().GetName())
			}
		}
	}

	roleCondition := backend.Exists()
	if rev := req.GetRevision(); rev != "" {
		roleCondition = backend.Revision(rev)
	}

	// atomically delete the role and its associated assignment lock while asserting that no assignments
	// have been concurrently applied that target this role.
	_, err = s.bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       scopedRoleKey(roleName),
			Condition: roleCondition,
			Action:    backend.Delete(),
		},
		{
			Key:       roleAssignmentLockKey(roleName),
			Condition: lockCondition,
			Action:    backend.Delete(),
		},
	})

	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped role %q has been concurrently modified and/or assigned", roleName)
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.DeleteScopedRoleResponse{}, nil
}

func (s *ScopedAccessService) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in get request")
	}

	item, err := s.bk.Get(ctx, scopedRoleAssignmentKey(assignmentName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped role assignment %q not found", assignmentName)
		}
		return nil, trace.Wrap(err)
	}

	assignment, err := scopedRoleAssignmentFromItem(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scopedaccess.WeakValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.GetScopedRoleAssignmentResponse{
		Assignment: assignment,
	}, nil
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
// NOTE: this method is only used by local auth caches, and doesn't implement sorting, filtering, or pagination.
func (s *ScopedAccessService) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	if req.GetResourceScope() != nil {
		return nil, trace.NotImplemented("filtering by resource scope is not implemented for direct backend scoped role assignment reads")
	}

	if req.GetAssignedScope() != nil {
		return nil, trace.NotImplemented("filtering by assigned scope is not implemented for direct backend scoped role assignment reads")
	}

	if req.GetPageToken() != "" {
		return nil, trace.NotImplemented("pagination is not implemented for direct backend scoped role assignment reads")
	}

	var out []*scopedaccessv1.ScopedRoleAssignment
	for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, assignment)
	}

	return &scopedaccessv1.ListScopedRoleAssignmentsResponse{
		Assignments: out,
	}, nil
}

// StreamScopedRoleAssignments returns a stream of all scoped role assignments in the backend. Malformed assignments are skipped.
// Returned assignments have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoleAssignments(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedRoleAssignment] {
	return func(yield func(*scopedaccessv1.ScopedRoleAssignment, error) bool) {
		startKey := scopedRoleAssignmentKey("")
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   backend.RangeEnd(startKey),
		}

		for item, err := range s.bk.Items(ctx, params) {
			if err != nil {
				// backend errors terminate the stream
				yield(nil, trace.Wrap(err))
				return
			}

			assignment, err := scopedRoleAssignmentFromItem(&item)
			if err != nil {
				// per-assignment errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped role assignment due to unmarshal error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if err := scopedaccess.WeakValidateAssignment(assignment); err != nil {
				// per-assignment errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped role assignment due to validation error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if !yield(assignment, nil) {
				return
			}
		}
	}
}

func (s *ScopedAccessService) CreateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error) {
	assignment := req.GetAssignment()
	if assignment == nil {
		return nil, trace.BadParameter("missing scoped role assignment in create request")
	}

	if err := scopedaccess.StrongValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	// independently enforce the max number of roles per assignment limit here since not all validation
	// may necessarily enforce it, but it is a hard-limit for the backend impl.
	if len(assignment.GetSpec().GetAssignments()) > scopedaccess.MaxRolesPerAssignment {
		return nil, trace.BadParameter("scoped role assignment resource %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), scopedaccess.MaxRolesPerAssignment)
	}

	item, err := scopedRoleAssignmentToItem(assignment)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set up conditional actions for assignment and user lock
	condacts := []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
		{
			Key:       userAssignmentLockKey(assignment.GetSpec().GetUser()),
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: newUserAssignmentLockVal(assignment.GetSpec().GetUser()),
			}),
		},
	}

	assertedRoles := make(map[string]struct{})

	// set up conditional actions for each assigned role lock
	for _, subAssignment := range assignment.GetSpec().GetAssignments() {
		// operation must verify that all associated roles have not been concurrently modified
		// as such modification could theoretically invalidate prior access-control checks.
		roleRevision, ok := req.GetRoleRevisions()[subAssignment.GetRole()]
		if !ok {
			// this is a bug in the API layer, we should never be missing a role revision as it should be
			// filled in with the revision of the role used for the access-control check.
			return nil, trace.BadParameter("missing role revision for role %q in backend create (this is a bug)", subAssignment.GetRole())
		}

		rrsp, err := s.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: subAssignment.GetRole(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if rrsp.GetRole().GetMetadata().GetRevision() != roleRevision {
			return nil, trace.CompareFailed("scoped role %q has been concurrently modified", subAssignment.GetRole())
		}

		// verify that the role is scoped to the same resource scope as the assignment itself
		// NOTE: this restriction may eventually be relaxed in favor of something more flexible,
		// but as of right now we haven't decided what that should look like.
		if scopes.Compare(rrsp.GetRole().GetScope(), assignment.GetScope()) != scopes.Equivalent {
			return nil, trace.BadParameter("role %q is not scoped to the same resource scope as assignment %q (%q -> %q)", subAssignment.GetRole(), assignment.GetMetadata().GetName(), rrsp.GetRole().GetScope(), assignment.GetScope())
		}

		// verify that the role is assignable at the specified scope
		if !scopedaccess.RoleIsAssignableAtScope(rrsp.GetRole(), subAssignment.GetScope()) {
			return nil, trace.BadParameter("scoped role %q is not configured to be assignable at scope %q", subAssignment.GetRole(), subAssignment.GetScope())
		}

		if _, ok := assertedRoles[subAssignment.GetRole()]; ok {
			// a previous sub-assignment already caused us to assert the revision
			// of this role, we can skip the assertion/lock update step.
			continue
		}

		// assert that role is unchanged and modify associated role lock so that role modifications can
		// detect concurrent modifications to their assignments.
		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       scopedRoleKey(subAssignment.GetRole()),
				Condition: backend.Revision(roleRevision),
				Action:    backend.Nop(),
			},
			{
				Key:       roleAssignmentLockKey(subAssignment.GetRole()),
				Condition: backend.Whatever(),
				Action: backend.Put(backend.Item{
					Value: newRoleAssignmentLockVal(subAssignment.GetRole()),
				}),
			},
		}...)

		assertedRoles[subAssignment.GetRole()] = struct{}{}
	}

	revision, err := s.bk.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			// return a general concurrent-modification error since it isn't clear which condition faile
			return nil, trace.CompareFailed("scoped role assignment %q failed due to concurrent modification of associated resources", assignment.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedRoleAssignmentResponse{
		Assignment: scopedRoleAssignmentWithRevision(assignment, revision),
	}, nil
}

func (s *ScopedAccessService) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in delete request")
	}

	extant, err := s.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: assignmentName,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.CompareFailed("scoped role assignment %q was concurrently delete", assignmentName)
		}
		return nil, trace.Wrap(err)
	}

	if rev := req.GetRevision(); rev != "" && rev != extant.Assignment.GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified", assignmentName)
	}

	// check to see if we have a lock on the user. if so, we need to check to see if we're the last assignment
	// relying on the lock. if we are, we can delete it.
	userLockItem, err := s.bk.Get(ctx, userAssignmentLockKey(extant.Assignment.GetSpec().GetUser()))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		userLockItem = nil
	}

	// start with initial condition assuming non-existence. note that this really should never happen unless
	// we have a bug somewhere else, but there isn't really a downside to being resilient to it.
	userLockCondition := backend.NotExists()
	userLockAction := backend.Nop()
	if userLockItem != nil {
		userLockCondition = backend.Revision(userLockItem.Revision)
		userLockAction = backend.Put(backend.Item{
			Value: newUserAssignmentLockVal(extant.Assignment.GetSpec().GetUser()),
		})

		// check to see if we're the last assignment relying on the user lock. if so, we should delete it.
		var hasOtherAssignments bool
		for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if assignment.GetSpec().GetUser() != extant.Assignment.GetSpec().GetUser() {
				// skip assignments related to other users
				continue
			}
			if assignment.GetMetadata().GetName() == extant.Assignment.GetMetadata().GetName() {
				// skip the assignment we're currently deleting
				continue
			}

			// found another assignment for the same user
			hasOtherAssignments = true
			break
		}

		if !hasOtherAssignments {
			// no other assignments for this user, we can delete the lock
			userLockAction = backend.Delete()
		}
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       scopedRoleAssignmentKey(assignmentName),
			Condition: backend.Revision(extant.Assignment.GetMetadata().GetRevision()),
			Action:    backend.Delete(),
		},
		{
			Key:       userAssignmentLockKey(extant.Assignment.GetSpec().GetUser()),
			Condition: userLockCondition,
			Action:    userLockAction,
		},
	}

	lockedRoles := make(map[string]struct{})

	for _, subAssignment := range extant.Assignment.GetSpec().GetAssignments() {

		if _, ok := lockedRoles[subAssignment.GetRole()]; ok {
			// a previous sub-assignment already caused us to update the lock
			// of this role, we can skip this update step.
			continue
		}

		// operation must modify all associated role locks to ensure that role operations can
		// efficiently assert that no assigment related to the role has changed.
		condacts = append(condacts, backend.ConditionalAction{
			Key:       roleAssignmentLockKey(subAssignment.GetRole()),
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: newRoleAssignmentLockVal(subAssignment.GetRole()),
			}),
		})

		lockedRoles[subAssignment.GetRole()] = struct{}{}
	}

	if _, err := s.bk.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped role assignment %q or another related assignment was concurrently modified", assignmentName)
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.DeleteScopedRoleAssignmentResponse{}, nil
}

func (s *ScopedAccessService) CreateScopedAccessList(ctx context.Context, req *scopedaccessv1.CreateScopedAccessListRequest) (*scopedaccessv1.CreateScopedAccessListResponse, error) {
	list := req.GetList()
	if list == nil {
		return nil, trace.BadParameter("missing scoped access list in create request")
	}

	if err := scopedaccess.StrongValidateAccessList(list); err != nil {
		return nil, trace.Wrap(err)
	}

	condacts, err := s.accessListGrantConditions(ctx, list)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := scopedAccessListToItem(list)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	condacts = append(condacts, backend.ConditionalAction{
		Key:       scopedAccessListKey(list.GetMetadata().GetName()),
		Condition: backend.NotExists(),
		Action:    backend.Put(item),
	})

	revision, err := s.bk.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped access list %q already exists or a granted role was concurrently modified", list.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedAccessListResponse{
		List: scopedAccessListWithRevision(list, revision),
	}, nil
}

func (s *ScopedAccessService) UpdateScopedAccessList(ctx context.Context, req *scopedaccessv1.UpdateScopedAccessListRequest) (*scopedaccessv1.UpdateScopedAccessListResponse, error) {
	list := req.GetList()
	if list == nil {
		return nil, trace.BadParameter("missing scoped access list in update request")
	}

	if err := scopedaccess.StrongValidateAccessList(list); err != nil {
		return nil, trace.Wrap(err)
	}

	extant, err := s.GetScopedAccessList(ctx, &scopedaccessv1.GetScopedAccessListRequest{
		Name: list.GetMetadata().GetName(),
	})
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return nil, trace.CompareFailed("scoped access list %q was deleted", list.GetMetadata().GetName())
	}

	if list.GetMetadata().GetRevision() != "" && list.GetMetadata().GetRevision() != extant.GetList().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped access list %q has been concurrently modified", list.GetMetadata().GetName())
	}

	// disallow change of resource scope via update. use of scopes.Compare directly is generally discouraged,
	// but that is due to ease of misuse, which isn't really a concern for a simple equivalence check.
	if scopes.Compare(list.GetScope(), extant.GetList().GetScope()) != scopes.Equivalent {
		// XXX: the current implementation of our access-control logic relies upon this invarient being enforced. if we ever
		// relax this restriction here we *must* first modify the outer access-control logic to understand the concept of
		// scope changing and correctly validate the transition.
		return nil, trace.BadParameter("cannot modify the resource scope of scoped access list %q (%q -> %q)", list.GetMetadata().GetName(), extant.GetList().GetScope(), list.GetScope())
	}

	condacts, err := s.accessListGrantConditions(ctx, list)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := scopedAccessListToItem(list)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	condacts = append(condacts, backend.ConditionalAction{
		Key:       scopedAccessListKey(list.GetMetadata().GetName()),
		Condition: backend.Revision(extant.GetList().GetMetadata().GetRevision()),
		Action:    backend.Put(item),
	})

	revision, err := s.bk.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped access list %q or a granted role was concurrently modified", list.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.UpdateScopedAccessListResponse{
		List: scopedAccessListWithRevision(list, revision),
	}, nil
}

func (s *ScopedAccessService) accessListGrantConditions(ctx context.Context, list *scopedaccessv1.ScopedAccessList) ([]backend.ConditionalAction, error) {
	var condacts []backend.ConditionalAction
	for _, scopedRoleGrant := range list.GetSpec().GetGrants().GetScopedRoles() {
		resp, err := s.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: scopedRoleGrant.GetRole(),
		})
		if err != nil {
			return nil, trace.Wrap(err, "verifying scoped role granted by scoped access list")
		}
		role := resp.GetRole()
		if scopes.Compare(list.GetScope(), role.GetScope()) == scopes.Descendant {
			return nil, trace.BadParameter("scoped access list %q in scope %q cannot assign role %q defined descendant scope %q",
				list.GetMetadata().GetName(), list.GetScope(),
				role.GetMetadata().GetName(), role.GetScope())
		}
		if !scopedaccess.RoleIsAssignableAtScope(role, scopedRoleGrant.GetScope()) {
			return nil, trace.BadParameter("scoped role %s is not assignable at scope %s", role.GetMetadata().GetName(), scopedRoleGrant.GetScope())
		}
		// TODO(nklaassen): a ConditionalAction on every single role grant is
		// probably too much, probably need to relax this.
		condacts = append(condacts, backend.ConditionalAction{
			Key:       scopedRoleKey(role.GetMetadata().GetName()),
			Condition: backend.Revision(role.GetMetadata().GetRevision()),
			Action:    backend.Nop(),
		})
	}
	return condacts, nil
}

func (s *ScopedAccessService) GetScopedAccessList(ctx context.Context, req *scopedaccessv1.GetScopedAccessListRequest) (*scopedaccessv1.GetScopedAccessListResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped access list name in get request")
	}

	item, err := s.bk.Get(ctx, scopedAccessListKey(req.GetName()))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped access list %q not found", req.GetName())
		}
		return nil, trace.Wrap(err)
	}

	list, err := scopedAccessListFromItem(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scopedaccess.WeakValidateAccessList(list); err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.GetScopedAccessListResponse{
		List: list,
	}, nil
}

// StreamScopedAccessLists returns a stream of all scoped access lists in the
// backend. Malformed access lists are skipped. Returned access lists have had
// weak validation applied.
func (s *ScopedAccessService) StreamScopedAccessLists(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedAccessList] {
	return func(yield func(*scopedaccessv1.ScopedAccessList, error) bool) {
		startKey := scopedAccessListKey("")
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   backend.RangeEnd(startKey),
		}

		for item, err := range s.bk.Items(ctx, params) {
			if err != nil {
				// backend errors terminate the stream
				yield(nil, trace.Wrap(err))
				return
			}

			list, err := scopedAccessListFromItem(&item)
			if err != nil {
				// per-list errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped access list due to unmarshal error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if err := scopedaccess.WeakValidateAccessList(list); err != nil {
				// per-list errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped access list due to validation error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if !yield(list, nil) {
				return
			}
		}
	}
}

func (s *ScopedAccessService) DeleteScopedAccessList(ctx context.Context, req *scopedaccessv1.DeleteScopedAccessListRequest) (*scopedaccessv1.DeleteScopedAccessListResponse, error) {
	listName := req.GetName()
	if listName == "" {
		return nil, trace.BadParameter("missing scoped access list name in delete request")
	}

	// TODO(nklaassen): make sure no memberships reference this list.

	action := backend.ConditionalAction{
		Key:       scopedAccessListKey(listName),
		Condition: backend.Exists(),
		Action:    backend.Delete(),
	}
	if rev := req.GetRevision(); rev != "" {
		action.Condition = backend.Revision(rev)
	}

	if _, err := s.bk.AtomicWrite(ctx, []backend.ConditionalAction{action}); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped access list %q has been concurrently modified and/or assigned", listName)
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.DeleteScopedAccessListResponse{}, nil
}

func (s *ScopedAccessService) CreateScopedAccessListMember(ctx context.Context, req *scopedaccessv1.CreateScopedAccessListMemberRequest) (*scopedaccessv1.CreateScopedAccessListMemberResponse, error) {
	member := req.GetMember()
	if member == nil {
		return nil, trace.BadParameter("missing scoped access list member in create request")
	}

	if err := scopedaccess.StrongValidateAccessListMember(member); err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := s.GetScopedAccessList(ctx, &scopedaccessv1.GetScopedAccessListRequest{
		Name: member.GetSpec().GetAccessList(),
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("scoped access list member %q references scoped access list %q which does not exist",
				member.GetMetadata().GetName(), member.GetSpec().GetAccessList())
		}
		return nil, trace.Wrap(err)
	}
	list := listResp.GetList()

	if scopes.Compare(list.GetScope(), member.GetScope()) != scopes.Equivalent {
		return nil, trace.BadParameter("access list member %q is not in the same scope as access list %q",
			member.GetMetadata().GetName(), list.GetMetadata().GetName())
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       scopedAccessListKey(list.GetMetadata().GetName()),
			Condition: backend.Revision(list.GetMetadata().GetRevision()),
			Action:    backend.Nop(),
		},
	}

	switch member.GetSpec().GetMembershipKind() {
	case scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_UNSPECIFIED, scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_USER:
		// No validation for users, the user resource may not exist until login.
	case scopedaccessv1.MembershipKind_MEMBERSHIP_KIND_LIST:
		assignedListResp, err := s.GetScopedAccessList(ctx, &scopedaccessv1.GetScopedAccessListRequest{
			Name: member.GetSpec().GetName(),
		})
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.BadParameter("scoped access list member %q assigns scoped access list %q which does not exist",
					member.GetMetadata().GetName(), member.GetSpec().GetName())
			}
			return nil, trace.Wrap(err)
		}
		assignedList := assignedListResp.GetList()
		if scopes.Compare(list.GetScope(), assignedList.GetScope()) == scopes.Descendant {
			return nil, trace.BadParameter("scoped access lists cannot have member lists defined in descendant scopes")
		}
		condacts = append(condacts, backend.ConditionalAction{
			Key:       scopedAccessListKey(assignedList.GetMetadata().GetName()),
			Condition: backend.Revision(assignedList.GetMetadata().GetRevision()),
			Action:    backend.Nop(),
		})
	default:
		return nil, trace.BadParameter("unhandled scoped access list membership kind %q", member.GetSpec().GetMembershipKind())
	}

	item, err := scopedAccessListMemberToItem(member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	condacts = append(condacts, backend.ConditionalAction{
		Key:       scopedAccessListMemberKey(member.GetMetadata().GetName()),
		Condition: backend.NotExists(),
		Action:    backend.Put(item),
	})

	revision, err := s.bk.AtomicWrite(ctx, condacts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedAccessListMemberResponse{
		Member: scopedAccessListMemberWithRevision(member, revision),
	}, nil
}

func (s *ScopedAccessService) GetScopedAccessListMember(ctx context.Context, req *scopedaccessv1.GetScopedAccessListMemberRequest) (*scopedaccessv1.GetScopedAccessListMemberResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped access list member name in get request")
	}

	item, err := s.bk.Get(ctx, scopedAccessListMemberKey(req.GetName()))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped access list member %q not found", req.GetName())
		}
		return nil, trace.Wrap(err)
	}

	member, err := scopedAccessListMemberFromItem(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scopedaccess.WeakValidateAccessListMember(member); err != nil {
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.GetScopedAccessListMemberResponse{
		Member: member,
	}, nil
}

// StreamScopedAccessListMembers returns a stream of all scoped access list
// members in the backend. Malformed access list members are skipped. Returned
// access list members have had weak validation applied.
func (s *ScopedAccessService) StreamScopedAccessListMembers(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedAccessListMember] {
	return func(yield func(*scopedaccessv1.ScopedAccessListMember, error) bool) {
		startKey := scopedAccessListMemberKey("")
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   backend.RangeEnd(startKey),
		}

		for item, err := range s.bk.Items(ctx, params) {
			if err != nil {
				// backend errors terminate the stream
				yield(nil, trace.Wrap(err))
				return
			}

			member, err := scopedAccessListMemberFromItem(&item)
			if err != nil {
				// per-member errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped access list member due to unmarshal error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if err := scopedaccess.WeakValidateAccessListMember(member); err != nil {
				// per-list errors are logged and skipped
				s.logger.WarnContext(ctx, "skipping scoped access list due to validation error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if !yield(member, nil) {
				return
			}
		}
	}
}

func scopedRoleKey(roleName string) backend.Key {
	return backend.NewKey(scopedRolePrefix, scopedRoleRoleComponent, roleName)
}

func scopedRoleWatchPrefix() backend.Key {
	return backend.ExactKey(scopedRolePrefix, scopedRoleRoleComponent)
}

func scopedRoleAssignmentKey(assignmentID string) backend.Key {
	return backend.NewKey(scopedRolePrefix, scopedRoleAssignmentComponent, assignmentID)
}

func scopedRoleAssignmentWatchPrefix() backend.Key {
	return backend.ExactKey(scopedRolePrefix, scopedRoleAssignmentComponent)
}

func userAssignmentLockKey(username string) backend.Key {
	return backend.NewKey(scopedRolePrefix, userAssignmentLockComponent, username)
}

func roleAssignmentLockKey(roleName string) backend.Key {
	return backend.NewKey(scopedRolePrefix, roleAssignmentLockComponent, roleName)
}

func scopedAccessListKey(name string) backend.Key {
	return backend.NewKey(scopedAccessListPrefix, name)
}

func scopedAccessListMemberKey(name string) backend.Key {
	return backend.NewKey(scopedAccessListMemberPrefix, name)
}

// newUserAssignmentLockVal generates a new user assignment lock value for the specified username. A random
// element is used to ensure that the lock value changes for each operation that changes assignments.
func newUserAssignmentLockVal(username string) []byte {
	return []byte(rand.Text() + "-" + username)
}

// newRoleAssignmentLockVal generates a new role assignment lock value for the specified role name. A random
// element is used to ensure that the lock value changes for each operation that changes assignments.
func newRoleAssignmentLockVal(roleName string) []byte {
	return []byte(rand.Text() + "-" + roleName)
}

func scopedRoleFromItem(item *backend.Item) (*scopedaccessv1.ScopedRole, error) {
	var role scopedaccessv1.ScopedRole
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(item.Value, &role); err != nil {
		return nil, trace.Wrap(err)
	}

	if role.GetMetadata() == nil {

		return nil, trace.BadParameter("role at %q is critically malformed (missing metadata)", item.Key)
	}

	role.Metadata.Revision = item.Revision
	role.Metadata.Expires = utils.TimeIntoProto(item.Expires)
	return &role, nil
}

func scopedRoleToItem(role *scopedaccessv1.ScopedRole) (backend.Item, error) {
	if role.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped role")
	}

	if role.GetMetadata().Expires != nil {
		return backend.Item{}, trace.BadParameter("scoped roles do not support expiration")
	}

	data, err := protojson.Marshal(role)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      scopedRoleKey(role.GetMetadata().GetName()),
		Value:    data,
		Revision: role.GetMetadata().GetRevision(),
	}, nil
}

func scopedRoleAssignmentFromItem(item *backend.Item) (*scopedaccessv1.ScopedRoleAssignment, error) {
	var assignment scopedaccessv1.ScopedRoleAssignment
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(item.Value, &assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	if assignment.GetMetadata() == nil {
		return nil, trace.BadParameter("assignment at %q is critically malformed (missing metadata)", item.Key)
	}

	assignment.Metadata.Revision = item.Revision
	assignment.Metadata.Expires = utils.TimeIntoProto(item.Expires)
	return &assignment, nil
}

func scopedRoleAssignmentToItem(assignment *scopedaccessv1.ScopedRoleAssignment) (backend.Item, error) {
	if assignment.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped role assignment")
	}

	if assignment.GetMetadata().Expires != nil {
		return backend.Item{}, trace.BadParameter("scoped role assignments do not support expiration")
	}

	data, err := protojson.Marshal(assignment)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      scopedRoleAssignmentKey(assignment.GetMetadata().GetName()),
		Value:    data,
		Revision: assignment.GetMetadata().GetRevision(),
	}, nil
}

func scopedAccessListFromItem(item *backend.Item) (*scopedaccessv1.ScopedAccessList, error) {
	var list scopedaccessv1.ScopedAccessList
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(item.Value, &list); err != nil {
		return nil, trace.Wrap(err)
	}

	if list.GetMetadata() == nil {
		return nil, trace.BadParameter("access list at %q is critically malformed (missing metadata)", item.Key)
	}

	list.Metadata.Revision = item.Revision
	list.Metadata.Expires = utils.TimeIntoProto(item.Expires)
	return &list, nil
}

func scopedAccessListToItem(list *scopedaccessv1.ScopedAccessList) (backend.Item, error) {
	if list.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped access list")
	}

	if list.GetMetadata().Expires != nil {
		return backend.Item{}, trace.BadParameter("scoped access lists do not support expiration")
	}

	data, err := protojson.Marshal(list)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      scopedAccessListKey(list.GetMetadata().GetName()),
		Value:    data,
		Revision: list.GetMetadata().GetRevision(),
	}, nil
}

func scopedAccessListMemberFromItem(item *backend.Item) (*scopedaccessv1.ScopedAccessListMember, error) {
	var member scopedaccessv1.ScopedAccessListMember
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(item.Value, &member); err != nil {
		return nil, trace.Wrap(err)
	}

	if member.GetMetadata() == nil {
		return nil, trace.BadParameter("access list member at %q is critically malformed (missing metadata)", item.Key)
	}

	member.Metadata.Revision = item.Revision
	member.Metadata.Expires = utils.TimeIntoProto(item.Expires)
	return &member, nil
}

func scopedAccessListMemberToItem(member *scopedaccessv1.ScopedAccessListMember) (backend.Item, error) {
	if member.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped access list member")
	}

	if member.GetMetadata().Expires != nil {
		return backend.Item{}, trace.BadParameter("scoped access list members do not support expiration")
	}

	data, err := protojson.Marshal(member)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      scopedAccessListMemberKey(member.GetMetadata().GetName()),
		Value:    data,
		Revision: member.GetMetadata().GetRevision(),
	}, nil
}

// scopedRoleWithRevision creates a copy of the provided role with an updated revision.
func scopedRoleWithRevision(role *scopedaccessv1.ScopedRole, revision string) *scopedaccessv1.ScopedRole {
	role = apiutils.CloneProtoMsg(role)
	role.Metadata.Revision = revision
	return role
}

// scopedRoleAssignmentWithRevision creates a copy of the provided assignment with an updated revision.
func scopedRoleAssignmentWithRevision(assignment *scopedaccessv1.ScopedRoleAssignment, revision string) *scopedaccessv1.ScopedRoleAssignment {
	assignment = apiutils.CloneProtoMsg(assignment)
	assignment.Metadata.Revision = revision
	return assignment
}

// scopedAccessListWithRevision creates a copy of the provided access list with an updated revision.
func scopedAccessListWithRevision(list *scopedaccessv1.ScopedAccessList, revision string) *scopedaccessv1.ScopedAccessList {
	list = apiutils.CloneProtoMsg(list)
	list.Metadata.Revision = revision
	return list
}

// scopedAccessListMemberWithRevision creates a copy of the provided access list with an updated revision.
func scopedAccessListMemberWithRevision(member *scopedaccessv1.ScopedAccessListMember, revision string) *scopedaccessv1.ScopedAccessListMember {
	member = apiutils.CloneProtoMsg(member)
	member.Metadata.Revision = revision
	return member
}
