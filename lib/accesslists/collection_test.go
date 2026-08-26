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
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestCollection_RefUpdates(t *testing.T) {
	clock := clockwork.NewFakeClock()

	t.Run("handles complex nested relationships", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)
		list3 := newAccessList(t, "list3", clock)

		list1.Spec.Owners = append(list1.Spec.Owners, accesslist.Owner{
			Name:           list2.GetName(),
			Description:    "list2 owner",
			MembershipKind: accesslist.MembershipKindList,
		})

		list2Member := newAccessListMember(t, list2.GetName(), list3.GetName(), accesslist.MembershipKindList, clock)

		list3Member := newAccessListMember(t, list3.GetName(), list1.GetName(), accesslist.MembershipKindList, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)
		collection.AddAccessList(list2, []*accesslist.AccessListMember{list2Member})
		collection.AddAccessList(list3, []*accesslist.AccessListMember{list3Member})

		err := collection.RefUpdates()
		require.NoError(t, err)

		// Verify relationships
		require.Contains(t, list2.Status.OwnerOf, list1.GetName())
		require.Contains(t, list3.Status.MemberOf, list2.GetName())
		require.Contains(t, list1.Status.MemberOf, list3.GetName())
	})

	t.Run("ignores non-list membership kinds for owners", func(t *testing.T) {
		// Create access lists
		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)

		list1.Spec.Owners = append(list1.Spec.Owners, accesslist.Owner{
			Name:           list2.GetName(),
			Description:    "user owner",
			MembershipKind: accesslist.MembershipKindUser,
		})

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)
		collection.AddAccessList(list2, nil)

		err := collection.RefUpdates()
		require.NoError(t, err)

		require.NotContains(t, list2.Status.OwnerOf, list1.GetName())
	})

	t.Run("ignores non-list membership kinds for members", func(t *testing.T) {
		// Create access lists
		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)

		// Add list2 as a User member of list1 (should be ignored)
		member := newAccessListMember(t, list1.GetName(), list2.GetName(), accesslist.MembershipKindUser, clock)

		// Create collection
		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member})
		collection.AddAccessList(list2, nil)

		err := collection.RefUpdates()
		require.NoError(t, err)

		require.NotContains(t, list2.Status.MemberOf, list1.GetName())
	})

	t.Run("avoids duplicate entries in OwnerOf", func(t *testing.T) {
		ownerList := newAccessList(t, "owner-list", clock)
		ownedList := newAccessList(t, "owned-list", clock)

		// Set ownerList as an owner of ownedList
		ownedList.Spec.Owners = append(ownedList.Spec.Owners, accesslist.Owner{
			Name:           ownerList.GetName(),
			Description:    "owner access list",
			MembershipKind: accesslist.MembershipKindList,
		})

		ownerList.Status.OwnerOf = []string{ownedList.GetName()}

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(ownerList, nil)
		collection.AddAccessList(ownedList, nil)

		err := collection.RefUpdates()
		require.NoError(t, err)

		// Verify no duplicates
		require.Len(t, ownerList.Status.OwnerOf, 1)
		require.Equal(t, ownedList.GetName(), ownerList.Status.OwnerOf[0])
	})

	t.Run("handles empty collection", func(t *testing.T) {
		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}

		err := collection.RefUpdates()
		require.NoError(t, err)
	})

	t.Run("handles multiple owners of same list", func(t *testing.T) {
		ownerList1 := newAccessList(t, "owner-list-1", clock)
		ownerList2 := newAccessList(t, "owner-list-2", clock)
		ownedList := newAccessList(t, "owned-list", clock)

		// Both ownerList1 and ownerList2 are owners of ownedList
		ownedList.Spec.Owners = append(ownedList.Spec.Owners,
			accesslist.Owner{
				Name:           ownerList1.GetName(),
				Description:    "first owner",
				MembershipKind: accesslist.MembershipKindList,
			},
			accesslist.Owner{
				Name:           ownerList2.GetName(),
				Description:    "second owner",
				MembershipKind: accesslist.MembershipKindList,
			},
		)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(ownerList1, nil)
		collection.AddAccessList(ownerList2, nil)
		collection.AddAccessList(ownedList, nil)

		err := collection.RefUpdates()
		require.NoError(t, err)

		// Verify both owners have the owned list in their OwnerOf
		require.Contains(t, ownerList1.Status.OwnerOf, ownedList.GetName())
		require.Contains(t, ownerList2.Status.OwnerOf, ownedList.GetName())
	})
}

