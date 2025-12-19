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

package accesslists

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache"
)

const (
	defaultPageSize = 256
	maxPageSize     = 1024
)

type AccessListCache struct {
	cache *cache.Cache[*scopedaccessv1.ScopedAccessList, string]
}

func NewAccessListCache() *AccessListCache {
	return &AccessListCache{
		cache: cache.Must(cache.Config[*scopedaccessv1.ScopedAccessList, string]{
			Scope: func(list *scopedaccessv1.ScopedAccessList) string {
				return list.GetScope()
			},
			Key: func(list *scopedaccessv1.ScopedAccessList) string {
				return list.GetMetadata().GetName()
			},
			Clone: proto.CloneOf[*scopedaccessv1.ScopedAccessList],
		}),
	}
}

func (c *AccessListCache) GetScopedAccessList(ctx context.Context, req *scopedaccessv1.GetScopedAccessListRequest) (*scopedaccessv1.GetScopedAccessListResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped access list name in request")
	}

	list, ok := c.cache.Get(req.GetName())
	if !ok {
		return nil, trace.NotFound("scoped access list %v not found", req.GetName())
	}

	return &scopedaccessv1.GetScopedAccessListResponse{
		List: list,
	}, nil
}

func (c *AccessListCache) ListScopedAccessLists(ctx context.Context, req *scopedaccessv1.ListScopedAccessListsRequest) (*scopedaccessv1.ListScopedAccessListsResponse, error) {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	cursor, err := cache.DecodeStringCursor(req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// resources subject to policy scope root is basically the scoped resource equivalent
	// of a "get all". this is a reasonable default for most queries.
	getter := c.cache.ResourcesSubjectToPolicyScope
	scope := scopes.Root

	var out []*scopedaccessv1.ScopedAccessList
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range getter(scope, c.cache.WithCursor(cursor)) {
		for list := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key:   list.GetMetadata().GetName(),
				}
				break Outer
			}
			out = append(out, list)
		}
	}

	var nextPageToken string
	if !nextCursor.IsZero() {
		nextPageToken, err = cache.EncodeStringCursor(nextCursor)
		if err != nil {
			return nil, trace.Errorf("failed to encode cursor %+v: %w (this is a bug)", nextCursor, err)
		}
	}

	return &scopedaccessv1.ListScopedAccessListsResponse{
		Lists:         out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new access list to the cache. It will overwrite any existing access list with the same name.
func (c *AccessListCache) Put(list *scopedaccessv1.ScopedAccessList) error {
	if err := scopedaccess.WeakValidateAccessList(list); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(list)
	return nil
}

// Del removes an access list from the cache by name.
func (c *AccessListCache) Delete(name string) {
	c.cache.Del(name)
}
