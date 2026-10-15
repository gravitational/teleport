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
package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type roleIndex struct{}

var roleNameIndex roleIndex

func roleNameIndexKeyForName(name string) string {
	// this matches the order of the roles in the backend, which is
	// unfortunately not the correct order (role "foo" should come earlier than
	// role "foo-bar", but with a "/params" suffix the order is the other way
	// around)
	return name + "/params"
}

type cachedRole = cachedGogoResource[*types.RoleV6, types.RoleV6]

func newCachedRole(role *types.RoleV6) (*cachedRole, error) {
	return newCachedGogoResource(roleNameIndexKeyForName(role.Metadata.Name), role)
}

func newRoleCollection(a services.Access, w types.WatchKind) (*collection[*cachedRole, roleIndex], error) {
	if a == nil {
		return nil, trace.BadParameter("missing parameter Access")
	}

	return &collection[*cachedRole, roleIndex]{
		store: newStore(
			types.KindRole,
			(*cachedRole).clone,
			map[roleIndex]func(*cachedRole) string{
				roleNameIndex: func(cr *cachedRole) string { return cr.key },
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*cachedRole, error) {
			pageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*types.RoleV6, string, error) {
				resp, err := a.ListRoles(ctx, &proto.ListRolesRequest{Limit: int32(pageSize), StartKey: pageToken})
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return resp.GetRoles(), resp.GetNextKey(), nil
			}
			// the default backend page size, which we unfortunately have to
			// just know on the client side to support page size scaling
			const pageSize = 100
			return stream.Collect(stream.TryMap(clientutils.ResourcesWithPageSize(ctx, pageFunc, pageSize), newCachedRole))
		},
		headerTransform: func(hdr *types.ResourceHeader) *cachedRole {
			return &cachedRole{
				key: roleNameIndexKeyForName(hdr.Metadata.Name),
			}
		},
		watch: w,
		resourcePutTransform: func(rsc types.Resource) (*cachedRole, error) {
			role, _ := rsc.(*types.RoleV6)
			if role == nil {
				return nil, trace.BadParameter("unexpected type %T (expected %T, this is a bug)", rsc, role)
			}
			return newCachedRole(role)
		},
	}, nil
}

// GetRoles is a part of auth.Cache implementation
func (c *Cache) GetRoles(ctx context.Context) ([]types.Role, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRoles")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		roles, err := c.Config.Access.GetRoles(ctx)
		return roles, trace.Wrap(err)
	}

	roles := make([]types.Role, 0, rg.store.len())
	for cr := range rg.store.resources(roleNameIndex, "", "") {
		roles = append(roles, cr.cloneResource())
	}

	return roles, nil
}

// ListRoles is a paginated role getter.
func (c *Cache) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListRoles")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		resp, err := c.Config.Access.ListRoles(ctx, req)
		return resp, trace.Wrap(err)
	}

	// Match the page sizing behavior from backend reads.
	pageSize := int(req.Limit)
	if pageSize == 0 {
		pageSize = 100
	}

	const maxPageSize = 16_000
	if pageSize > maxPageSize {
		return nil, trace.BadParameter("page size of %d is too large", pageSize)
	}

	var resp proto.ListRolesResponse
	var start string
	if req.StartKey != "" {
		start = roleNameIndexKeyForName(req.StartKey)
	}
	for cr := range rg.store.resources(roleNameIndex, start, "") {
		if req.Filter != nil && !req.Filter.Match(cr.rsc) {
			continue
		}

		if len(resp.Roles) == pageSize {
			resp.NextKey = cr.rsc.Metadata.Name
			break
		}

		resp.Roles = append(resp.Roles, cr.cloneResource())

	}
	return &resp, nil
}

// GetRole is a part of auth.Cache implementation
func (c *Cache) GetRole(ctx context.Context, name string) (types.Role, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRole")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		role, err := c.Config.Access.GetRole(ctx, name)
		return role, trace.Wrap(err)
	}

	cr, err := rg.store.get(roleNameIndex, roleNameIndexKeyForName(name))
	if err != nil {
		// release read lock early
		rg.Release()

		// fallback is sane because method is never used
		// in construction of derivative caches.
		if trace.IsNotFound(err) {
			if role, err := c.Config.Access.GetRole(ctx, name); err == nil {
				return role, nil
			}

			// This error message format should be kept in sync with web/packages/teleport/src/services/api/api.isRoleNotFoundError
			return nil, trace.NotFound("role %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}

	return cr.cloneResource(), nil
}
