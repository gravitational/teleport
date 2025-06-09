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
	"github.com/gravitational/teleport/lib/services"
)

type roleIndex string

const roleNameIndex roleIndex = "name"

func newRoleCollection(a services.Access, w types.WatchKind) (*collection[types.Role, roleIndex], error) {
	if a == nil {
		return nil, trace.BadParameter("missing parameter Access")
	}

	return &collection[types.Role, roleIndex]{
		store: newStore(
			types.Role.Clone,
			map[roleIndex]func(types.Role) string{
				roleNameIndex: types.Role.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Role, error) {
			return a.GetRoles(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Role {
			return &types.RoleV6{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
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
	for r := range rg.store.resources(roleNameIndex, "", "") {
		roles = append(roles, r.Clone())
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
	for r := range rg.store.resources(roleNameIndex, req.StartKey, "") {
		rv6, ok := r.(*types.RoleV6)
		if !ok {
			continue
		}

		if req.Filter != nil && !req.Filter.Match(rv6) {
			continue
		}

		if len(resp.Roles) == pageSize {
			resp.NextKey = r.GetName()
			break
		}

		resp.Roles = append(resp.Roles, r.Clone().(*types.RoleV6))

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

	r, err := rg.store.get(roleNameIndex, name)
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

	return r.Clone(), nil
}
