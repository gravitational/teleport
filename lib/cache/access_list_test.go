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

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
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
		list: func(ctx context.Context) ([]*accesslist.AccessList, error) {
			items, _, err := p.accessLists.ListAccessLists(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheGet: p.cache.GetAccessList,
		cacheList: func(ctx context.Context) ([]*accesslist.AccessList, error) {
			items, _, err := p.cache.ListAccessLists(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
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
		list: func(ctx context.Context) ([]*accesslist.AccessListMember, error) {
			items, _, err := p.accessLists.ListAllAccessListMembers(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheGet: func(ctx context.Context, name string) (*accesslist.AccessListMember, error) {
			return p.cache.GetAccessListMember(ctx, al.GetName(), name)
		},
		cacheList: func(ctx context.Context) ([]*accesslist.AccessListMember, error) {
			items, _, err := p.cache.ListAccessListMembers(ctx, al.GetName(), 0 /* page size */, "")
			return items, trace.Wrap(err)
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
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {
					Enabled: true,
					Limit:   10,
				},
			},
		},
	})

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
			// Use the old name from the description.
			oldName := review.Metadata.Description
			reviews[oldName].SetName(review.GetName())
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*accesslist.Review, error) {
			items, _, err := p.accessLists.ListAllAccessListReviews(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*accesslist.Review, error) {
			items, _, err := p.cache.ListAccessListReviews(ctx, al.GetName(), 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessListReviews,
	})

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
		assert.Empty(t, next)

		assert.Len(t, out, 1)
		assert.Empty(t, cmp.Diff([]*accesslist.Review{review1}, out,
			cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
			protocmp.Transform()),
		)
	}, 15*time.Second, 100*time.Millisecond)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		out, next, err := p.cache.ListAccessListReviews(context.Background(), "fake-al-2", 100, "")
		require.NoError(t, err)
		assert.Empty(t, next)

		assert.Len(t, out, 1)
		assert.Empty(t, cmp.Diff([]*accesslist.Review{review2}, out,
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
		assert.Len(t, out, 10)
	}, 15*time.Second, 100*time.Millisecond)

}

func TestCountAccessListMembersScoping(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

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
