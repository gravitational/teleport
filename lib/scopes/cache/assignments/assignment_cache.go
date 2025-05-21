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

	"github.com/gravitational/trace"

	srpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedrole/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/cache"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

const defaultPageSize = 256

// AssignmentCache is a cache for scoped role assignments.
type AssignmentCache struct {
	cache *cache.Cache[*srpb.ScopedRoleAssignment, string]
}

// NewAssignmentCache creates a new assignment cache instance.
func NewAssignmentCache() *AssignmentCache {
	return &AssignmentCache{
		cache: cache.Must(cache.Config[*srpb.ScopedRoleAssignment, string]{
			Scope: func(assignment *srpb.ScopedRoleAssignment) string {
				return assignment.Scope
			},
			Key: func(assignment *srpb.ScopedRoleAssignment) string {
				return assignment.Metadata.Name
			},
			Clone: func(assignment *srpb.ScopedRoleAssignment) *srpb.ScopedRoleAssignment {
				return apiutils.CloneProtoMsg(assignment)
			},
		}),
	}
}

func (c *AssignmentCache) ListScopedRoleAssignments(ctx context.Context, req *srpb.ListScopedRoleAssignmentsRequest) (*srpb.ListScopedRoleAssignmentsResponse, error) {
	pageSize := int(req.PageSize)
	if pageSize == 0 {
		pageSize = defaultPageSize
	}

	if req.ResourceScope != nil && req.AssignedScope != nil {
		return nil, trace.BadParameter("cannot filter by both resource scope and assigned scope simultaneously")
	}

	filter := func(assignment *srpb.ScopedRoleAssignment) bool {
		if req.User != "" && assignment.Spec.User != req.User {
			return false
		}

		if req.Role != "" {
			var foundRole bool
			for _, subAssignment := range assignment.Spec.Assignments {
				if subAssignment.Role != req.Role {
					continue
				}
				foundRole = true
				break
			}
			if !foundRole {
				return false
			}
		}

		return true
	}

	cursor, err := cache.DecodeStringCursor(req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// resources subject to policy scope root is basically the scoped resource equivalent
	// of a "get all". this is a reasonable default for most queries.
	getter := c.cache.ResourcesSubjectToPolicyScope
	scope := scopes.Root

	// if a resource scope filter has been provided, the user has specified a custom scope/mode to
	// query by.
	if req.ResourceScope != nil {
		// a resource-scope based filter has been provided
		switch req.ResourceScope.Mode {
		case scopespb.Mode_MODE_UNSPECIFIED:
			return nil, trace.BadParameter("missing scoping mode in scoped role assignment resource scope filter")
		case scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE:
			getter = c.cache.ResourcesSubjectToPolicyScope
		case scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE:
			getter = c.cache.PoliciesApplicableToResourceScope
		default:
			return nil, trace.BadParameter("unsupported scoping mode %q in scoped role assignment resource scope filter", req.ResourceScope.Mode)
		}
		scope = req.ResourceScope.Scope
	}

	if req.AssignedScope != nil {
		// NOTE: we eventually want to be able to query assignments by the scopes at which they assign roles,
		// instead of just resource scope. this is important for introspection/auditing but not necessary for
		// the core funcationality of teleport, so implementation has been deferred. (will require implementation
		// of a much more complex caching structure where each resource will be associable with multiple scopes)
		return nil, trace.NotImplemented("assigned scope filtering for scoped role assignments is not yet supported")
	}

	var out []*srpb.ScopedRoleAssignment
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range getter(scope, c.cache.WithFilter(filter), c.cache.WithCursor(cursor)) {
		for assignment := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key:   assignment.Metadata.Name,
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
			return nil, trace.Errorf("failed to encode cursor +%v: %v (this is a bug)", nextCursor, err)
		}
	}

	return &srpb.ListScopedRoleAssignmentsResponse{
		Assignments:   out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new assignment to the cache. It will overwrite any existing assignment with the same name.
func (c *AssignmentCache) Put(assignment *srpb.ScopedRoleAssignment) error {
	if err := sr.WeakValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(assignment)
	return nil
}

// Del removes an assignment from the cache by name.
func (c *AssignmentCache) Del(name string) {
	c.cache.Del(name)
}
