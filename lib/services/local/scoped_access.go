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
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// scoped role and assignment state is modeled with the following key ranges:
//
//   - `/scoped_role/role/<role-name>`             (the location of the role resource, stored at author-chosen name)
//   - `/scoped_role/assignment/<assignment-name>` (the assignment resource itself, always stored at a random UUID)
//
// Cross-resource consistency (e.g. verifying that an assignment's scope is compatible with the role it references) is
// intentionally not enforced at write time. Each write is a single-resource atomic operation. Scoped role assignments
// may be dangling or invalid and access-checking logic *must* skip them in that case. The RoleIsEnforceableAt function
// in lib/scopes/access is the primary source of truth for what qualifies as an enforceable/valid assignment, and all
// assignments must be filtered by that function prior to being used to make any access decisions. Currently this happens
// in exactly one place: services.scopedAccessCheckerBuilder.newCheckerForRole.

const (
	scopedRolePrefix              = "scoped_role"
	scopedRoleRoleComponent       = "role"
	scopedRoleAssignmentComponent = "assignment"

	// maxScopedResourceUpsertAttempts is the maximum number of times an upsert
	// operation will retry on a concurrent modification before giving up.
	maxScopedResourceUpsertAttempts = 4
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

	if req.GetNameFilter() != "" {
		return nil, trace.NotImplemented("filtering by name is not implemented for direct backend scoped role reads")
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

	lease, err := s.bk.Create(ctx, item)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			// generic condition failure keeps error handling simpler
			return nil, trace.CompareFailed("scoped role %q already exists", role.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, lease.Revision),
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
		if trace.IsNotFound(err) {
			// generic condition failure keeps error handling simpler
			return nil, trace.CompareFailed("scoped role %q not found", role.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	if role.GetMetadata().GetRevision() != "" && role.GetMetadata().GetRevision() != extant.GetRole().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role %q has been concurrently modified", role.GetMetadata().GetName())
	}

	// disallow change of resource scope via update. use of scopes.Compare directly is generally discouraged,
	// but that is due to ease of misuse, which isn't really a concern for a simple equivalence check.
	if scopes.Compare(role.GetScope(), extant.GetRole().GetScope()) != scopes.Equivalent {
		// XXX: the current implementation of our access-control logic relies upon this invariant being enforced. if we ever
		// relax this restriction here we *must* first modify the outer access-control logic to understand the concept of
		// scope changing and correctly validate the transition.
		return nil, trace.BadParameter("cannot modify the resource scope of scoped role %q (%q -> %q)", role.GetMetadata().GetName(), extant.GetRole().GetScope(), role.GetScope())
	}

	// use the observed revision as the condition so that a concurrent modification is detected.
	role = scopedRoleWithRevision(role, extant.GetRole().GetMetadata().GetRevision())
	item, err := scopedRoleToItem(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.bk.ConditionalUpdate(ctx, item)
	if err != nil {
		if errors.Is(err, backend.ErrIncorrectRevision) {
			return nil, trace.CompareFailed("scoped role %q has been concurrently modified", role.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.UpdateScopedRoleResponse{
		Role: scopedRoleWithRevision(role, lease.Revision),
	}, nil
}

func (s *ScopedAccessService) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	roleName := req.GetName()
	if roleName == "" {
		return nil, trace.BadParameter("missing scoped role name in delete request")
	}

	if rev := req.GetRevision(); rev != "" {
		if err := s.bk.ConditionalDelete(ctx, scopedRoleKey(roleName), rev); err != nil {
			if errors.Is(err, backend.ErrIncorrectRevision) {
				return nil, trace.CompareFailed("scoped role %q has been concurrently modified", roleName)
			}
			return nil, trace.Wrap(err)
		}
	} else {
		if err := s.bk.Delete(ctx, scopedRoleKey(roleName)); err != nil {
			if trace.IsNotFound(err) {
				// generic condition failure keeps error handling simpler
				return nil, trace.NotFound("scoped role %q not found", roleName)
			}
			return nil, trace.Wrap(err)
		}
	}

	return &scopedaccessv1.DeleteScopedRoleResponse{}, nil
}

func (s *ScopedAccessService) UpsertScopedRole(ctx context.Context, req *scopedaccessv1.UpsertScopedRoleRequest) (*scopedaccessv1.UpsertScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in upsert request")
	}

	if err := scopedaccess.StrongValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	// upsert operations ignore user-provided revision
	role = scopedRoleWithRevision(role, "")

	for attempt := range maxScopedResourceUpsertAttempts {
		if attempt != 0 {
			select {
			case <-time.After(retryutils.FullJitter(time.Duration(300*attempt) * time.Millisecond)):
			case <-ctx.Done():
				return nil, trace.Wrap(ctx.Err())
			}
		}

		existing, err := s.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: role.GetMetadata().GetName(),
		})
		if trace.IsNotFound(err) {
			rsp, err := s.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
				Role: role,
			})
			if err != nil {
				if trace.IsCompareFailed(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			return &scopedaccessv1.UpsertScopedRoleResponse{Role: rsp.GetRole()}, nil
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rsp, err := s.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
			Role: scopedRoleWithRevision(role, existing.GetRole().GetMetadata().GetRevision()),
		})
		if err != nil {
			if trace.IsCompareFailed(err) || trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		return &scopedaccessv1.UpsertScopedRoleResponse{Role: rsp.GetRole()}, nil
	}

	return nil, trace.LimitExceeded("exceeded max retries attempting to upsert scoped role %q", role.GetMetadata().GetName())
}

func (s *ScopedAccessService) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in get request")
	}
	subKind := req.GetSubKind()
	if subKind == scopedaccess.SubKindMaterialized {
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

	lease, err := s.bk.Create(ctx, item)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			// generic condition failure keeps error handling simpler
			return nil, trace.CompareFailed("scoped role assignment %q already exists", assignment.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.CreateScopedRoleAssignmentResponse{
		Assignment: scopedRoleAssignmentWithRevision(assignment, lease.Revision),
	}, nil
}

// UpdateScopedRoleAssignment updates an existing scoped role assignment.
func (s *ScopedAccessService) UpdateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleAssignmentRequest) (*scopedaccessv1.UpdateScopedRoleAssignmentResponse, error) {
	assignment := req.GetAssignment()
	if assignment == nil {
		return nil, trace.BadParameter("missing scoped role assignment in update request")
	}

	if err := scopedaccess.StrongValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(assignment.GetSpec().GetAssignments()) > scopedaccess.MaxRolesPerAssignment {
		return nil, trace.BadParameter("scoped role assignment resource %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), scopedaccess.MaxRolesPerAssignment)
	}

	extant, err := s.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    assignment.GetMetadata().GetName(),
		SubKind: assignment.GetSubKind(),
	})
	if trace.IsNotFound(err) {
		// generic condition failure keeps error handling simpler
		return nil, trace.CompareFailed("scoped role assignment %q not found", assignment.GetMetadata().GetName())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if rev := assignment.GetMetadata().GetRevision(); rev != "" && rev != extant.GetAssignment().GetMetadata().GetRevision() {
		return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified", assignment.GetMetadata().GetName())
	}

	// disallow change of resource scope; this invariant is load-bearing for ACL logic.
	if scopes.Compare(assignment.GetScope(), extant.GetAssignment().GetScope()) != scopes.Equivalent {
		return nil, trace.BadParameter("cannot modify the resource scope of scoped role assignment %q (%q -> %q)", assignment.GetMetadata().GetName(), extant.GetAssignment().GetScope(), assignment.GetScope())
	}

	// use the observed revision as the condition so that a concurrent modification is detected.
	assignment = scopedRoleAssignmentWithRevision(assignment, extant.GetAssignment().GetMetadata().GetRevision())
	item, err := scopedRoleAssignmentToItem(assignment)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.bk.ConditionalUpdate(ctx, item)
	if err != nil {
		if errors.Is(err, backend.ErrIncorrectRevision) {
			return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified", assignment.GetMetadata().GetName())
		}
		return nil, trace.Wrap(err)
	}

	return &scopedaccessv1.UpdateScopedRoleAssignmentResponse{
		Assignment: scopedRoleAssignmentWithRevision(assignment, lease.Revision),
	}, nil
}

func (s *ScopedAccessService) UpsertScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.UpsertScopedRoleAssignmentRequest) (*scopedaccessv1.UpsertScopedRoleAssignmentResponse, error) {
	assignment := req.GetAssignment()
	if assignment == nil {
		return nil, trace.BadParameter("missing scoped role assignment in upsert request")
	}

	if err := scopedaccess.StrongValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	// upsert operations ignore user-provided revision
	assignment = scopedRoleAssignmentWithRevision(assignment, "")

	for attempt := range maxScopedResourceUpsertAttempts {
		if attempt != 0 {
			select {
			case <-time.After(retryutils.FullJitter(time.Duration(300*attempt) * time.Millisecond)):
			case <-ctx.Done():
				return nil, trace.Wrap(ctx.Err())
			}
		}

		_, err := s.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		})
		if trace.IsNotFound(err) {
			rsp, err := s.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
				Assignment: assignment,
			})
			if err != nil {
				if trace.IsCompareFailed(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			return &scopedaccessv1.UpsertScopedRoleAssignmentResponse{Assignment: rsp.GetAssignment()}, nil
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// update path
		ursp, err := s.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{Assignment: assignment})
		if err != nil {
			if trace.IsCompareFailed(err) || trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		return &scopedaccessv1.UpsertScopedRoleAssignmentResponse{Assignment: ursp.GetAssignment()}, nil
	}

	return nil, trace.LimitExceeded("exceeded max retries attempting to upsert scoped role assignment %q", assignment.GetMetadata().GetName())
}