func TestCollection_Validate(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	t.Run("validates successfully with valid access lists and members", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)
		collection.AddAccessList(list2, nil)

		err := collection.Validate(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when access list is invalid", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		// Make the list invalid by removing owners
		list1.Spec.Owners = []accesslist.Owner{}

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)

		err := collection.Validate(ctx)
		require.Error(t, err)
		require.ErrorContains(t, err, "owners")
	})

	t.Run("fails when member is invalid", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		member := newAccessListMember(t, list1.GetName(), "user1", accesslist.MembershipKindUser, clock)
		// Make the member invalid by clearing required field
		member.Spec.AccessList = ""

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member})

		err := collection.Validate(ctx)
		require.Error(t, err)
	})

	t.Run("applies RefUpdates during validation", func(t *testing.T) {
		ownerList := newAccessList(t, "owner-list", clock)
		ownedList := newAccessList(t, "owned-list", clock)

		// Set ownerList as an owner of ownedList
		ownedList.Spec.Owners = append(ownedList.Spec.Owners, accesslist.Owner{
			Name:           ownerList.GetName(),
			Description:    "owner access list",
			MembershipKind: accesslist.MembershipKindList,
		})

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(ownerList, nil)
		collection.AddAccessList(ownedList, nil)

		err := collection.Validate(ctx)
		require.NoError(t, err)

		// Verify that RefUpdates was applied
		require.Contains(t, ownerList.Status.OwnerOf, ownedList.GetName())
	})

	t.Run("validates access list hierarchy with nested members", func(t *testing.T) {
		parentList := newAccessList(t, "parent", clock)
		childList := newAccessList(t, "child", clock)

		// Add childList as member of parentList
		member := newAccessListMember(t, parentList.GetName(), childList.GetName(), accesslist.MembershipKindList, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(parentList, []*accesslist.AccessListMember{member})
		collection.AddAccessList(childList, nil)

		err := collection.Validate(ctx)
		require.NoError(t, err)

		// Verify that MemberOf was updated
		require.Contains(t, childList.Status.MemberOf, parentList.GetName())
	})

	t.Run("validates empty collection", func(t *testing.T) {
		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}

		err := collection.Validate(ctx)
		require.NoError(t, err)
	})

	t.Run("validates multiple access lists with mixed relationships", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)
		list3 := newAccessList(t, "list3", clock)

		// list2 is owner of list1
		list1.Spec.Owners = append(list1.Spec.Owners, accesslist.Owner{
			Name:           list2.GetName(),
			Description:    "owner",
			MembershipKind: accesslist.MembershipKindList,
		})

		member := newAccessListMember(t, list2.GetName(), list3.GetName(), accesslist.MembershipKindList, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)
		collection.AddAccessList(list2, []*accesslist.AccessListMember{member})
		collection.AddAccessList(list3, nil)

		err := collection.Validate(ctx)
		require.NoError(t, err)

		require.Contains(t, list2.Status.OwnerOf, list1.GetName())
		require.Contains(t, list3.Status.MemberOf, list2.GetName())
	})

	t.Run("calls ValidateAccessListWithMembers for each list", func(t *testing.T) {
		// This test ensures that ValidateAccessListWithMembers is being called
		// by creating a scenario that would fail ValidateAccessListWithMembers
		// Create a circular reference that ValidateAccessListWithMembers should catch

		list1 := newAccessList(t, "list1", clock)
		list2 := newAccessList(t, "list2", clock)

		// list1 -> list2 (member)
		member1 := newAccessListMember(t, list1.GetName(), list2.GetName(), accesslist.MembershipKindList, clock)

		// list2 -> list1 (member) - creates circular reference
		member2 := newAccessListMember(t, list2.GetName(), list1.GetName(), accesslist.MembershipKindList, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member1})
		collection.AddAccessList(list2, []*accesslist.AccessListMember{member2})

		err := collection.Validate(ctx)
		// circular reference should cause validation to fail
		require.Error(t, err)
	})
}

