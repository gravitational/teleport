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
	"strings"

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

func newUserCollection(upstream services.UsersService, w types.WatchKind) (*collection[types.User, userIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter UsersService")
	}

	return &collection[types.User, userIndex]{
		store: newStore(
			types.KindUser,
			types.User.Clone,
			map[userIndex]func(types.User) string{
				userNameIndex: func(u types.User) string {
					return userNameIndexKeyForName(u.GetName())
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.User, error) {
			fn := func(ctx context.Context, pageSize int, token string) ([]types.User, string, error) {
				return listUsersPage(ctx, upstream, loadSecrets, pageSize, token)
			}

			// Use clientutils for auto pagesize backoff.
			out, err := stream.Collect(clientutils.Resources(ctx, fn))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.User {
			return &types.UserV2{
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

func listUsersPage(
	ctx context.Context, upstream services.UsersService, withSecrets bool, pageSize int, token string,
) ([]types.User, string, error) {
	rsp, err := upstream.ListUsers(ctx, &userspb.ListUsersRequest{
		WithSecrets: withSecrets,
		PageSize:    int32(pageSize),
		PageToken:   token,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out := make([]types.User, 0, len(rsp.GetUsers()))
	for _, u := range rsp.GetUsers() {
		out = append(out, u)
	}
	return out, rsp.GetNextPageToken(), nil
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
		user, err := c.Config.Users.GetUser(ctx, name, withSecrets)
		return user, trace.Wrap(err)
	}

	u, err := rg.store.get(userNameIndex, userNameIndexKeyForName(name))
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

	if withSecrets {
		return u.Clone(), nil
	}

	return u.WithoutSecrets().(types.User), nil
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
	for u := range rg.store.resources(userNameIndex, "", "") {
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
	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	var resp userspb.ListUsersResponse
	var start string
	if pageToken := req.GetPageToken(); pageToken != "" {
		start = userNameIndexKeyForName(pageToken)
	}
	for u := range rg.store.resources(userNameIndex, start, "") {
		uv2, ok := u.(*types.UserV2)
		if !ok {
			continue
		}

		if req.Filter != nil && !req.Filter.Match(uv2) {
			continue
		}

		if len(resp.Users) == pageSize {
			resp.NextPageToken = u.GetName()
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

// RangeUsers returns users within the range [start, end), without secrets.
func (c *Cache) RangeUsers(ctx context.Context, start, end string) iter.Seq2[types.User, error] {
	lister := genericLister[types.User, userIndex]{
		cache:      c,
		collection: c.collections.users,
		index:      userNameIndex,
		// Tokens move through the lister as index keys, while the upstream
		// takes and returns bare user names.
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.User, string, error) {
			users, next, err := listUsersPage(ctx, c.Config.Users, false, pageSize, strings.TrimSuffix(pageToken, "/params"))
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			if next != "" {
				next = userNameIndexKeyForName(next)
			}
			return users, next, nil
		},
		nextToken: func(u types.User) string {
			return userNameIndexKeyForName(u.GetName())
		},
	}
	if start != "" {
		start = userNameIndexKeyForName(start)
	}
	if end != "" {
		end = userNameIndexKeyForName(end)
	}

	return func(yield func(types.User, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeUsers")
		defer span.End()

		for u, err := range lister.Range(ctx, start, end) {
			if err != nil {
				yield(nil, err)
				return
			}

			// The auth cache stores users with secrets.
			if !yield(u.WithoutSecrets().(types.User), nil) {
				return
			}
		}
	}
}