func (s *ScopedAccessService) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in delete request")
	}

	subKind := req.GetSubKind()
	switch subKind {
	case scopedaccess.SubKindDynamic, "":
	case scopedaccess.SubKindMaterialized:
		return nil, trace.BadParameter(`deleting scoped role assignments with sub_kind "materialized" is not supported`)
	default:
		return nil, trace.BadParameter("unhandled sub_kind %q in scoped role assignment delete request", subKind)
	}

	key := scopedRoleAssignmentKey{name: assignmentName, subKind: subKind}.Key()

	if rev := req.GetRevision(); rev != "" {
		if err := s.bk.ConditionalDelete(ctx, key, rev); err != nil {
			if errors.Is(err, backend.ErrIncorrectRevision) {
				return nil, trace.CompareFailed("scoped role assignment %q has been concurrently modified", assignmentName)
			}
			return nil, trace.Wrap(err)
		}
	} else {
		if err := s.bk.Delete(ctx, key); err != nil {
			if trace.IsNotFound(err) {
				// generic condition failure keeps error handling simpler
				return nil, trace.NotFound("scoped role assignment %q not found", assignmentName)
			}
			return nil, trace.Wrap(err)
		}
	}

	return &scopedaccessv1.DeleteScopedRoleAssignmentResponse{}, nil
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
	if k.subKind == "" {
		// Supports reading old scoped role assignments created without a subkind.
		return backend.NewKey(scopedRolePrefix, scopedRoleAssignmentComponent, k.name)
	}
	return backend.NewKey(scopedRolePrefix, scopedRoleAssignmentComponent, k.name, k.subKind)
}

func scopedRoleAssignmentWatchPrefix() backend.Key {
	return backend.ExactKey(scopedRolePrefix, scopedRoleAssignmentComponent)
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
