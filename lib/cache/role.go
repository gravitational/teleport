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

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type roleIndex struct{}

var roleNameIndex roleIndex

type cachedRole struct {
	role *types.RoleV6
	wire string
}

func newRoleCollection(a services.Access, w types.WatchKind) (*collection[*cachedRole, roleIndex], error) {
	if a == nil {
		return nil, trace.BadParameter("missing parameter Access")
	}

	store := newStore(
		types.KindRole,
		func(cr *cachedRole) *cachedRole {
			return &cachedRole{
				role: cloneRoleV6(cr.role),
				wire: cr.wire,
			}
		},
		map[roleIndex]func(*cachedRole) string{
			roleNameIndex: func(cr *cachedRole) string {
				return cr.role.GetName()
			},
		})
	return &collection[*cachedRole, roleIndex]{
		store: store,
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*cachedRole, error) {
			roles, err := a.GetRoles(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cachedRoles := make([]*cachedRole, 0, len(roles))
			for _, role := range roles {
				rv6, _ := role.(*types.RoleV6)
				if rv6 == nil {
					continue
				}
				wire, err := gogoproto.Marshal(rv6)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				cachedRoles = append(cachedRoles, &cachedRole{
					role: rv6,
					wire: string(wire),
				})
			}
			return cachedRoles, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *cachedRole {
			return &cachedRole{
				role: &types.RoleV6{
					Metadata: types.Metadata{
						Name: hdr.GetName(),
					},
				},
			}
		},
		customPut: func(rsc types.Resource) error {
			rv6, _ := rsc.(*types.RoleV6)
			if rv6 == nil {
				return trace.BadParameter("unexpected type %T (expected %T, this is a bug)", rsc, rv6)
			}
			wire, err := gogoproto.Marshal(rv6)
			if err != nil {
				return trace.Wrap(err)
			}
			store.put(&cachedRole{
				role: rv6,
				wire: string(wire),
			})
			return nil
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
	for cr := range rg.store.resources(roleNameIndex, "", "") {
		roles = append(roles, cloneRoleV6(cr.role))
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
	for cr := range rg.store.resources(roleNameIndex, req.StartKey, "") {
		if req.Filter != nil && !req.Filter.Match(cr.role) {
			continue
		}

		if len(resp.Roles) == pageSize {
			resp.NextKey = cr.role.GetName()
			break
		}

		resp.Roles = append(resp.Roles, cloneRoleV6(cr.role))

	}
	return &resp, nil
}

// ListRolesForGRPC implements [authclient.Cache].
func (c *Cache) ListRolesForGRPC(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListRolesForGRPC")
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
	for cr := range rg.store.resources(roleNameIndex, req.StartKey, "") {
		if req.Filter != nil && !req.Filter.Match(cr.role) {
			continue
		}

		if len(resp.Roles) == pageSize {
			resp.NextKey = cr.role.GetName()
			break
		}

		// this is a terrible hack but we live in a terrible world
		resp.Roles = append(resp.Roles, &types.RoleV6{XXX_unrecognized: []byte(cr.wire)})
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

	return cloneRoleV6(r.role), nil
}

func cloneRoleV6(src *types.RoleV6) *types.RoleV6 {
	if src == nil {
		return nil
	}
	dst := new(types.RoleV6)
	gogoproto.Merge(dst, src)
	return dst
}
