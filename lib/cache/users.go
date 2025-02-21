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

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

func newUserCollection(u services.UsersService, w types.WatchKind) (*collection[types.User, *resourceStore[types.User], *userUpstream], error) {
	if u == nil {
		return nil, trace.BadParameter("missing parameter UsersService")
	}

	return &collection[types.User, *resourceStore[types.User], *userUpstream]{
		store: newResourceStore(map[string]func(types.User) string{
			"name": func(u types.User) string {
				return u.GetName()
			},
		}),
		upstream: &userUpstream{UsersService: u},
		watch:    w,
	}, nil
}

type userUpstream struct {
	services.UsersService
}

func (c userUpstream) getAll(ctx context.Context, loadSecrets bool) ([]types.User, error) {
	return c.UsersService.GetUsers(ctx, loadSecrets)
}

// GetUser is a part of auth.Cache implementation.
func (c *Cache) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUser")
	defer span.End()

	collection := c.collections.users

	if withSecrets { // cache never tracks user secrets
		return collection.upstream.GetUser(ctx, name, withSecrets)
	}

	rg, err := acquireReadGuard(c, collection.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		user, err := collection.upstream.GetUser(ctx, name, withSecrets)
		return user, trace.Wrap(err)
	}

	u, err := collection.store.get("name", name)
	if err != nil {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if trace.IsNotFound(err) {
			if user, err := c.Config.Users.GetUser(ctx, name, withSecrets); err == nil {
				return user, nil
			}
		}
		return nil, trace.Wrap(err)
	}

	if withSecrets {
		return u.Clone(), nil
	}

	return u.WithoutSecrets().(types.User), nil
}

// GetUsers is a part of auth.Cache implementation
func (c *Cache) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUsers")
	defer span.End()

	collection := c.collections.users

	if withSecrets { // cache never tracks user secrets
		return collection.upstream.GetUsers(ctx, withSecrets)
	}

	rg, err := acquireReadGuard(c, collection.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := collection.upstream.GetUsers(ctx, withSecrets)
		return users, trace.Wrap(err)
	}

	var users []types.User
	for u := range collection.store.iterate("name", "", "") {
		if withSecrets {
			users = append(users, u.Clone())
		} else {
			users = append(users, u.WithoutSecrets().(types.User))
		}
	}

	return users, nil
}

// ListUsers returns a page of users.
func (c *Cache) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListUsers")
	defer span.End()

	collection := c.collections.users

	if req.WithSecrets { // cache never tracks user secrets
		rsp, err := collection.upstream.ListUsers(ctx, req)
		return rsp, trace.Wrap(err)
	}

	rg, err := acquireReadGuard(c, collection.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		resp, err := collection.upstream.ListUsers(ctx, req)
		return resp, trace.Wrap(err)
	}

	var resp userspb.ListUsersResponse
	for u := range collection.store.iterate("name", req.PageToken, "") {
		uv2, ok := u.(*types.UserV2)
		if !ok {
			continue
		}

		if req.Filter != nil && !req.Filter.Match(uv2) {
			continue
		}

		if len(resp.Users) == int(req.PageSize) {
			key := backend.RangeEnd(backend.ExactKey(u.GetName())).String()
			resp.NextPageToken = strings.Trim(key, string(backend.Separator))
			break
		}

		if req.WithSecrets {
			resp.Users = append(resp.Users, u.Clone().(*types.UserV2))
		} else {
			resp.Users = append(resp.Users, u.WithoutSecrets().(*types.UserV2))
		}
	}
	return &resp, nil
}
