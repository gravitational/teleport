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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
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
//   - `/scoped_role/role/<role-name>`             (the location of the role resource, stored at author-chosen name)
//   - `/scoped_role/assignment/<assignment-name>` (the assignment resource itself, always stored at a random UUID)
//
// Cross-resource consistency (e.g. verifying that an assignment's scope is compatible with the role it references) is
// intentionally not enforced at write time. Each write is a single-resource atomic operation. Scoped role assignments
// may be dangling or invalid and access-checking logic *must* skip them in that case. The RoleIsEnforceableAt function
// in lib/scopes/access is the primary source of truth for what qualifies as an enforceable/valid assignment, and all
// assignments must be filtered by that function prior to being used to make any access decisions. Currently this happens
// in exactly one place: services.scopedAccessCheckerBuilder.newCheckerForRole.

// scoped roles and assignments are keyed by their resource kind directly beneath the shared scoped
// prefix (i.e. /scoped/<kind>/...), consistent with the other scoped resource families. We reuse the
// canonical kind constants so that the key's kind component can never drift from the resource kind.
const (
	scopedRolePrefix           = scopedaccess.KindScopedRole
	scopedRoleAssignmentPrefix = scopedaccess.KindScopedRoleAssignment
)

// ScopedAccessService manages backend state for the ScopedRole and ScopedRoleAssignment types.
type ScopedAccessService struct {
	bk     backend.Backend
	logger *slog.Logger
}

// NewScopedAccessService creates a new ScopedAccessService for the specified backend.
func NewScopedAccessService(bk backend.Backend) *ScopedAccessService {
	// TODO(fspmarshall/scopes): switch this over to use the generic scoped backend once
	// it can support the kind of sub_kind model we use here.
	return &ScopedAccessService{
		bk:     bk,
		logger: slog.With(teleport.ComponentKey, "scopedrole"),
	}
}

func (s *ScopedAccessService) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped role name in get request")
	}

	key, err := scopedRoleKey{scope: req.GetScope(), name: req.GetName()}.Key()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.bk.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped role %q not found in scope %q", req.GetName(), req.GetScope())
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

	return scopedaccessv1.GetScopedRoleResponse_builder{
		Role: role,
	}.Build(), nil
}

// ListScopedRoles returns a paginated list of scoped roles.
// NOTE: this method is only used by local auth caches, and doesn't implement sorting, filtering, or pagination.
func (s *ScopedAccessService) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	if err := scopes.ValidateFilter(req.GetScopeFilter()); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetNameFilter() != "" {
		return nil, trace.NotImplemented("filtering by name is not implemented for direct backend scoped role reads")
	}

	if req.GetPageToken() != "" {
		return nil, trace.NotImplemented("pagination is not implemented for direct backend scoped role reads")
	}

	// use scopedListRange to narrow the read range where permitted by the scope filter.
	startKey, endKey, err := scopedListRange(scopedRolePrefix, req.GetScopeFilter())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*scopedaccessv1.ScopedRole
	for role, err := range s.streamScopedRoles(ctx, startKey, endKey) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !scopes.MatchScope(req.GetScopeFilter(), role.GetScope()) {
			continue
		}

		out = append(out, role)
	}

	return scopedaccessv1.ListScopedRolesResponse_builder{
		Roles: out,
	}.Build(), nil
}

// StreamScopedRoles returns a stream of all scoped roles in the backend. Malformed roles are skipped. Returned roles
// have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoles(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedRole] {
	startKey := scopedRoleWatchPrefix()
	return s.streamScopedRoles(ctx, startKey, backend.RangeEnd(startKey))
}

// streamScopedRoles streams scoped roles from the given backend key range. Malformed roles are skipped. Returned
// roles have had weak validation applied.
func (s *ScopedAccessService) streamScopedRoles(ctx context.Context, startKey, endKey backend.Key) stream.Stream[*scopedaccessv1.ScopedRole] {
	return func(yield func(*scopedaccessv1.ScopedRole, error) bool) {
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   endKey,
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
				s.logger.WarnContext(ctx, "skipping malformed scoped role", "error", err, "key", logutils.StringerAttr(item.Key))
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

	return scopedaccessv1.CreateScopedRoleResponse_builder{
		Role: scopedRoleWithRevision(role, lease.Revision),
	}.Build(), nil
}

func (s *ScopedAccessService) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	role := req.GetRole()
	if role == nil {
		return nil, trace.BadParameter("missing scoped role in update request")
	}

	if err := scopedaccess.StrongValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	extant, err := s.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  role.GetMetadata().GetName(),
		Scope: role.GetScope(),
	}.Build())
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

	return scopedaccessv1.UpdateScopedRoleResponse_builder{
		Role: scopedRoleWithRevision(role, lease.Revision),
	}.Build(), nil
}

