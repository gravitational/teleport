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

	if withSecrets { // cache never tracks user secrets
		return c.Config.Users.GetUser(ctx, name, withSecrets)
	}

	user, err := readCachedResource(
		ctx,
		c,
		c.collections.users,
		func(ctx context.Context, store *resourceStore[types.User]) (types.User, error) {
			u, err := store.get("name", name)
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
		},
		func(ctx context.Context, upstream *userUpstream) (types.User, error) {
			user, err := upstream.GetUser(ctx, name, withSecrets)
			return user, trace.Wrap(err)
		})

	return user, trace.Wrap(err)
}

// GetUsers is a part of auth.Cache implementation
func (c *Cache) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUsers")
	defer span.End()

	if withSecrets { // cache never tracks user secrets
		return c.Users.GetUsers(ctx, withSecrets)
	}

	users, err := readCachedResource(
		ctx,
		c,
		c.collections.users,
		func(_ context.Context, store *resourceStore[types.User]) ([]types.User, error) {
			var users []types.User
			for u := range store.iterate("name", "", "") {
				if withSecrets {
					users = append(users, u.Clone())
				} else {
					users = append(users, u.WithoutSecrets().(types.User))
				}
			}

			return users, nil
		},
		func(ctx context.Context, upstream *userUpstream) ([]types.User, error) {
			users, err := upstream.GetUsers(ctx, withSecrets)
			return users, trace.Wrap(err)
		})

	return users, trace.Wrap(err)
}

// ListUsers returns a page of users.
func (c *Cache) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListUsers")
	defer span.End()

	if req.WithSecrets { // cache never tracks user secrets
		rsp, err := c.Users.ListUsers(ctx, req)
		return rsp, trace.Wrap(err)
	}

	users, err := readCachedResource(
		ctx,
		c,
		c.collections.users,
		func(_ context.Context, store *resourceStore[types.User]) (*userspb.ListUsersResponse, error) {
			var resp userspb.ListUsersResponse
			for u := range store.iterate("name", req.PageToken, "") {
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
		},
		func(ctx context.Context, upstream *userUpstream) (*userspb.ListUsersResponse, error) {
			resp, err := upstream.ListUsers(ctx, req)
			return resp, trace.Wrap(err)
		})

	return users, trace.Wrap(err)
}
