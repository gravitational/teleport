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

	"github.com/gravitational/trace"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/defaults"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type accessListIndex string

const (
	accessListNameIndex          accessListIndex = "name"
	accessListTitleIndex         accessListIndex = "title"
	accessListAuditNextDateIndex accessListIndex = "auditNextDate"
)

type accessListMatchingLister interface {
	ListMatchingAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsV2Request, searchTermMatchers ...services.AccessListSearchTermMatcherFunc) ([]*accesslist.AccessList, string, error)
}

func newAccessListCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.AccessList, accessListIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}
	scopeFilter := w.ScopeFilter.ToProto()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, trace.Wrap(err)
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
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
				return upstream.ListAccessListsV2(ctx, accesslistv1.ListAccessListsV2Request_builder{
					PageSize:    int32(pageSize),
					PageToken:   pageToken,
					ScopeFilter: scopeFilter,
				}.Build())
			}))
			return out, trace.Wrap(err)
		},
		// Note: scoped access list refs are never sent in a ResourceHeader, they
		// use *accesslist.AccessList for delete events, so this transform will
		// not apply to scoped access lists.
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

// GetAccessLists returns a list of all unscoped access lists.
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
	for n := range rg.store.resources(accessListNameIndex, "", scopes.ResourceCursorScopedStart()) {
		out = append(out, n.Clone())
	}
	return out, nil
}

// ListAccessListsV2 returns a filtered and sorted paginated list of access lists.
func (c *Cache) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	return c.ListMatchingAccessLists(ctx, req)
}

// ListMatchingAccessLists returns a filtered and sorted paginated list of access lists.
// Search term matchers are evaluated in order for terms that do not match stored access list fields.
func (c *Cache) ListMatchingAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsV2Request, searchTermMatchers ...services.AccessListSearchTermMatcherFunc) ([]*accesslist.AccessList, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListMatchingAccessLists")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(req.GetScopeFilter()); err != nil {
		return nil, "", trace.Wrap(err)
	}

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
			if matchingLister, ok := c.Config.AccessLists.(accessListMatchingLister); ok {
				return matchingLister.ListMatchingAccessLists(ctx, req, searchTermMatchers...)
			}
			return c.Config.AccessLists.ListAccessListsV2(ctx, req)
		},
		filter: func(al *accesslist.AccessList) bool {
			return services.MatchAccessList(al, req.GetFilter(), searchTermMatchers...) && scopes.MatchScope(scopeFilter, al.GetScope())
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
	out, next, err := lister.listRange(ctx, pageSize, pageToken, scopes.ResourceCursorScopedStart())
	return out, next, trace.Wrap(err)
}

// GetAccessList returns the specified access list resource.
func (c *Cache) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return c.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Name: name,
	}.Build())
}

// GetAccessListV2 returns the specified access list resource.
func (c *Cache) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessListV2")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[*accesslist.AccessList, accessListIndex]{
		cache:      c,
		collection: c.collections.accessLists,
		index:      accessListNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*accesslist.AccessList, error) {
			upstreamRead = true
			return c.Config.AccessLists.GetAccessListV2(ctx, req)
		},
	}
	out, err := getter.get(ctx, accessListCursor(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}))
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if item, err := c.Config.AccessLists.GetAccessListV2(ctx, req); err == nil {
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
	if r.GetScope() == "" {
		return r.Spec.AccessList + "/" + r.GetName()
	}
	listSQN, listErr := accesslists.ParentListOf(r)
	memberSQN, memberErr := accesslists.MemberScopeQualifiedName(r)
	if listErr != nil || memberErr != nil {
		return "~invalid/" + base32Encode(string(ordered.Encode(r.Spec.AccessList, r.GetName())))
	}
	return namedAccessListMemberNameIndexKey(listSQN.ToScopesQualifiedName(), memberSQN.ToScopesQualifiedName())
}

func namedAccessListMemberNameIndexKey(listName, memberName scopes.QualifiedName) string {
	return scopes.MakeNestedResourceCursor(listName, memberName)
}

func accessListCursor(listName scopes.QualifiedName) string {
	return scopes.MakeResourceCursor(listName.Scope, listName.Name)
}

func accessListMemberKindIndexKey(r *accesslist.AccessListMember) string {
	if r.GetScope() == "" {
		return r.Spec.AccessList + "/" + r.Spec.MembershipKind + "/" + r.GetName()
	}

	listSQN, listErr := scopes.ParseQualifiedName(r.Spec.AccessList)
	memberSQN, memberErr := accesslists.MemberScopeQualifiedName(r)
	if listErr != nil || memberErr != nil {
		return "~invalid/" + base32Encode(string(ordered.Encode(r.Spec.AccessList, r.Spec.MembershipKind, r.GetName())))
	}

	listCursor := scopes.MakeResourceCursor(listSQN.Scope, listSQN.Name)
	encodedMemberScope := scopes.EncodeForResourceCursor(memberSQN.Scope)
	return listCursor + "/" + r.Spec.MembershipKind + "/" + encodedMemberScope + "/" + memberSQN.Name
}

func newAccessListMemberCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.AccessListMember, accessListMemberIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}
	scopeFilter := w.ScopeFilter.ToProto()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, trace.Wrap(err)
	}

	return &collection[*accesslist.AccessListMember, accessListMemberIndex]{
		store: newStore(
			types.KindAccessListMember,
			(*accesslist.AccessListMember).Clone,
			map[accessListMemberIndex]func(*accesslist.AccessListMember) string{
				accessListMemberNameIndex: accessListMemberNameIndexKey,
				accessListMemberKindIndex: accessListMemberKindIndexKey,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accesslist.AccessListMember, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
				return upstream.ListAllAccessListMembersV2(ctx, accesslistv1.ListAllAccessListMembersRequest_builder{
					PageSize:    int32(pageSize),
					PageToken:   pageToken,
					ScopeFilter: scopeFilter,
				}.Build())
			}))
			return out, trace.Wrap(err)
		},
		// Note: scoped member refs are never sent in a ResourceHeader, they
		// use *accesslist.AccessListMember for delete events, so this
		// transform will not apply to scoped members.
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
	return c.CountAccessListMembersV2(ctx, accesslistv1.CountAccessListMembersRequest_builder{
		AccessListName: accessListName,
	}.Build())
}

// CountAccessListMembersV2 will count all access list members.
func (c *Cache) CountAccessListMembersV2(ctx context.Context, req *accesslistv1.CountAccessListMembersRequest) (uint32, uint32, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/CountAccessListMembersV2")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		count, listCount, err := c.Config.AccessLists.CountAccessListMembersV2(ctx, req)
		return count, listCount, trace.Wrap(err)
	}

	listCursor := accessListCursor(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessListName(),
	})

	startUserKey := listCursor + "/" + accesslist.MembershipKindUser + "/"
	endUserKey := sortcache.NextKey(startUserKey)
	userCount := uint32(rg.store.count(accessListMemberKindIndex, startUserKey, endUserKey))

	startListKey := listCursor + "/" + accesslist.MembershipKindList + "/"
	endListKey := sortcache.NextKey(startListKey)
	listCount := uint32(rg.store.count(accessListMemberKindIndex, startListKey, endListKey))
	startScopedListKey := listCursor + "/" + accesslist.MembershipKindScopedList + "/"
	endScopedListKey := sortcache.NextKey(startScopedListKey)
	listCount += uint32(rg.store.count(accessListMemberKindIndex, startScopedListKey, endScopedListKey))

	return userCount, listCount, nil
}

// ListAccessListMembers returns a paginated list of all access list members.
func (c *Cache) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	return c.ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessList: accessListName,
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
	}.Build())
}

// ListAccessListMembersV2 returns a paginated list of all access list members.
func (c *Cache) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (members []*accesslist.AccessListMember, nextToken string, err error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListMembers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := c.Config.AccessLists.ListAccessListMembersV2(ctx, req)
		return out, next, trace.Wrap(err)
	}

	return listAccessListMembers(rg.store, req)
}

func listAccessListMembers(store *store[*accesslist.AccessListMember, accessListMemberIndex], req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	listCursor := accessListCursor(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})

	// The ending "/" is very important here, otherwise we can start listing members of access
	// lists which names are prefixed with this access list name. E.g. we'd list members of
	// "dev-suffix" for list "dev".
	start := listCursor + "/"
	current := start + req.GetPageToken()
	end := sortcache.NextKey(start)

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	var out []*accesslist.AccessListMember
	for member := range store.resources(accessListMemberNameIndex, current, end) {
		if len(out) == pageSize {
			key := accessListMemberNameIndexKey(member)
			return out, key[len(start):], nil
		}

		out = append(out, member.Clone())
	}

	return out, "", nil
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (c *Cache) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return c.ListAllAccessListMembersV2(ctx, accesslistv1.ListAllAccessListMembersRequest_builder{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_UNSCOPED,
		}.Build(),
	}.Build())

}

// ListAllAccessListMembersV2 returns a paginated list of all access list members for all access lists.
func (c *Cache) ListAllAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAllAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAllAccessListMembersV2")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(req.GetScopeFilter()); err != nil {
		return nil, "", trace.Wrap(err)
	}

	lister := genericLister[*accesslist.AccessListMember, accessListMemberIndex]{
		cache:           c,
		collection:      c.collections.accessListMembers,
		index:           accessListMemberNameIndex,
		defaultPageSize: 200,
		upstreamList: func(ctx context.Context, _ int, _ string) ([]*accesslist.AccessListMember, string, error) {
			return c.Config.AccessLists.ListAllAccessListMembersV2(ctx, req)
		},
		nextToken: func(t *accesslist.AccessListMember) string {
			return accessListMemberNameIndexKey(t)
		},
		filter: func(member *accesslist.AccessListMember) bool {
			return scopes.MatchScope(scopeFilter, member.GetScope())
		},
	}
	out, next, err := lister.list(ctx, int(req.GetPageSize()), req.GetPageToken())
	return out, next, trace.Wrap(err)
}

