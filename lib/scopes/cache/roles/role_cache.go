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

package roles

import (
	"context"

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

// RoleCache is a cache for scoped role roles.
type RoleCache struct {
	cache *cache.Cache[*accessv1.ScopedRole, string]
}

// NewRoleCache creates a new role cache instance.
func NewRoleCache() *RoleCache {
	return &RoleCache{
		cache: cache.Must(cache.Config[*accessv1.ScopedRole, string]{
			Scope: func(role *accessv1.ScopedRole) string {
				return role.GetScope()
			},
			Key: func(role *accessv1.ScopedRole) string {
				return role.GetMetadata().GetName()
			},
			Clone: proto.CloneOf[*accessv1.ScopedRole],
		}),
	}
}

// ListScopedRoles returns a paginated list of scoped roles.
func (c *RoleCache) ListScopedRoles(ctx context.Context, req *accessv1.ListScopedRolesRequest) (*accessv1.ListScopedRolesResponse, error) {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	if req.GetResourceScope() != nil && req.GetAssignableScope() != nil {
		return nil, trace.BadParameter("cannot filter by both resource scope and assignable scope simultaneously")
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
			return nil, trace.BadParameter("unsupported or unspecified scoping mode %q in scoped role role resource scope filter", req.GetResourceScope().GetMode())
		}
		scope = req.GetResourceScope().GetScope()
	}

	if req.GetAssignableScope() != nil {
		// NOTE: we eventually want to be able to query roles by the scopes at which they are assignable,
		// instead of just resource scope. this is important for introspection/auditing but not necessary for
		// the core funcationality of teleport, so implementation has been deferred. (will require implementation
		// of a more complex caching structure where each resource will be associable with multiple scopes)
		return nil, trace.NotImplemented("assignable scope filtering for scoped role roles is not yet supported")
	}

	var out []*accessv1.ScopedRole
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range getter(scope, c.cache.WithCursor(cursor)) {
		for role := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key:   role.GetMetadata().GetName(),
				}
				break Outer
			}
			out = append(out, role)
		}
	}

	var nextPageToken string
	if !nextCursor.IsZero() {
		nextPageToken, err = cache.EncodeStringCursor(nextCursor)
		if err != nil {
			return nil, trace.Errorf("failed to encode cursor %+v: %w (this is a bug)", nextCursor, err)
		}
	}

	return &accessv1.ListScopedRolesResponse{
		Roles:         out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new role to the cache. It will overwrite any existing role with the same name.
func (c *RoleCache) Put(role *accessv1.ScopedRole) error {
	if err := sr.WeakValidateRole(role); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(role)
	return nil
}

// Del removes an role from the cache by name.
func (c *RoleCache) Delete(name string) {
	c.cache.Del(name)
}