func (s *ScopedAccessService) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	roleName := req.GetName()
	if roleName == "" {
		return nil, trace.BadParameter("missing scoped role name in delete request")
	}

	key, err := scopedRoleKey{scope: req.GetScope(), name: roleName}.Key()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if rev := req.GetRevision(); rev != "" {
		if err := s.bk.ConditionalDelete(ctx, key, rev); err != nil {
			if errors.Is(err, backend.ErrIncorrectRevision) {
				return nil, trace.CompareFailed("scoped role %q has been concurrently modified", roleName)
			}
			return nil, trace.Wrap(err)
		}
	} else {
		if err := s.bk.Delete(ctx, key); err != nil {
			if trace.IsNotFound(err) {
				// generic condition failure keeps error handling simpler
				return nil, trace.NotFound("scoped role %q not found in scope %q", roleName, req.GetScope())
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

	item, err := scopedRoleToItem(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.bk.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return scopedaccessv1.UpsertScopedRoleResponse_builder{
		Role: scopedRoleWithRevision(role, lease.Revision),
	}.Build(), nil
}

func (s *ScopedAccessService) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in get request")
	}
	if req.GetSubKind() == scopedaccess.SubKindMaterialized {
		return nil, trace.BadParameter(`reading scoped role assignments with sub_kind "materialized" from the backend is not supported`)
	}

	key, err := scopedRoleAssignmentKey{
		scope:   req.GetScope(),
		name:    req.GetName(),
		subKind: req.GetSubKind(),
	}.Key()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.bk.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("scoped role assignment %q not found in scope %q", req.GetName(), req.GetScope())
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

	return scopedaccessv1.GetScopedRoleAssignmentResponse_builder{
		Assignment: assignment,
	}.Build(), nil
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
// NOTE: this method is only used by local auth caches, and doesn't implement sorting, filtering, or pagination.
func (s *ScopedAccessService) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	if err := scopes.ValidateFilter(req.GetScopeFilter()); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := scopes.ValidateFilter(req.GetAssignedScopeFilter()); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetPageToken() != "" {
		return nil, trace.NotImplemented("pagination is not implemented for direct backend scoped role assignment reads")
	}

	// use scopedListRange to narrow the read range where permitted by the scope filter.
	startKey, endKey, err := scopedListRange(scopedRoleAssignmentPrefix, req.GetScopeFilter())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*scopedaccessv1.ScopedRoleAssignment
	for assignment, err := range s.streamScopedRoleAssignments(ctx, startKey, endKey) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !scopes.MatchScope(req.GetScopeFilter(), assignment.GetScope()) {
			continue
		}

		if !scopedaccess.MatchSecondaryAssignmentFilters(req, assignment) {
			continue
		}

		out = append(out, assignment)
	}

	return scopedaccessv1.ListScopedRoleAssignmentsResponse_builder{
		Assignments: out,
	}.Build(), nil
}

// StreamScopedRoleAssignments returns a stream of all scoped role assignments in the backend. Malformed assignments are skipped.
// Returned assignments have had weak validation applied.
func (s *ScopedAccessService) StreamScopedRoleAssignments(ctx context.Context) stream.Stream[*scopedaccessv1.ScopedRoleAssignment] {
	startKey := scopedRoleAssignmentWatchPrefix()
	return s.streamScopedRoleAssignments(ctx, startKey, backend.RangeEnd(startKey))
}

// streamScopedRoleAssignments streams scoped role assignments from the given backend key range. Malformed assignments
// are skipped. Returned assignments have had weak validation applied.
func (s *ScopedAccessService) streamScopedRoleAssignments(ctx context.Context, startKey, endKey backend.Key) stream.Stream[*scopedaccessv1.ScopedRoleAssignment] {
	return func(yield func(*scopedaccessv1.ScopedRoleAssignment, error) bool) {
		params := backend.ItemsParams{
			StartKey: startKey,
			EndKey:   endKey,
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
				s.logger.WarnContext(ctx, "skipping malformed scoped role assignment", "error", err, "key", logutils.StringerAttr(item.Key))
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

	return scopedaccessv1.CreateScopedRoleAssignmentResponse_builder{
		Assignment: scopedRoleAssignmentWithRevision(assignment, lease.Revision),
	}.Build(), nil
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

	extant, err := s.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment.GetMetadata().GetName(),
		SubKind: assignment.GetSubKind(),
		Scope:   assignment.GetScope(),
	}.Build())
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

	return scopedaccessv1.UpdateScopedRoleAssignmentResponse_builder{
		Assignment: scopedRoleAssignmentWithRevision(assignment, lease.Revision),
	}.Build(), nil
}