func TestCollection_ListAccessListMembers(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	t.Run("returns all members when pageSize is 0", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		member1 := newAccessListMember(t, list1.GetName(), "user1", accesslist.MembershipKindUser, clock)
		member2 := newAccessListMember(t, list1.GetName(), "user2", accesslist.MembershipKindUser, clock)
		member3 := newAccessListMember(t, list1.GetName(), "user3", accesslist.MembershipKindUser, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member1, member2, member3})

		members, nextToken, err := collection.ListAccessListMembers(ctx, list1.GetName(), 0, "")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Len(t, members, 3)
	})

	t.Run("paginates correctly with pageSize", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		member1 := newAccessListMember(t, list1.GetName(), "user1", accesslist.MembershipKindUser, clock)
		member2 := newAccessListMember(t, list1.GetName(), "user2", accesslist.MembershipKindUser, clock)
		member3 := newAccessListMember(t, list1.GetName(), "user3", accesslist.MembershipKindUser, clock)
		member4 := newAccessListMember(t, list1.GetName(), "user4", accesslist.MembershipKindUser, clock)
		member5 := newAccessListMember(t, list1.GetName(), "user5", accesslist.MembershipKindUser, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member1, member2, member3, member4, member5})

		// First page
		members, nextToken, err := collection.ListAccessListMembers(ctx, list1.GetName(), 2, "")
		require.NoError(t, err)
		require.NotEmpty(t, nextToken)
		require.Len(t, members, 2)
		require.Equal(t, "user1", members[0].GetName())
		require.Equal(t, "user2", members[1].GetName())

		// Second page
		members, nextToken, err = collection.ListAccessListMembers(ctx, list1.GetName(), 2, nextToken)
		require.NoError(t, err)
		require.NotEmpty(t, nextToken)
		require.Len(t, members, 2)
		require.Equal(t, "user3", members[0].GetName())
		require.Equal(t, "user4", members[1].GetName())

		// Third page (last)
		members, nextToken, err = collection.ListAccessListMembers(ctx, list1.GetName(), 2, nextToken)
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Len(t, members, 1)
		require.Equal(t, "user5", members[0].GetName())
	})

	t.Run("returns empty when pageToken is beyond members", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)
		member1 := newAccessListMember(t, list1.GetName(), "user1", accesslist.MembershipKindUser, clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, []*accesslist.AccessListMember{member1})

		members, nextToken, err := collection.ListAccessListMembers(ctx, list1.GetName(), 10, "100")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Empty(t, members)
	})

	t.Run("returns error for invalid pageToken", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)

		_, _, err := collection.ListAccessListMembers(ctx, list1.GetName(), 10, "invalid")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid page token")
	})

	t.Run("returns not found for non-existent access list", func(t *testing.T) {
		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}

		_, _, err := collection.ListAccessListMembers(ctx, "non-existent", 10, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("handles empty member list", func(t *testing.T) {
		list1 := newAccessList(t, "list1", clock)

		collection := &Collection{
			MembersByAccessList: make(map[string][]*accesslist.AccessListMember),
			AccessListsByName:   make(map[string]*accesslist.AccessList),
		}
		collection.AddAccessList(list1, nil)

		members, nextToken, err := collection.ListAccessListMembers(ctx, list1.GetName(), 10, "")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Empty(t, members)
	})
}