// GetAccessListMember returns the specified access list member resource.
func (c *Cache) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	return c.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessList: accessList,
		MemberName: memberName,
	}.Build())
}

// GetAccessListMemberV2 returns the specified access list member resource.
func (c *Cache) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessListMember")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, err := c.Config.AccessLists.GetAccessListMemberV2(ctx, req)
		return out, trace.Wrap(err)
	}

	listName := scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	}
	memberName := scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	}
	key := namedAccessListMemberNameIndexKey(listName, memberName)

	member, err := rg.store.get(accessListMemberNameIndex, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return member.Clone(), nil
}

type accessListMembersListerStore struct {
	store *store[*accesslist.AccessListMember, accessListMemberIndex]
}

// ListAccessListMembers implements [accesslists.AccessListMembersLister].
func (l accessListMembersListerStore) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	return listAccessListMembers(l.store, accesslistv1.ListAccessListMembersRequest_builder{
		AccessList: accessListName,
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
	}.Build())
}

func (l accessListMembersListerStore) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (members []*accesslist.AccessListMember, nextToken string, err error) {
	return listAccessListMembers(l.store, req)
}

// GetAccessListOwners returns the owners of the specified access list, including those inherited.
func (c *Cache) GetAccessListOwners(ctx context.Context, accessListName string) ([]*accesslist.Owner, error) {
	return c.GetAccessListOwnersV2(ctx, accesslistv1.GetAccessListOwnersRequest_builder{
		AccessList: accessListName,
	}.Build())
}

// GetAccessListOwnersV2 returns the owners of the specified access list, including those inherited.
func (c *Cache) GetAccessListOwnersV2(ctx context.Context, req *accesslistv1.GetAccessListOwnersRequest) ([]*accesslist.Owner, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessListOwnersV2")
	defer span.End()

	accessList, err := c.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rg, err := acquireReadGuard(c, c.collections.accessListMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		return accesslists.GetOwnersFor(ctx, accessList, c.Config.AccessLists)
	}

	return accesslists.GetOwnersFor(ctx, accessList, accessListMembersListerStore{store: rg.store})
}

type accessListReviewIndex string

const accessListReviewNameIndex = "name"

func accessListReviewNameIndexKey(r *accesslist.Review) string {
	if r.GetScope() == "" {
		return r.Spec.AccessList + "/" + r.GetName()
	}

	listSQN, err := accesslists.ReviewedList(r)
	if err != nil {
		return "~invalid/" + base32Encode(string(ordered.Encode(r.Spec.AccessList, r.GetName())))
	}

	listCursor := accessListCursor(listSQN.ToScopesQualifiedName())
	return listCursor + "/" + r.GetName()
}

func newAccessListReviewCollection(upstream services.AccessLists, w types.WatchKind) (*collection[*accesslist.Review, accessListReviewIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AccessLists")
	}
	scopeFilter := w.ScopeFilter.ToProto()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, trace.Wrap(err)
	}

	return &collection[*accesslist.Review, accessListReviewIndex]{
		store: newStore(
			types.KindAccessListReview,
			(*accesslist.Review).Clone,
			map[accessListReviewIndex]func(*accesslist.Review) string{
				accessListReviewNameIndex: accessListReviewNameIndexKey,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*accesslist.Review, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
				return upstream.ListAllAccessListReviewsV2(ctx, accesslistv1.ListAllAccessListReviewsRequest_builder{
					PageSize:    int32(pageSize),
					NextToken:   pageToken,
					ScopeFilter: scopeFilter,
				}.Build())
			}))
			return out, trace.Wrap(err)
		},
		// Note: scoped review refs are never sent in a ResourceHeader, they
		// use *accesslist.Review for delete events, so this transform will not apply
		// to scoped reviews.
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
	return c.ListAccessListReviewsV2(ctx, accesslistv1.ListAccessListReviewsRequest_builder{
		AccessList: accessList,
		PageSize:   int32(pageSize),
		NextToken:  pageToken,
	}.Build())
}

// ListAccessListReviewsV2 will list access list reviews for a particular access list.
func (c *Cache) ListAccessListReviewsV2(ctx context.Context, req *accesslistv1.ListAccessListReviewsRequest) ([]*accesslist.Review, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListReviews")
	defer span.End()

	lister := genericLister[*accesslist.Review, accessListReviewIndex]{
		cache:           c,
		collection:      c.collections.accessListReviews,
		index:           accessListReviewNameIndex,
		defaultPageSize: 200,
		upstreamList: func(ctx context.Context, _ int, _ string) ([]*accesslist.Review, string, error) {
			return c.AccessLists.ListAccessListReviewsV2(ctx, req)
		},
		nextToken: func(t *accesslist.Review) string {
			return t.GetName()
		},
	}

	listCursor := accessListCursor(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})

	start := listCursor + "/"
	end := sortcache.NextKey(start)
	pageToken := req.GetNextToken()
	if pageToken != "" {
		start += pageToken
	}

	out, next, err := lister.listRange(ctx, int(req.GetPageSize()), start, end)
	return out, next, trace.Wrap(err)
}
