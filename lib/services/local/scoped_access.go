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
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedrole "github.com/gravitational/teleport/lib/scopes/roles"
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

func (s *ScopedAccessService) GetScopedRole(ctx context.Context, req *accessv1.GetScopedRoleRequest) (*accessv1.GetScopedRoleResponse, error) {
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

	if err := scopedrole.WeakValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	return &accessv1.GetScopedRoleResponse{
		Role: role,
	}, nil
}

// StreamScopedRoles returns a stream of all scoped roles in the backend. Malformed roles are skipped. Returned roles
// have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoles(ctx context.Context) stream.Stream[*accessv1.ScopedRole] {
	return func(yield func(*accessv1.ScopedRole, error) bool) {
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

			if err := scopedrole.WeakValidateRole(role); err != nil {
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

func (s *ScopedAccessService) CreateScopedRole(ctx context.Context, req *accessv1.CreateScopedRoleRequest) (*accessv1.CreateScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in create request")
	}

	if err := scopedrole.StrongValidateRole(role); err != nil {
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

	return &accessv1.CreateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, revision),
	}, nil
}

func (s *ScopedAccessService) UpdateScopedRole(ctx context.Context, req *accessv1.UpdateScopedRoleRequest) (*accessv1.UpdateScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in update request")
	}

	if err := scopedrole.StrongValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	extant, err := s.GetScopedRole(ctx, &accessv1.GetScopedRoleRequest{
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

				if !scopedrole.RoleIsAssignableAtScope(extant.GetRole(), subAssignment.GetScope()) {
					// theoretically, we prevent broken assignments. in practice, its best to
					// assume they may exist and to not allow them to prevent an otherwsie
					// valid update. We will still force all broken assignments to be
					// removed at the time of role deletion.
					continue
				}

				if !scopedrole.RoleIsAssignableAtScope(role, subAssignment.GetScope()) {
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

	return &accessv1.UpdateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, revision),
	}, nil
}

func (s *ScopedAccessService) DeleteScopedRole(ctx context.Context, req *accessv1.DeleteScopedRoleRequest) (*accessv1.DeleteScopedRoleResponse, error) {
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

	return &accessv1.DeleteScopedRoleResponse{}, nil
}

func (s *ScopedAccessService) GetScopedRoleAssignment(ctx context.Context, req *accessv1.GetScopedRoleAssignmentRequest) (*accessv1.GetScopedRoleAssignmentResponse, error) {
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

	if err := scopedrole.WeakValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	return &accessv1.GetScopedRoleAssignmentResponse{
		Assignment: assignment,
	}, nil
}

// StreamScopedRoleAssignments returns a stream of all scoped role assignments in the backend. Malformed assignments are skipped.
// Returned assignments have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoleAssignments(ctx context.Context) stream.Stream[*accessv1.ScopedRoleAssignment] {
	return func(yield func(*accessv1.ScopedRoleAssignment, error) bool) {
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

			if err := scopedrole.WeakValidateAssignment(assignment); err != nil {
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

func (s *ScopedAccessService) CreateScopedRoleAssignment(ctx context.Context, req *accessv1.CreateScopedRoleAssignmentRequest) (*accessv1.CreateScopedRoleAssignmentResponse, error) {
	assignment := req.GetAssignment()
	if assignment == nil {
		return nil, trace.BadParameter("missing scoped role assignment in create request")
	}

	if err := scopedrole.StrongValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	// independently enforce the max number of roles per assignment limit here since not all validation
	// may necessarily enforce it, but it is a hard-limit for the backend impl.
	if len(assignment.GetSpec().GetAssignments()) > scopedrole.MaxRolesPerAssignment {
		return nil, trace.BadParameter("scoped role assignment resource %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), scopedrole.MaxRolesPerAssignment)
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

		rrsp, err := s.GetScopedRole(ctx, &accessv1.GetScopedRoleRequest{
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
			return nil, trace.BadParameter("role %q is not scoped to the same resource scope as assignment %q (%q -> %q)", subAssignment.GetRole(), assignment.GetMetadata().GetName(), rrsp.GetRole().GetScope(), subAssignment.GetScope())
		}

		// verify that the role is assignable at the specified scope
		if !scopedrole.RoleIsAssignableAtScope(rrsp.GetRole(), subAssignment.GetScope()) {
			return nil, trace.BadParameter("scoped role %q is not configured to be assignable at scope %q", subAssignment.GetRole(), subAssignment.GetScope())
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
	}

	revision, err := s.bk.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			// return a general concurrent-modification error since it isn't clear which condition faile
			return nil, trace.CompareFailed("scoped role assignment %q failed due to concurrent modification of associated resources", assignment.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &accessv1.CreateScopedRoleAssignmentResponse{
		Assignment: scopedRoleAssignmentWithRevision(assignment, revision),
	}, nil
}

func (s *ScopedAccessService) DeleteScopedRoleAssignment(ctx context.Context, req *accessv1.DeleteScopedRoleAssignmentRequest) (*accessv1.DeleteScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in delete request")
	}

	extant, err := s.GetScopedRoleAssignment(ctx, &accessv1.GetScopedRoleAssignmentRequest{
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

	for _, subAssignment := range extant.Assignment.GetSpec().GetAssignments() {
		// operation must modify all associated role locks to ensure that role operations can
		// efficiently assert that no assigment related to the role has changed.
		condacts = append(condacts, backend.ConditionalAction{
			Key:       roleAssignmentLockKey(subAssignment.GetRole()),
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: newRoleAssignmentLockVal(subAssignment.GetRole()),
			}),
		})
	}

	if _, err := s.bk.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("scoped role assignment %q or another related assignment was concurrently modified", assignmentName)
		}
		return nil, trace.Wrap(err)
	}

	return &accessv1.DeleteScopedRoleAssignmentResponse{}, nil
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

func scopedRoleFromItem(item *backend.Item) (*accessv1.ScopedRole, error) {
	var role accessv1.ScopedRole
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

func scopedRoleToItem(role *accessv1.ScopedRole) (backend.Item, error) {
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

func scopedRoleAssignmentFromItem(item *backend.Item) (*accessv1.ScopedRoleAssignment, error) {
	var assignment accessv1.ScopedRoleAssignment
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

func scopedRoleAssignmentToItem(assignment *accessv1.ScopedRoleAssignment) (backend.Item, error) {
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

// scopedRoleWithRevision creates a copy of the provided role with an updated revision.
func scopedRoleWithRevision(role *accessv1.ScopedRole, revision string) *accessv1.ScopedRole {
	role = apiutils.CloneProtoMsg(role)
	role.Metadata.Revision = revision
	return role
}

// scopedRoleAssignmentWithRevision creates a shallow copy of the provided assignment with an updated revision.
func scopedRoleAssignmentWithRevision(assignment *accessv1.ScopedRoleAssignment, revision string) *accessv1.ScopedRoleAssignment {
	assignment = apiutils.CloneProtoMsg(assignment)
	assignment.Metadata.Revision = revision
	return assignment
}
