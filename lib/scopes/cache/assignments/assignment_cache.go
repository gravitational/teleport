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

package assignments

import (
	"context"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/cache"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

const (
	defaultPageSize = 256
	maxPageSize     = 1024
)

// AssignmentCache is a cache for scoped role assignments.
type AssignmentCache struct {
	cache *cache.Cache[*accessv1.ScopedRoleAssignment, string]
}

// NewAssignmentCache creates a new assignment cache instance.
func NewAssignmentCache() *AssignmentCache {
	return &AssignmentCache{
		cache: cache.Must(cache.Config[*accessv1.ScopedRoleAssignment, string]{
			Scope: func(assignment *accessv1.ScopedRoleAssignment) string {
				return assignment.GetScope()
			},
			Key: func(assignment *accessv1.ScopedRoleAssignment) string {
				return assignment.GetMetadata().GetName()
			},
			Clone: proto.CloneOf[*accessv1.ScopedRoleAssignment],
		}),
	}
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
func (c *AssignmentCache) ListScopedRoleAssignments(ctx context.Context, req *accessv1.ListScopedRoleAssignmentsRequest) (*accessv1.ListScopedRoleAssignmentsResponse, error) {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	if req.GetResourceScope() != nil && req.GetAssignedScope() != nil {
		return nil, trace.BadParameter("cannot filter by both resource scope and assigned scope simultaneously")
	}

	filter := func(assignment *accessv1.ScopedRoleAssignment) bool {
		if req.GetUser() != "" && assignment.GetSpec().GetUser() != req.GetUser() {
			return false
		}

		if req.GetRole() != "" {
			if !slices.ContainsFunc(assignment.GetSpec().Assignments, func(a *accessv1.Assignment) bool { return a.GetRole() == req.GetRole() }) {
				// if the assignment does not have the requested role, skip it
				return false
			}
		}

		return true
	}

	cursor, err := cache.DecodeStringCursor(req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// resources subject to policy scope root is basically the scoped resource equivalent
	// of a "get all". this is a reasonable default for most queries.
	getter := c.cache.ResourcesSubjectToPolicyScope
	scope := scopes.Root

	// if a resource scope filter has been provided, the user has specified a custom scope/mode to
	// query by.
	if req.GetResourceScope() != nil {
		// a resource-scope based filter has been provided
		switch req.GetResourceScope().GetMode() {
		case scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE:
			getter = c.cache.ResourcesSubjectToPolicyScope
		case scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE:
			getter = c.cache.PoliciesApplicableToResourceScope
		default:
			return nil, trace.BadParameter("unsupported or unspecified scoping mode %q in scoped role assignment resource scope filter", req.GetResourceScope().GetMode())
		}
		scope = req.GetResourceScope().GetScope()
	}

	if req.GetAssignedScope() != nil {
		// NOTE: we eventually want to be able to query assignments by the scopes at which they assign roles,
		// instead of just resource scope. this is important for introspection/auditing but not necessary for
		// the core funcationality of teleport until we've fully migrated to PDP. For now, the implementation has
		// been deferred. (will require implementation of a more complex caching structure where each resource
		// will be associable with multiple scopes)
		return nil, trace.NotImplemented("assigned scope filtering for scoped role assignments is not yet supported")
	}

	var out []*accessv1.ScopedRoleAssignment
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range getter(scope, c.cache.WithFilter(filter), c.cache.WithCursor(cursor)) {
		for assignment := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key:   assignment.GetMetadata().GetName(),
				}
				break Outer
			}
			out = append(out, assignment)
		}
	}

	var nextPageToken string
	if !nextCursor.IsZero() {
		nextPageToken, err = cache.EncodeStringCursor(nextCursor)
		if err != nil {
			return nil, trace.Errorf("failed to encode cursor %+v: %w (this is a bug)", nextCursor, err)
		}
	}

	return &accessv1.ListScopedRoleAssignmentsResponse{
		Assignments:   out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new assignment to the cache. It will overwrite any existing assignment with the same name.
func (c *AssignmentCache) Put(assignment *accessv1.ScopedRoleAssignment) error {
	if err := sr.WeakValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(assignment)
	return nil
}

// Del removes an assignment from the cache by name.
func (c *AssignmentCache) Delete(name string) {
	c.cache.Del(name)
}
