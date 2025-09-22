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
	"cmp"
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type accessListIndex string

const (
	accessListNameIndex          accessListIndex = "name"
	accessListTitleIndex         accessListIndex = "title"
	accessListAuditNextDateIndex accessListIndex = "auditNextDate"
)

func newAccessListCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.AccessList, accessListIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}

	return &collection[*accesslist.AccessList, accessListIndex]{
		store: newStore(
			types.KindAccessList,
			(*accesslist.AccessList).Clone,
			map[accessListIndex]func(*accesslist.AccessList) string{
				// sorted by name
				accessListNameIndex: services.AccessListNameIndexKey,
				// sorted by title, sanitized.
				accessListTitleIndex: services.AccessListTitleIndexKey,
				// sorted by upcoming audit date. lists with no audit dates sorted to the back
				accessListAuditNextDateIndex: services.AccessListAuditDateIndexKey,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accesslist.AccessList, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListAccessLists))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *accesslist.AccessList {
			return &accesslist.AccessList{
				ResourceHeader: header.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: header.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (c *Cache) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessLists")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessLists)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, err := c.Config.AccessLists.GetAccessLists(ctx)
		return out, trace.Wrap(err)
	}

	out := make([]*accesslist.AccessList, 0, rg.store.len())
	for n := range rg.store.resources(accessListNameIndex, "", "") {
		out = append(out, n.Clone())
	}
	return out, nil
}

// ListAccessListsV2 returns a filtered and sorted paginated list of access lists.
func (c *Cache) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListsV2")
	defer span.End()

	index := accessListNameIndex

	var isDesc bool
	sortBy := req.GetSortBy()
	if sortBy != nil {
		isDesc = req.GetSortBy().IsDesc

		switch sortBy.Field {
		case "name", "":
			index = accessListNameIndex
		case "auditNextDate":
			index = accessListAuditNextDateIndex
		case "title":
			index = accessListTitleIndex
		default:
			return nil, "", trace.BadParameter("unsupported sort %q but expected name, title or auditNextDate", sortBy.Field)
		}
	}
	lister := genericLister[*accesslist.AccessList, accessListIndex]{
		cache:           c,
		collection:      c.collections.accessLists,
		isDesc:          isDesc,
		index:           index,
		defaultPageSize: 100,
		upstreamList: func(ctx context.Context, limit int, start string) ([]*accesslist.AccessList, string, error) {
			return c.Config.AccessLists.ListAccessListsV2(ctx, req)
		},
		filter: func(al *accesslist.AccessList) bool {
			return services.MatchAccessList(al, req.GetFilter())
		},
		nextToken: func(al *accesslist.AccessList) string {
			// ignore error because CreateAccessListNextKey only errors
			// if the index is invalid, which we already check above
			nextKey, _ := services.CreateAccessListNextKey(al, string(index))
			return nextKey
		},
	}
	out, next, err := lister.list(ctx, int(req.GetPageSize()), req.GetPageToken())
	return out, next, trace.Wrap(err)
}

// ListAccessLists returns a paginated list of access lists.
func (c *Cache) ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessLists")
	defer span.End()

	lister := genericLister[*accesslist.AccessList, accessListIndex]{
		cache:           c,
		collection:      c.collections.accessLists,
		index:           accessListNameIndex,
		defaultPageSize: 100,
		upstreamList:    c.Config.AccessLists.ListAccessLists,
		nextToken: func(t *accesslist.AccessList) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetAccessList returns the specified access list resource.
func (c *Cache) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessList")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[*accesslist.AccessList, accessListIndex]{
		cache:      c,
		collection: c.collections.accessLists,
		index:      accessListNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*accesslist.AccessList, error) {
			upstreamRead = true
			return c.Config.AccessLists.GetAccessList(ctx, s)
		},
	}
	out, err := getter.get(ctx, name)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if item, err := c.Config.AccessLists.GetAccessList(ctx, name); err == nil {
			return item, nil
		}
	}
	return out, trace.Wrap(err)
}

type accessListMemberIndex string

const (
	accessListMemberNameIndex accessListMemberIndex = "name"
	accessListMemberKindIndex accessListMemberIndex = "kind"
)

func accessListMemberNameIndexKey(r *accesslist.AccessListMember) string {
	return r.Spec.AccessList + "/" + r.GetName()
}

func newAccessListMemberCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.AccessListMember, accessListMemberIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}

	return &collection[*accesslist.AccessListMember, accessListMemberIndex]{
		store: newStore(
			types.KindAccessListMember,
			(*accesslist.AccessListMember).Clone,
			map[accessListMemberIndex]func(*accesslist.AccessListMember) string{
				accessListMemberNameIndex: func(r *accesslist.AccessListMember) string {
					return accessListMemberNameIndexKey(r)
				},
				accessListMemberKindIndex: func(r *accesslist.AccessListMember) string {
					return r.Spec.AccessList + "/" + r.Spec.MembershipKind + "/" + r.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accesslist.AccessListMember, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListAllAccessListMembers))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *accesslist.AccessListMember {
			return &accesslist.AccessListMember{
				ResourceHeader: header.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: header.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
				Spec: accesslist.AccessListMemberSpec{
					AccessList: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// CountAccessListMembers will count all access list members.
func (c *Cache) CountAccessListMembers(ctx context.Context, accessListName string) (uint32, uint32, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/CountAccessListMembers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		count, listCount, err := c.Config.AccessLists.CountAccessListMembers(ctx, accessListName)
		return count, listCount, trace.Wrap(err)
	}

	startUserKey := accessListName + "/" + accesslist.MembershipKindUser + "/"
	endUserKey := sortcache.NextKey(startUserKey)
	userCount := uint32(rg.store.count(accessListMemberKindIndex, startUserKey, endUserKey))

	startListKey := accessListName + "/" + accesslist.MembershipKindList + "/"
	endListKey := sortcache.NextKey(startListKey)
	listCount := uint32(rg.store.count(accessListMemberKindIndex, startListKey, endListKey))

	return userCount, listCount, nil
}

// ListAccessListMembers returns a paginated list of all access list members.
func (c *Cache) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListMembers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := c.Config.AccessLists.ListAccessListMembers(ctx, accessListName, pageSize, pageToken)
		return out, next, trace.Wrap(err)
	}

	start := cmp.Or(pageToken, accessListName)
	end := sortcache.NextKey(accessListName + "/")

	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	var out []*accesslist.AccessListMember
	for member := range rg.store.resources(accessListMemberNameIndex, start, end) {
		if len(out) == pageSize {
			return out, accessListName + "/" + member.GetName(), nil
		}

		out = append(out, member.Clone())
	}

	return out, "", trace.Wrap(err)
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (c *Cache) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAllAccessListMembers")
	defer span.End()

	lister := genericLister[*accesslist.AccessListMember, accessListMemberIndex]{
		cache:           c,
		collection:      c.collections.accessListMembers,
		index:           accessListMemberNameIndex,
		defaultPageSize: 200,
		upstreamList:    c.Config.AccessLists.ListAllAccessListMembers,
		nextToken: func(t *accesslist.AccessListMember) string {
			return accessListMemberNameIndexKey(t)
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetAccessListMember returns the specified access list member resource.
func (c *Cache) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessListMember")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, err := c.Config.AccessLists.GetAccessListMember(ctx, accessList, memberName)
		return out, trace.Wrap(err)
	}

	member, err := rg.store.get(accessListMemberNameIndex, accessList+"/"+memberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return member.Clone(), nil
}

type accessListReviewIndex string

const accessListReviewNameIndex = "name"

func newAccessListReviewCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.Review, accessListReviewIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}

	return &collection[*accesslist.Review, accessListReviewIndex]{
		store: newStore(
			types.KindAccessListReview,
			(*accesslist.Review).Clone,
			map[accessListReviewIndex]func(*accesslist.Review) string{
				accessListReviewNameIndex: func(r *accesslist.Review) string {
					return r.Spec.AccessList + "/" + r.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accesslist.Review, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListAllAccessListReviews))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *accesslist.Review {
			return &accesslist.Review{
				ResourceHeader: header.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: header.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
				Spec: accesslist.ReviewSpec{
					AccessList: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (c *Cache) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListReviews")
	defer span.End()

	lister := genericLister[*accesslist.Review, accessListReviewIndex]{
		cache:           c,
		collection:      c.collections.accessListReviews,
		index:           accessListReviewNameIndex,
		defaultPageSize: 200,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
			reviews, next, err := c.AccessLists.ListAccessListReviews(ctx, accessList, pageSize, pageToken)
			return reviews, next, trace.Wrap(err)
		},
		nextToken: func(t *accesslist.Review) string {
			return t.GetName()
		},
	}

	start := accessList
	end := sortcache.NextKey(accessList + "/")
	if pageToken != "" {
		start += "/" + pageToken
	}

	out, next, err := lister.listRange(ctx, pageSize, start, end)
	return out, next, trace.Wrap(err)
}
