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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type userGroupIndex string

const userGroupNameIndex userGroupIndex = "name"

func newUserGroupCollection(u services.UserGroups, w types.WatchKind) (*collection[types.UserGroup, userGroupIndex], error) {
	if u == nil {
		return nil, trace.BadParameter("missing parameter UserGroups")
	}

	return &collection[types.UserGroup, userGroupIndex]{
		store: newStore(
			types.UserGroup.Clone,
			map[userGroupIndex]func(types.UserGroup) string{
				userGroupNameIndex: types.UserGroup.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.UserGroup, error) {
			var startKey string
			var groups []types.UserGroup
			for {
				resp, next, err := u.ListUserGroups(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				groups = append(groups, resp...)
				if next == "" {
					break
				}
				startKey = next
			}
			return groups, nil
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

// ListUserGroups returns a paginated list of user group resources.
func (c *Cache) ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListUserGroups")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.userGroups)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		group, nextKey, err := c.Config.UserGroups.ListUserGroups(ctx, pageSize, nextKey)
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

// GetUserGroup returns the specified user group resources.
func (c *Cache) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserGroup")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.userGroups)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		group, err := c.Config.UserGroups.GetUserGroup(ctx, name)
		return group, trace.Wrap(err)
	}

	group, err := rg.store.get(userGroupNameIndex, name)
	return group.Clone(), trace.Wrap(err)
}
