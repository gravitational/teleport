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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestAccessList tests that CRUD operations on access list resources are
// replicated from the backend to the cache.
func TestAccessList(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClock()

	testResources(t, p, testFuncs[*accesslist.AccessList]{
		newResource: func(name string) (*accesslist.AccessList, error) {
			return newAccessList(t, name, clock), nil
		},
		create: func(ctx context.Context, item *accesslist.AccessList) error {
			_, err := p.accessLists.UpsertAccessList(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.accessLists.ListAccessLists,
		cacheGet:  p.cache.GetAccessList,
		cacheList: p.cache.ListAccessLists,
		update: func(ctx context.Context, item *accesslist.AccessList) error {
			_, err := p.accessLists.UpsertAccessList(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessLists,
	})
}

// TestAccessListMembers tests that CRUD operations on access list member resources are
// replicated from the backend to the cache.
func TestAccessListMembers(t *testing.T) {
	t.Parallel()

	const numMembers = 32

	p := newTestPack(t, ForAuth, memoryBackend(true))
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClock()

	al, err := p.accessLists.UpsertAccessList(context.Background(), newAccessList(t, "access-list", clock))
	require.NoError(t, err)

	testResources(t, p, testFuncs[*accesslist.AccessListMember]{
		newResource: func(name string) (*accesslist.AccessListMember, error) {
			return newAccessListMember(t, al.GetName(), name), nil
		},
		create: func(ctx context.Context, item *accesslist.AccessListMember) error {
			_, err := p.accessLists.UpsertAccessListMember(ctx, item)
			return trace.Wrap(err)
		},
		list: p.accessLists.ListAllAccessListMembers,
		cacheGet: func(ctx context.Context, name string) (*accesslist.AccessListMember, error) {
			return p.cache.GetAccessListMember(ctx, al.GetName(), name)
		},
		cacheList: func(ctx context.Context, pageSize int, startKey string) ([]*accesslist.AccessListMember, string, error) {
			return p.cache.ListAccessListMembers(ctx, al.GetName(), pageSize, startKey)

		},
		update: func(ctx context.Context, item *accesslist.AccessListMember) error {
			_, err := p.accessLists.UpsertAccessListMember(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessListMembers,
	})

	// Verify counting.
	ctx := context.Background()
	for i := range numMembers {
		_, err = p.accessLists.UpsertAccessListMember(ctx, newAccessListMember(t, al.GetName(), strconv.Itoa(i)))
		require.NoError(t, err)
	}

	count, listCount, err := p.accessLists.CountAccessListMembers(ctx, al.GetName())
	require.NoError(t, err)
	require.Equal(t, uint32(numMembers), count)
	require.Equal(t, uint32(0), listCount)

	// Eventually, this should be reflected in the cache.
	timeout := time.After(5 * time.Second)
	for {
		count, listCount, err := p.cache.CountAccessListMembers(ctx, al.GetName())
		require.NoError(t, err)
		if count == numMembers && listCount == 0 {
			break
		}
		select {
		case <-timeout:
			require.Fail(t, "timed out waiting for correct member counts")
		case <-p.eventsC:
		}
	}
}

// TestAccessListReviews tests that CRUD operations on access list review resources are
// replicated from the backend to the cache.
func TestAccessListReviews(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClock()

	al, _, err := p.accessLists.UpsertAccessListWithMembers(context.Background(), newAccessList(t, "access-list", clock),
		[]*accesslist.AccessListMember{
			newAccessListMember(t, "access-list", "member1"),
			newAccessListMember(t, "access-list", "member2"),
			newAccessListMember(t, "access-list", "member3"),
			newAccessListMember(t, "access-list", "member4"),
			newAccessListMember(t, "access-list", "member5"),
		})
	require.NoError(t, err)

	// Keep track of the reviews, as create can update them. We'll use this
	// to make sure the values are up to date during the test.
	reviews := map[string]*accesslist.Review{}

	testResources(t, p, testFuncs[*accesslist.Review]{
		newResource: func(name string) (*accesslist.Review, error) {
			review := newAccessListReview(t, al.GetName(), name)
			// Store the name in the description.
			review.Metadata.Description = name
			reviews[name] = review
			return review, nil
		},
		create: func(ctx context.Context, item *accesslist.Review) error {
			review, _, err := p.accessLists.CreateAccessListReview(ctx, item)
			if err != nil {
				return trace.Wrap(err)
			}
			// Use the old name from the description.
			oldName := review.Metadata.Description
			reviews[oldName].SetName(review.GetName())
			return trace.Wrap(err)
		},
		list: p.accessLists.ListAllAccessListReviews,
		cacheList: func(ctx context.Context, pageSize int, startKey string) ([]*accesslist.Review, string, error) {
			return p.cache.ListAccessListReviews(ctx, al.GetName(), pageSize, startKey)
		},
		deleteAll: p.accessLists.DeleteAllAccessListReviews,
	}, withSkipPaginationTest()) // access list reviews resources have customer pagination test.

	_, _, err = p.accessLists.UpsertAccessListWithMembers(t.Context(), newAccessList(t, "fake-al-1", clock),
		[]*accesslist.AccessListMember{
			newAccessListMember(t, "fake-al-1", "member1"),
			newAccessListMember(t, "fake-al-1", "member2"),
		})
	require.NoError(t, err)

	_, _, err = p.accessLists.UpsertAccessListWithMembers(t.Context(), newAccessList(t, "fake-al-2", clock),
		[]*accesslist.AccessListMember{
			newAccessListMember(t, "fake-al-2", "member1"),
			newAccessListMember(t, "fake-al-2", "member2"),
		})
	require.NoError(t, err)

	review1 := newAccessListReview(t, "fake-al-1", "initial-review-1")
	review1, _, err = p.accessLists.CreateAccessListReview(t.Context(), review1)
	require.NoError(t, err)
	review2 := newAccessListReview(t, "fake-al-2", "initial-review-2")
	review2, _, err = p.accessLists.CreateAccessListReview(t.Context(), review2)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		out, next, err := p.cache.ListAccessListReviews(context.Background(), "fake-al-1", 100, "")
		require.NoError(t, err)
		require.Empty(t, next)

		require.Len(t, out, 1)
		require.Empty(t, cmp.Diff([]*accesslist.Review{review1}, out,
			cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
			protocmp.Transform()),
		)
	}, 15*time.Second, 100*time.Millisecond)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		out, next, err := p.cache.ListAccessListReviews(context.Background(), "fake-al-2", 100, "")
		require.NoError(t, err)
		require.Empty(t, next)

		require.Len(t, out, 1)
		require.Empty(t, cmp.Diff([]*accesslist.Review{review2}, out,
			cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
			protocmp.Transform()),
		)
	}, 15*time.Second, 100*time.Millisecond)

	_, _, err = p.accessLists.UpsertAccessListWithMembers(t.Context(), newAccessList(t, "access-list-test", clock),
		[]*accesslist.AccessListMember{
			newAccessListMember(t, "access-list-test", "member1"),
			newAccessListMember(t, "access-list-test", "member2"),
		})
	require.NoError(t, err)

	for i := range 10 {
		review := newAccessListReview(t, "access-list-test", "fake-review-"+strconv.Itoa(i))
		review.Spec.Changes = accesslist.ReviewChanges{}
		_, _, err = p.accessLists.CreateAccessListReview(t.Context(), review)
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		count := p.cache.collections.accessListReviews.store.len()
		_ = count

		var start string
		var out []*accesslist.Review
		for range 10 {
			page, next, err := p.cache.ListAccessListReviews(context.Background(), "access-list-test", 3, start)
			require.NoError(t, err)

			out = append(out, page...)
			if next == "" {
				break
			}
			start = next
		}
		require.Len(t, out, 10)
	}, 15*time.Second, 100*time.Millisecond)

}

func TestListAccessListsV2(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	ctx := context.Background()
	baseTime := time.Date(1984, 4, 4, 0, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(baseTime)

	names := []string{"apple-list", "banana-access", "cherry-management", "apple-admin", "zebra-test"}

	for i, name := range names {
		al := newAccessList(t, name, clock)
		auditDate := clock.Now().Add(time.Duration(i) * (time.Hour) * 24)
		// add arbitrary date so we can make sure its not just sorted by name
		if name == "banana-access" {
			auditDate = clock.Now().Add(100 * (time.Hour) * 24)
			al.Spec.Title = "bananatitle"
		}
		al.Spec.Audit.NextAuditDate = auditDate

		_, err := p.accessLists.UpsertAccessList(ctx, al)
		require.NoError(t, err)
	}

	testCases := []struct {
		name            string
		search          string
		sortBy          *types.SortBy
		expectedNames   []string
		pageSize        int
		expectedNextKey string
		startKey        string
	}{
		{
			name:          "no filter - all lists",
			expectedNames: []string{"apple-admin", "apple-list", "banana-access", "cherry-management", "zebra-test"},
		},
		{
			name:          "sort by name reverse",
			sortBy:        &types.SortBy{Field: "name", IsDesc: true},
			expectedNames: []string{"zebra-test", "cherry-management", "banana-access", "apple-list", "apple-admin"},
		},
		{
			name:          "sort by audit date",
			sortBy:        &types.SortBy{Field: "auditNextDate", IsDesc: false},
			expectedNames: []string{"apple-list", "cherry-management", "apple-admin", "zebra-test", "banana-access"},
		},
		{
			name:          "sort by title",
			sortBy:        &types.SortBy{Field: "title", IsDesc: false},
			expectedNames: []string{"banana-access", "apple-admin", "apple-list", "cherry-management", "zebra-test"},
		},
		{
			name:          "sort by title reverse",
			sortBy:        &types.SortBy{Field: "title", IsDesc: true},
			expectedNames: []string{"zebra-test", "cherry-management", "apple-list", "apple-admin", "banana-access"},
		},
		{
			name:          "sort by audit date reverse",
			sortBy:        &types.SortBy{Field: "auditNextDate", IsDesc: true},
			expectedNames: []string{"banana-access", "zebra-test", "apple-admin", "cherry-management", "apple-list"},
		},
		{
			name:            "paginated results",
			expectedNames:   []string{"apple-admin", "apple-list"},
			pageSize:        2,
			expectedNextKey: "banana-access",
		},
		{
			name:            "paginated results reverse",
			expectedNames:   []string{"zebra-test", "cherry-management", "banana-access"},
			sortBy:          &types.SortBy{Field: "name", IsDesc: true},
			pageSize:        3,
			expectedNextKey: "apple-list",
		},
		{
			name:          "with search",
			search:        "apple",
			expectedNames: []string{"apple-admin", "apple-list"},
		},
		{
			name:          "with startKey",
			startKey:      "banana-access",
			expectedNames: []string{"banana-access", "cherry-management", "zebra-test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				results, nextToken, err := p.cache.ListAccessListsV2(ctx, &accesslistv1.ListAccessListsV2Request{
					PageSize:  int32(tc.pageSize),
					PageToken: tc.startKey,
					Filter: &accesslistv1.AccessListsFilter{
						Search: tc.search,
					},
					SortBy: tc.sortBy,
				})
				require.NoError(t, err)
				require.Equal(t, tc.expectedNextKey, nextToken)

				require.Len(t, results, len(tc.expectedNames))
				actualNames := make([]string, len(results))
				for i, al := range results {
					actualNames[i] = al.GetName()
				}

				require.Equal(t, tc.expectedNames, actualNames)
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

func TestCountAccessListMembersScoping(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth, memoryBackend(true))
	t.Cleanup(p.Close)

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	listA, err := p.accessLists.UpsertAccessList(ctx, newAccessList(t, "list-a", clock))
	require.NoError(t, err)
	listB, err := p.accessLists.UpsertAccessList(ctx, newAccessList(t, "list-b", clock))
	require.NoError(t, err)

	const (
		listAUsers = 4
		listBUsers = 3
		listALists = 2
		listBLists = 1
	)

	for i := range listAUsers {
		member := newAccessListMember(t, listA.GetName(), fmt.Sprintf("user-%d", i))
		member.Spec.MembershipKind = accesslist.MembershipKindUser
		_, err := p.accessLists.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)
	}
	for i := range listBUsers {
		member := newAccessListMember(t, listB.GetName(), fmt.Sprintf("b-user-%d", i))
		member.Spec.MembershipKind = accesslist.MembershipKindUser
		_, err := p.accessLists.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)
	}

	for i := range listALists {
		nestedList, err := p.accessLists.UpsertAccessList(ctx, newAccessList(t, fmt.Sprintf("nested-list-a-%d", i), clock))
		require.NoError(t, err)
		member := newAccessListMember(t, listA.GetName(), nestedList.GetName())
		member.Spec.MembershipKind = accesslist.MembershipKindList
		_, err = p.accessLists.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)
	}
	for i := range listBLists {
		nestedList, err := p.accessLists.UpsertAccessList(ctx, newAccessList(t, fmt.Sprintf("nested-list-b-%d", i), clock))
		require.NoError(t, err)
		member := newAccessListMember(t, listB.GetName(), nestedList.GetName())
		member.Spec.MembershipKind = accesslist.MembershipKindList
		_, err = p.accessLists.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)
	}

	// wait for cache to reflect updates
	timeout := time.After(5 * time.Second)
	for {
		aUsers, aLists, err := p.cache.CountAccessListMembers(ctx, listA.GetName())
		require.NoError(t, err)
		bUsers, bLists, err := p.cache.CountAccessListMembers(ctx, listB.GetName())
		require.NoError(t, err)
		if aUsers == listAUsers && aLists == listALists && bUsers == listBUsers && bLists == listBLists {
			break
		}
		select {
		case <-timeout:
			require.Fail(t, "timed out waiting for correct member counts")
		case <-p.eventsC:
		}
	}
}

func TestGetAllAccessListMembers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClock()

	_, members, err := p.accessLists.UpsertAccessListWithMembers(context.Background(), newAccessList(t, "access-list", clock),
		makeMembers(t, "access-list", 10),
	)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		result, err := stream.Collect(clientutils.ResourcesWithPageSize(ctx, p.cache.ListAllAccessListMembers, 1))
		require.NoError(t, err)
		require.Len(t, result, len(members))
	}, time.Second*10, time.Millisecond*30)
}

func makeMembers(t *testing.T, alName string, count int) []*accesslist.AccessListMember {
	members := make([]*accesslist.AccessListMember, 0, count)
	for i := 0; i < count; i++ {
		members = append(members, newAccessListMember(t, alName, fmt.Sprintf("member-%d", i)))
	}
	return members
}
