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
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
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

	p := newTestPack(t, ForAuth)
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
	for i := 0; i < 40; i++ {
		_, err = p.accessLists.UpsertAccessListMember(ctx, newAccessListMember(t, al.GetName(), strconv.Itoa(i)))
		require.NoError(t, err)
	}

	count, listCount, err := p.accessLists.CountAccessListMembers(ctx, al.GetName())
	require.NoError(t, err)
	require.Equal(t, uint32(40), count)
	require.Equal(t, uint32(0), listCount)

	// Eventually, this should be reflected in the cache.
	require.Eventually(t, func() bool {
		// Make sure the cache has a single resource in it.
		count, listCount, err := p.cache.CountAccessListMembers(ctx, al.GetName())
		assert.NoError(t, err)
		return count == uint32(40) && listCount == uint32(0)
	}, time.Second*2, time.Millisecond*250)
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
}