func (s *ScopedAccessService) UpsertScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.UpsertScopedRoleAssignmentRequest) (*scopedaccessv1.UpsertScopedRoleAssignmentResponse, error) {
	assignment := req.GetAssignment()
	if assignment == nil {
		return nil, trace.BadParameter("missing scoped role assignment in upsert request")
	}

	if err := scopedaccess.StrongValidateAssignment(assignment); err != nil {
		return nil, trace.Wrap(err)
	}

	switch assignment.GetSubKind() {
	case scopedaccess.SubKindDynamic:
	default:
		return nil, trace.BadParameter("upserting scoped role assignments with sub_kind %q is not supported", assignment.GetSubKind())
	}

	if len(assignment.GetSpec().GetAssignments()) > scopedaccess.MaxRolesPerAssignment {
		return nil, trace.BadParameter("scoped role assignment resource %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), scopedaccess.MaxRolesPerAssignment)
	}

	// upsert operations ignore user-provided revision
	assignment = scopedRoleAssignmentWithRevision(assignment, "")

	item, err := scopedRoleAssignmentToItem(assignment)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.bk.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return scopedaccessv1.UpsertScopedRoleAssignmentResponse_builder{
		Assignment: scopedRoleAssignmentWithRevision(assignment, lease.Revision),
	}.Build(), nil
}

func (s *ScopedAccessService) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	assignmentName := req.GetName()
	if assignmentName == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in delete request")
	}

	subKind := req.GetSubKind()
	switch subKind {
	case scopedaccess.SubKindDynamic:
	case scopedaccess.SubKindMaterialized:
		return nil, trace.BadParameter(`deleting scoped role assignments with sub_kind "materialized" is not supported`)
	default:
		return nil, trace.BadParameter("unhandled sub_kind %q in scoped role assignment delete request", subKind)
	}

	key, err := scopedRoleAssignmentKey{scope: req.GetScope(), name: assignmentName, subKind: subKind}.Key()
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
				return nil, trace.NotFound("scoped role assignment %q not found in scope %q", assignmentName, req.GetScope())
			}
			return nil, trace.Wrap(err)
		}
	}

	return &scopedaccessv1.DeleteScopedRoleAssignmentResponse{}, nil
}

type scopedRoleKey struct {
	scope string
	name  string
}

// Key builds the backend key for a scoped role. The layout
// is /scoped/scoped_role/<encoded_scope>/<name>.
func (k scopedRoleKey) Key() (backend.Key, error) {
	encodedScope, err := scopes.EncodeForKey(k.scope)
	if err != nil {
		return backend.Key{}, trace.Wrap(err)
	}
	return backend.NewKey(scopedPrefix, scopedRolePrefix, encodedScope, k.name), nil
}

func scopedRoleWatchPrefix() backend.Key {
	return backend.ExactKey(scopedPrefix, scopedRolePrefix)
}

type scopedRoleAssignmentKey struct {
	scope   string
	name    string
	subKind string
}

// Key builds the backend key for a scoped role assignment. The layout is
// /scoped/scoped_role_assignment/<encoded_scope>/<name>/<sub_kind>.
func (k scopedRoleAssignmentKey) Key() (backend.Key, error) {
	if k.subKind == "" {
		return backend.Key{}, trace.BadParameter("scoped role assignment sub_kind is required")
	}
	encodedScope, err := scopes.EncodeForKey(k.scope)
	if err != nil {
		return backend.Key{}, trace.Wrap(err)
	}
	return backend.NewKey(scopedPrefix, scopedRoleAssignmentPrefix, encodedScope, k.name, k.subKind), nil
}

func scopedRoleAssignmentWatchPrefix() backend.Key {
	return backend.ExactKey(scopedPrefix, scopedRoleAssignmentPrefix)
}

// scopedListRange is a helper for optimistically narrowing the backend key range to be scanned when the
// scope filter is one that is easily expressible as a backend range query.
func scopedListRange(kindPrefix string, filter *scopesv1.Filter) (startKey, endKey backend.Key, err error) {
	switch filter.GetMode() {
	case scopesv1.Mode_MODE_EXACT:
		encodedScope, err := scopes.EncodeForKey(filter.GetScope())
		if err != nil {
			return backend.Key{}, backend.Key{}, trace.Wrap(err)
		}
		// ExactKey appends a trailing separator, restricting the prefix to match only this exact scope segment.
		start := backend.ExactKey(scopedPrefix, kindPrefix, encodedScope)
		return start, backend.RangeEnd(start), nil
	case scopesv1.Mode_MODE_DESCENDANTS:
		encodedScope, err := scopes.EncodeForKey(filter.GetScope())
		if err != nil {
			return backend.Key{}, backend.Key{}, trace.Wrap(err)
		}
		// NewKey does not append a trailing separator, so the prefix also matches any descendant scopes.
		start := backend.NewKey(scopedPrefix, kindPrefix, encodedScope)
		return start, backend.RangeEnd(start), nil
	default:
		start := backend.ExactKey(scopedPrefix, kindPrefix)
		return start, backend.RangeEnd(start), nil
	}
}

