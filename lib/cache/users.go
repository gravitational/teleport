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
	"iter"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type userIndex struct{}

var userNameIndex userIndex

func userNameIndexKeyForName(name string) string {
	// this matches the order of the users in the backend, which is
	// unfortunately not the correct order (user "foo" should come earlier than
	// user "foo-bar", but with a "/params" suffix the order is the other way
	// around)
	return name + "/params"
}

type cachedUser = cachedGogoResource[*types.UserV2, types.UserV2]

func newCachedUser(user *types.UserV2) (*cachedUser, error) {
	return newCachedGogoResource(userNameIndexKeyForName(user.Metadata.Name), user)
}

func newUserCollection(upstream services.UsersService, w types.WatchKind) (*collection[*cachedUser, userIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter UsersService")
	}

	return &collection[*cachedUser, userIndex]{
		store: newStore(
			types.KindUser,
			(*cachedUser).clone,
			map[userIndex]func(*cachedUser) string{
				userNameIndex: func(cu *cachedUser) string { return cu.key },
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*cachedUser, error) {
			// the cache never holds secrets
			_ = loadSecrets

			pageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*types.UserV2, string, error) {
				resp, err := upstream.ListUsers(ctx, userspb.ListUsersRequest_builder{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return resp.GetUsers(), resp.GetNextPageToken(), nil
			}
			return stream.Collect(stream.TryMap(clientutils.Resources(ctx, pageFunc), newCachedUser))
		},
		headerTransform: func(hdr *types.ResourceHeader) *cachedUser {
			return &cachedUser{key: userNameIndexKeyForName(hdr.Metadata.Name)}
		},
		watch: w,
		resourcePutTransform: func(rsc types.Resource) (*cachedUser, error) {
			user, _ := rsc.(*types.UserV2)
			if user == nil {
				return nil, trace.BadParameter("unexpected type %T (expected %T, this is a bug)", rsc, user)
			}
			return newCachedUser(user)
		},
	}, nil
}

// GetUser is a part of auth.Cache implementation.
func (c *Cache) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUser")
	defer span.End()

	if withSecrets { // cache never tracks user secrets
		return c.Config.Users.GetUser(ctx, name, withSecrets)
	}

	rg, err := acquireReadGuard(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		return c.Config.Users.GetUser(ctx, name, withSecrets)
	}

	cu, err := rg.store.get(userNameIndex, userNameIndexKeyForName(name))
	if err != nil {
		// release read lock early
		rg.Release()

		// fallback is sane because method is never used
		// in construction of derivative caches.
		if trace.IsNotFound(err) {
			if user, err := c.Config.Users.GetUser(ctx, name, withSecrets); err == nil {
				return user, nil
			}
		}
		return nil, trace.Wrap(err)
	}
	return cu.cloneResource(), nil
}

// GetUsers is a part of auth.Cache implementation
func (c *Cache) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUsers")
	defer span.End()

	if withSecrets { // cache never tracks user secrets
		return c.Config.Users.GetUsers(ctx, withSecrets)
	}

	rg, err := acquireReadGuard(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := c.Config.Users.GetUsers(ctx, withSecrets)
		return users, trace.Wrap(err)
	}

	users := make([]types.User, 0, rg.store.len())
	for cu := range rg.store.resources(userNameIndex, "", "") {
		users = append(users, cu.cloneResource())
	}

	return users, nil
}

// ListUsers returns a page of users.
func (c *Cache) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListUsers")
	defer span.End()

	if req.WithSecrets { // cache never tracks user secrets
		rsp, err := c.Config.Users.ListUsers(ctx, req)
		return rsp, trace.Wrap(err)
	}

	rg, err := acquireReadGuard(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		resp, err := c.Config.Users.ListUsers(ctx, req)
		return resp, trace.Wrap(err)
	}

	// Adjust page size, so it can't be too large.
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	var resp userspb.ListUsersResponse
	var start string
	if pageToken := req.GetPageToken(); pageToken != "" {
		start = userNameIndexKeyForName(pageToken)
	}
	for cu := range rg.store.resources(userNameIndex, start, "") {
		if req.HasFilter() && !req.GetFilter().Match(cu.rsc) {
			continue
		}

		if len(resp.GetUsers()) == pageSize {
			resp.SetNextPageToken(cu.rsc.Metadata.Name)
			break
		}

		resp.SetUsers(append(resp.GetUsers(), cu.cloneResource()))
	}
	return &resp, nil
}

// RangeUsers returns users within the range [start, end), without secrets.
func (c *Cache) RangeUsers(ctx context.Context, start, end string) iter.Seq2[types.User, error] {
	pageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*types.UserV2, string, error) {
		resp, err := c.ListUsers(ctx, userspb.ListUsersRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
		}.Build())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.GetUsers(), resp.GetNextPageToken(), nil
	}

	// users are sorted by userNameIndexKeyForName, so to check the end we have
	// to tweak the end parameter and provide the appropriate keyFunc
	if end != "" {
		end = userNameIndexKeyForName(end)
	}
	keyFunc := func(user *types.UserV2) string {
		return userNameIndexKeyForName(user.Metadata.Name)
	}

	return stream.Map(
		clientutils.RangeResources(ctx, start, end, pageFunc, keyFunc),
		func(user *types.UserV2) types.User { return user },
	)
}
