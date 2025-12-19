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

type MemberCache struct {
	cache *cache.Cache[*scopedaccessv1.ScopedAccessListMember, string]
}

func memberKey(listName, memberName string) string {
	return listName + "/" + memberName
}

func NewMemberCache() *MemberCache {
	return &MemberCache{
		cache: cache.Must(cache.Config[*scopedaccessv1.ScopedAccessListMember, string]{
			Scope: func(member *scopedaccessv1.ScopedAccessListMember) string {
				return member.GetScope()
			},
			Key: func(member *scopedaccessv1.ScopedAccessListMember) string {
				return memberKey(member.GetSpec().GetAccessList(), member.GetMetadata().GetName())
			},
			Clone: proto.CloneOf[*scopedaccessv1.ScopedAccessListMember],
		}),
	}
}

func (c *MemberCache) GetScopedAccessListMember(ctx context.Context, req *scopedaccessv1.GetScopedAccessListMemberRequest) (*scopedaccessv1.GetScopedAccessListMemberResponse, error) {
	if req.GetScopedAccessList() == "" {
		return nil, trace.BadParameter("missing scoped access list name in member get request")
	}
	if req.GetMemberName() == "" {
		return nil, trace.BadParameter("missing scoped access list member name in get request")
	}

	member, ok := c.cache.Get(memberKey(req.GetScopedAccessList(), req.GetMemberName()))
	if !ok {
		return nil, trace.NotFound("scoped access list member %v not found", req.GetMemberName())
	}

	return &scopedaccessv1.GetScopedAccessListMemberResponse{
		Member: member,
	}, nil
}

func (c *MemberCache) ListScopedAccessListMembers(ctx context.Context, req *scopedaccessv1.ListScopedAccessListMembersRequest) (*scopedaccessv1.ListScopedAccessListMembersResponse, error) {
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

	var out []*scopedaccessv1.ScopedAccessListMember
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range getter(scope, c.cache.WithCursor(cursor)) {
		for member := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key:   member.GetMetadata().GetName(),
				}
				break Outer
			}
			out = append(out, member)
		}
	}

	var nextPageToken string
	if !nextCursor.IsZero() {
		nextPageToken, err = cache.EncodeStringCursor(nextCursor)
		if err != nil {
			return nil, trace.Errorf("failed to encode cursor %+v: %w (this is a bug)", nextCursor, err)
		}
	}

	return &scopedaccessv1.ListScopedAccessListMembersResponse{
		Members:       out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new access list member to the cache. It will overwrite any existing access list with the same name.
func (c *MemberCache) Put(member *scopedaccessv1.ScopedAccessListMember) error {
	if err := scopedaccess.WeakValidateAccessListMember(member); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(member)
	return nil
}

// Del removes an access list member from the cache by name.
func (c *MemberCache) Delete(listName, memberName string) {
	c.cache.Del(memberKey(listName, memberName))
}
