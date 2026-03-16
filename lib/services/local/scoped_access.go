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
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
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

		for binding, err := range s.streamBindingsForRole(ctx, role.GetMetadata().GetName()) {
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// an assignment or access list already exists referencing this role, we need to check if that is because
			// a role with this name exists, or because the assignment is dangling.
			_, err = s.bk.Get(ctx, scopedRoleKey(role.GetMetadata().GetName()))
			if err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
				// this is a dangling assignment, we need to return an error
				return nil, trace.CompareFailed("cannot create scoped role %q while %s references it", role.GetMetadata().GetName(), binding.source)
			}
			return nil, trace.CompareFailed("scoped role %q already exists", role.GetMetadata().GetName())
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
	}

	// An update that modifies the assignable scopes of the role may invalidate
	// any assignments or grants in another resource.
	if !apiutils.ContainSameUniqueElements(extant.GetRole().GetSpec().GetAssignableScopes(), role.GetSpec().GetAssignableScopes()) {
		// TODO(nklaassen): make full cross-resource validation opt-in.
		for binding, err := range s.streamBindingsForRole(ctx, role.GetMetadata().GetName()) {
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if !scopedaccess.RoleIsAssignableToScopeOfEffect(extant.GetRole(), binding.scope) {
				// theoretically, we prevent broken assignments. in practice, its best to
				// assume they may exist and to not allow them to prevent an otherwsie
				// valid update. We will still force all broken assignments to be
				// removed at the time of role deletion.
				continue
			}

			if !scopedaccess.RoleIsAssignableToScopeOfEffect(role, binding.scope) {
				return nil, trace.BadParameter("update of scoped role %q would invalidate %s which binds it to scope %q", role.GetMetadata().GetName(), binding.source, binding.scope)
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

	// now that we have a lock condition established, we can read all
	// assignments and access lists with a "happens after" relationship to the
	// current lock value and verify that no assignments or lists target this role.
	for binding, err := range s.streamBindingsForRole(ctx, roleName) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.CompareFailed("cannot delete scoped role %q while %s references it", roleName, binding.source)
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
	subKind := req.GetSubKind()
	switch subKind {
	case "":
		return nil, trace.BadParameter("missing scoped role assignment sub_kind in get request")
	case scopedaccess.SubKindMaterialized:
		return nil, trace.BadParameter(`reading scoped role assignments with sub_kind "materialized" from the backend is not supported`)
	}

	item, err := s.bk.Get(ctx, scopedRoleAssignmentKey{
		name:    assignmentName,
		subKind: subKind,
	}.Key())
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
		startKey := scopedRoleAssignmentWatchPrefix()
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

			if assignment.GetSubKind() == scopedaccess.SubKindMaterialized {
				// Reading materialized assignments from the backend is not
				// currently supported, we skip them in case materialized
				// assignments are persisted to the backend in a future
				// version.
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

	switch assignment.GetSubKind() {
	case scopedaccess.SubKindDynamic:
	case "":
		return nil, trace.BadParameter("creating scoped role assignments with empty sub_kind is not supported")
	default:
		return nil, trace.BadParameter("creating scoped role assignments with sub_kind %q is not supported", assignment.GetSubKind())
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
		// NOTE: this restriction will eventually be relaxed in favor of [scopedaccess.RoleIsAssignableFromScopeOfOrigin]
		// once we've finalized the details of the more relaxed role assignment model (the primary prerequisite is ensuring
		// robust handling of dangling/invalid scoped role assignments).
		if scopes.Compare(rrsp.GetRole().GetScope(), assignment.GetScope()) != scopes.Equivalent {
			return nil, trace.BadParameter("role %q is not scoped to the same resource scope as assignment %q (%q -> %q)", subAssignment.GetRole(), assignment.GetMetadata().GetName(), rrsp.GetRole().GetScope(), assignment.GetScope())
		}

		// verify that the role is assignable at the specified scope
		if !scopedaccess.RoleIsAssignableToScopeOfEffect(rrsp.GetRole(), subAssignment.GetScope()) {
			return nil, trace.BadParameter("scoped role %q is not configured to be assignable at scope %q", subAssignment.GetRole(), subAssignment.GetScope())
		}

		if _, ok := assertedRoles[subAssignment.GetRole()]; ok {
			// a previous sub-assignment already caused us to assert the revision
			// of this role, we can skip the assertion/lock update step.
			continue
		}

		condacts = append(condacts,
			// assert that role is unchanged since it was checked.
			s.assertAssignedRoleStable(subAssignment.GetRole(), roleRevision),
			// touch role lock so that role modifications can detect concurrent
			// modifications to their assignments.
			s.touchRoleAssignmentLock(subAssignment.GetRole()))

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

	subKind := req.GetSubKind()
	switch subKind {
	case scopedaccess.SubKindDynamic:
		// This is the only subkind that should be allowed to be deleted in this version.
	case scopedaccess.SubKindMaterialized:
		return nil, trace.BadParameter(`deleting scoped role assignments with sub_kind "materialized" is not supported`)
	case "":
		return nil, trace.BadParameter("missing scoped role assignment sub_kind in delete request")
	default:
		return nil, trace.BadParameter("unhandled sub_kind %q in scoped role assignment delete request", subKind)
	}

	extant, err := s.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    assignmentName,
		SubKind: subKind,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.CompareFailed("scoped role assignment %q was concurrently deleted", assignmentName)
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
			if assignment.GetMetadata().GetName() == extant.Assignment.GetMetadata().GetName() &&
				assignment.GetSubKind() == extant.Assignment.GetSubKind() {
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
			Key: scopedRoleAssignmentKey{
				name:    assignmentName,
				subKind: extant.Assignment.GetSubKind(),
			}.Key(),
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

func (s *ScopedAccessService) streamAccessLists(ctx context.Context) stream.Stream[*accesslist.AccessList] {
	return func(yield func(*accesslist.AccessList, error) bool) {
		startKey := backend.ExactKey(accessListPrefix)
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   backend.RangeEnd(startKey),
		}

		for item, err := range s.bk.Items(ctx, params) {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			accessList, err := services.UnmarshalAccessList(item.Value,
				services.WithRevision(item.Revision),
				services.WithExpires(item.Expires),
			)
			if err != nil {
				s.logger.WarnContext(ctx, "skipping access list due to unmarshal error", "error", err, "key", logutils.StringerAttr(item.Key))
				continue
			}

			if !yield(accessList, nil) {
				return
			}
		}
	}
}

// scopeBinding describes a concrete scope that a scoped role is "bound" to. A
// scoped role is "bound" to a scope if it is assigned at that scope by a
// scoped role assignment or if it is granted at that scoped by an access list grant.
type scopeBinding struct {
	scope string
	// source describes the resource that bound the role to this scope, useful
	// for error messages.
	source string
}

// streamBindingsForRole streams all bindings of the given role to a scope,
// either by a scoped role assignment or an access list grant. This essentially
// streams all cross-resource references to the role.
func (s *ScopedAccessService) streamBindingsForRole(ctx context.Context, role string) stream.Stream[scopeBinding] {
	return func(yield func(scopeBinding, error) bool) {
		for assignment, err := range s.StreamScopedRoleAssignments(ctx) {
			if err != nil {
				yield(scopeBinding{}, trace.Wrap(err))
				return
			}

			for i, subAssignment := range assignment.GetSpec().GetAssignments() {
				if subAssignment.GetRole() != role {
					continue
				}
				if !yield(scopeBinding{
					scope:  subAssignment.GetScope(),
					source: fmt.Sprintf("scoped role assignment %q spec.assignments[%d]", assignment.GetMetadata().GetName(), i),
				}, nil) {
					return
				}
			}
		}
		for acl, err := range s.streamAccessLists(ctx) {
			if err != nil {
				yield(scopeBinding{}, trace.Wrap(err))
				return
			}

			for _, lense := range []struct {
				field  string
				grants []accesslist.ScopedRoleGrant
			}{
				{field: "grants", grants: acl.GetGrants().ScopedRoles},
				{field: "owner_grants", grants: acl.GetOwnerGrants().ScopedRoles},
			} {
				for i, grant := range lense.grants {
					if grant.Role != role {
						continue
					}
					if !yield(scopeBinding{
						scope:  grant.Scope,
						source: fmt.Sprintf("access list %q spec.%s.scoped_roles[%d]", acl.GetName(), lense.field, i),
					}, nil) {
						return
					}
				}
			}
		}
	}
}

// assertAssignedRoleStable returns a backend.ConditionalAction that asserts
// that a scoped role has not been modified since it was checked at a given
// roleRevision.
func (s *ScopedAccessService) assertAssignedRoleStable(roleName, roleRevision string) backend.ConditionalAction {
	return backend.ConditionalAction{
		Key:       scopedRoleKey(roleName),
		Condition: backend.Revision(roleRevision),
		Action:    backend.Nop(),
	}
}

// touchRoleAssignmentLock returns a backend.ConditionalAction to be included
// in the list of backend.ConditionalActions for an AtomicWrite that modifies a
// resource that assigns a scoped role. This allows operations that create,
// modify, or delete a scoped role to efficiently assert that no assignment of
// that role has been concurrently created or modified.
func (s *ScopedAccessService) touchRoleAssignmentLock(roleName string) backend.ConditionalAction {
	return backend.ConditionalAction{
		Key:       roleAssignmentLockKey(roleName),
		Condition: backend.Whatever(),
		Action: backend.Put(backend.Item{
			Value: newRoleAssignmentLockVal(roleName),
		}),
	}
}

func scopedRoleKey(roleName string) backend.Key {
	return backend.NewKey(scopedRolePrefix, scopedRoleRoleComponent, roleName)
}

func scopedRoleWatchPrefix() backend.Key {
	return backend.ExactKey(scopedRolePrefix, scopedRoleRoleComponent)
}

type scopedRoleAssignmentKey struct {
	name    string
	subKind string
}

func (k scopedRoleAssignmentKey) Key() backend.Key {
	return backend.NewKey(scopedRolePrefix, scopedRoleAssignmentComponent, k.name, k.subKind)
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

	if assignment.GetSubKind() == "" {
		return backend.Item{}, trace.BadParameter("scoped role assignments must have a sub_kind")
	}

	data, err := protojson.Marshal(assignment)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key: scopedRoleAssignmentKey{
			name:    assignment.GetMetadata().GetName(),
			subKind: assignment.GetSubKind(),
		}.Key(),
		Value:    data,
		Revision: assignment.GetMetadata().GetRevision(),
	}, nil
}

// scopedRoleWithRevision creates a copy of the provided role with an updated revision.
func scopedRoleWithRevision(role *scopedaccessv1.ScopedRole, revision string) *scopedaccessv1.ScopedRole {
	role = apiutils.CloneProtoMsg(role)
	role.Metadata.Revision = revision
	return role
}

// scopedRoleAssignmentWithRevision creates a shallow copy of the provided assignment with an updated revision.
func scopedRoleAssignmentWithRevision(assignment *scopedaccessv1.ScopedRoleAssignment, revision string) *scopedaccessv1.ScopedRoleAssignment {
	assignment = apiutils.CloneProtoMsg(assignment)
	assignment.Metadata.Revision = revision
	return assignment
}