// verifyKeyScope checks that a scoped resource's scope field agrees with the scope encoded in its
// backend key, rejecting the resource if they disagree.
func verifyKeyScope(key, watchPrefix backend.Key, fieldScope string) error {
	components := key.TrimPrefix(watchPrefix).Components()
	if len(components) == 0 {
		return trace.BadParameter("scoped resource key %q is missing its scope component", key)
	}

	keyScope, err := scopes.DecodeFromKey(components[0])
	if err != nil {
		return trace.Wrap(err, "failed decoding scope from scoped resource key %q", key)
	}

	if scopes.Compare(keyScope, fieldScope) != scopes.Equivalent {
		return trace.BadParameter("scoped resource at key %q has scope field %q conflicting with key-encoded scope %q", key, fieldScope, keyScope)
	}

	return nil
}

func scopedRoleFromItem(item *backend.Item) (*scopedaccessv1.ScopedRole, error) {
	var role scopedaccessv1.ScopedRole
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(item.Value, &role); err != nil {
		return nil, trace.Wrap(err)
	}

	if role.GetMetadata() == nil {

		return nil, trace.BadParameter("role at %q is critically malformed (missing metadata)", item.Key)
	}

	if err := verifyKeyScope(item.Key, scopedRoleWatchPrefix(), role.GetScope()); err != nil {
		return nil, trace.Wrap(err)
	}

	role.GetMetadata().SetRevision(item.Revision)
	role.GetMetadata().SetExpires(utils.TimeIntoProto(item.Expires))
	return &role, nil
}

func scopedRoleToItem(role *scopedaccessv1.ScopedRole) (backend.Item, error) {
	if role.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped role")
	}

	if role.GetMetadata().HasExpires() {
		return backend.Item{}, trace.BadParameter("scoped roles do not support expiration")
	}

	data, err := protojson.Marshal(role)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	key, err := scopedRoleKey{scope: role.GetScope(), name: role.GetMetadata().GetName()}.Key()
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      key,
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

	if err := verifyKeyScope(item.Key, scopedRoleAssignmentWatchPrefix(), assignment.GetScope()); err != nil {
		return nil, trace.Wrap(err)
	}

	assignment.GetMetadata().SetRevision(item.Revision)
	assignment.GetMetadata().SetExpires(utils.TimeIntoProto(item.Expires))
	return &assignment, nil
}

func scopedRoleAssignmentToItem(assignment *scopedaccessv1.ScopedRoleAssignment) (backend.Item, error) {
	if assignment.GetMetadata() == nil {
		return backend.Item{}, trace.BadParameter("missing metadata in scoped role assignment")
	}

	if assignment.GetMetadata().HasExpires() {
		return backend.Item{}, trace.BadParameter("scoped role assignments do not support expiration")
	}

	if assignment.GetSubKind() == "" {
		return backend.Item{}, trace.BadParameter("scoped role assignments must have a sub_kind")
	}

	data, err := protojson.Marshal(assignment)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	key, err := scopedRoleAssignmentKey{
		scope:   assignment.GetScope(),
		name:    assignment.GetMetadata().GetName(),
		subKind: assignment.GetSubKind(),
	}.Key()
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      key,
		Value:    data,
		Revision: assignment.GetMetadata().GetRevision(),
	}, nil
}

// scopedRoleWithRevision creates a copy of the provided role with an updated revision.
func scopedRoleWithRevision(role *scopedaccessv1.ScopedRole, revision string) *scopedaccessv1.ScopedRole {
	role = apiutils.CloneProtoMsg(role)
	role.GetMetadata().SetRevision(revision)
	return role
}

// scopedRoleAssignmentWithRevision creates a shallow copy of the provided assignment with an updated revision.
func scopedRoleAssignmentWithRevision(assignment *scopedaccessv1.ScopedRoleAssignment, revision string) *scopedaccessv1.ScopedRoleAssignment {
	assignment = apiutils.CloneProtoMsg(assignment)
	assignment.GetMetadata().SetRevision(revision)
	return assignment
}
