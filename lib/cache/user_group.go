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
	"strings"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type userGroupIndex string

const userGroupNameIndex userGroupIndex = "name"

func newUserGroupCollection(upstream services.UserGroups, w types.WatchKind) (*collection[types.UserGroup, userGroupIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter UserGroups")
	}

	return &collection[types.UserGroup, userGroupIndex]{
		store: newStore(
			types.KindUserGroup,
			types.UserGroup.Clone,
			map[userGroupIndex]func(types.UserGroup) string{
				userGroupNameIndex: types.UserGroup.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.UserGroup, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListUserGroups))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.UserGroup {
			return &types.UserGroupV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// userGroupCollection provides read access to cached user groups. Its
// exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type userGroupCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.UserGroups
	col      *collection[types.UserGroup, userGroupIndex]
}

// ListUserGroups returns a paginated list of user group resources.
func (c userGroupCollection) ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListUserGroups")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		group, nextKey, err := c.upstream.ListUserGroups(ctx, pageSize, nextKey)
		return group, nextKey, trace.Wrap(err)
	}

	// TODO(tross): DELETE IN V20.0.0
	nextKey = strings.TrimPrefix(nextKey, "/")

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > local.GroupMaxPageSize {
		pageSize = local.GroupMaxPageSize
	}

	var groups []types.UserGroup
	for r := range rg.store.resources(userGroupNameIndex, nextKey, "") {
		if len(groups) == pageSize {
			return groups, r.GetName(), nil
		}

		groups = append(groups, r.Clone())

	}
	return groups, "", nil
}

// ListUserGroups returns a paginated list of user group resources.
func (c *Cache) ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error) {
	return userGroupCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.UserGroups,
		col:      c.collections.userGroups,
	}.ListUserGroups(ctx, pageSize, nextKey)
}

// GetUserGroup returns the specified user group resources.
func (c userGroupCollection) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetUserGroup")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		group, err := c.upstream.GetUserGroup(ctx, name)
		return group, trace.Wrap(err)
	}

	group, err := rg.store.get(userGroupNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return group.Clone(), nil
}

// GetUserGroup returns the specified user group resources.
func (c *Cache) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	return userGroupCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.UserGroups,
		col:      c.collections.userGroups,
	}.GetUserGroup(ctx, name)
}
