/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package local

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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
)

// TestAccessListCRUD tests backend operations with access list resources.
func TestAccessListCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	// Initially we expect no access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	// Create both access lists.
	accessList, err := service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// Fetch all access lists.
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList1, accessList2}, out, cmpOpts...))

	// Fetch a paginated list of access lists
	paginatedOut := make([]*accesslist.AccessList, 0, 2)
	var nextToken string
	for {
		out, nextToken, err = service.ListAccessLists(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Len(t, paginatedOut, 2)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList1, accessList2}, paginatedOut, cmpOpts...))

	// Fetch a specific access list.
	accessList, err = service.GetAccessList(ctx, accessList2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// Try to fetch an access list that doesn't exist.
	_, err = service.GetAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Update an access list.
	accessList1.SetExpiry(clock.Now().Add(30 * time.Minute))
	accessList, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))

	// Delete an access list.
	err = service.DeleteAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{accessList2}, out, cmpOpts...))

	// Try to delete an access list that doesn't exist.
	err = service.DeleteAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all access lists.
	err = service.DeleteAllAccessLists(ctx)
	require.NoError(t, err)
	out, err = service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Try to create an access list with duplicate owners.
	accessListDuplicateOwners := newAccessList(t, "accessListDuplicateOwners", clock)
	accessListDuplicateOwners.Spec.Owners = append(accessListDuplicateOwners.Spec.Owners, accessListDuplicateOwners.Spec.Owners[0])

	_, err = service.UpsertAccessList(ctx, accessListDuplicateOwners)
	require.True(t, trace.IsAlreadyExists(err))
}

// TestAccessListCreate_UpsertAccessList_WithoutLimit tests creating access list
// is unlimited if IGS feature is enabled.
func TestAccessListCreate_UpsertAccessList_WithoutLimit(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)
	accessList3 := newAccessList(t, "accessList3", clock)

	// No limit to creating access list.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	_, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)
	_, err = service.UpsertAccessList(ctx, accessList3)
	require.NoError(t, err)

	// Fetch all access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 3)
}

// TestAccessListCreate_UpdateAccessList tests creating access list
// and updating access list with the same name.
func TestAccessListCreate_UpdateAccessList(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// No limit to creating access list.
	result, err := service.UpsertAccessList(ctx, newAccessList(t, "accessList1", clock))
	require.NoError(t, err)
	// Fetch all access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 1)

	result.Spec.Description = "changing description"
	// Update access list with the correct revision.
	_, err = service.UpdateAccessList(ctx, result)
	require.NoError(t, err)
	result.Spec.Description = "changing description again"
	result.Metadata.Revision = "fake revision"
	// Update access list with wrong revision should return an error.
	_, err = service.UpdateAccessList(ctx, result)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected precondition failed error, got %v", err)
}

// TestAccessListCreate_UpsertAccessList_WithLimit tests creating access list
// is limited to the limit defined in feature if IGS is NOT enabled.
// Also tests "upserting" and deleting is allowed despite "create" limit reached.
func TestAccessListCreate_UpsertAccessList_WithLimit(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, false /* igsEnabled */)

	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	// First create is free.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	// Second create should return an error.
	_, err = service.UpsertAccessList(ctx, accessList2)
	require.True(t, trace.IsAccessDenied(err), "expected access denied / license limit error, got %v", err)
	require.ErrorContains(t, err, "reached its limit")

	// Double check only be one access list exists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 1)

	// Updating existing access list should be allowed.
	accessList1.Spec.Description = "changing description"
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	// Delete the one access list.
	err = service.DeleteAccessList(ctx, "accessList1")
	require.NoError(t, err)

	// Create the same list again.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
}

// TestAccessListCreate_UpsertAccessListWithMembers_WithLimit tests creating access list
// with members, is limited to the limit defined in feature if IGS is NOT enabled.
// Also tests "upserting" and deleting is allowed despite "create" limit reached.
func TestAccessListCreate_UpsertAccessListWithMembers_WithLimit(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, false /* igsEnabled */)

	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	accessListMember1 := newAccessListMember(t, accessList1.GetName(), "alice")
	accessListMember2 := newAccessListMember(t, accessList1.GetName(), "bob")

	// First create is free.
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessListMember1})
	require.NoError(t, err)

	// Check the count
	count, err := service.CountAccessListMembers(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Equal(t, uint32(1), count)

	// Second create should return an error.
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList2, []*accesslist.AccessListMember{accessListMember2})
	require.True(t, trace.IsAccessDenied(err), "expected access denied / license limit error, got %v", err)
	require.ErrorContains(t, err, "reached its limit")

	// Double check only be one access list exists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "accessList1", out[0].Metadata.Name)

	// Double check only one member exists.
	members, _, err := service.ListAccessListMembers(ctx, accessList1.GetName(), 0 /* default size*/, "")
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "alice", members[0].Metadata.Name)

	// Updating existing access list should be allowed.
	accessList1.Spec.Description = "changing description"
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessListMember1})
	require.NoError(t, err)

	// Delete the one access list.
	err = service.DeleteAccessList(ctx, "accessList1")
	require.NoError(t, err)

	// Create the same list again.
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessListMember1})
	require.NoError(t, err)
}

// TestAccessListCreate_UpsertAccessListWithMembers_WithoutLimit tests creating access list
// with members is unlimited if IGS feature is enabled.
func TestAccessListCreate_UpsertAccessListWithMembers_WithoutLimit(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)
	accessList3 := newAccessList(t, "accessList3", clock)

	accessListMember1 := newAccessListMember(t, accessList1.GetName(), "alice")
	accessListMember2 := newAccessListMember(t, accessList1.GetName(), "bob")

	// No limit to creating access list.
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessListMember1})
	require.NoError(t, err)
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList2, []*accesslist.AccessListMember{accessListMember2})
	require.NoError(t, err)
	_, _, err = service.UpsertAccessListWithMembers(ctx, accessList3, []*accesslist.AccessListMember{})
	require.NoError(t, err)

	// Fetch all access lists.
	out, err := service.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, out, 3)
}

func TestAccessListDedupeOwnersBackwardsCompat(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Put an unduplicated owners access list in the backend.
	accessListDuplicateOwners := newAccessList(t, "accessListDuplicateOwners", clock)
	accessListDuplicateOwners.Spec.Owners = append(accessListDuplicateOwners.Spec.Owners, accessListDuplicateOwners.Spec.Owners[0])
	require.Len(t, accessListDuplicateOwners.Spec.Owners, 3)

	item, err := service.service.MakeBackendItem(accessListDuplicateOwners, accessListDuplicateOwners.GetName())
	require.NoError(t, err)
	_, err = mem.Put(ctx, item)
	require.NoError(t, err)

	accessList, err := service.GetAccessList(ctx, accessListDuplicateOwners.GetName())
	require.NoError(t, err)

	require.Len(t, accessList.Spec.Owners, 2)
}

func TestAccessListUpsertWithMembers(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID"),
	}

	t.Run("create access list", func(t *testing.T) {
		// Create both access lists.
		accessList, _, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	})

	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "alice")

	t.Run("add member to the access list", func(t *testing.T) {
		// Add access list members.
		updatedAccessList, updatedMembers, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessList1Member1})
		require.NoError(t, err)
		// Assert that access list is returned.
		require.Empty(t, cmp.Diff(updatedAccessList, updatedAccessList, cmpOpts...))
		// Assert that the member is returned.
		require.Len(t, updatedMembers, 1)
		require.Empty(t, cmp.Diff(updatedMembers[0], accessList1Member1, cmpOpts...))

		listMembers, err := service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member1, cmpOpts...))
	})

	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "bob")

	t.Run("add another member to the access list", func(t *testing.T) {
		// Add access list members.
		updatedAccessList, updatedMembers, err := service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{accessList1Member1, accessList1Member2})
		require.NoError(t, err)
		// Assert that access list is returned.
		require.Empty(t, cmp.Diff(updatedAccessList, updatedAccessList, cmpOpts...))
		// Assert that the member is returned.
		require.Len(t, updatedMembers, 2)
		require.Empty(t, cmp.Diff(updatedMembers, []*accesslist.AccessListMember{accessList1Member1, accessList1Member2}, cmpOpts...))

		listMembers, err := service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member1, cmpOpts...))

		listMembers, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(listMembers, accessList1Member2, cmpOpts...))
	})

	t.Run("empty members removes all members", func(t *testing.T) {
		_, _, err = service.UpsertAccessListWithMembers(ctx, accessList1, []*accesslist.AccessListMember{})
		require.NoError(t, err)

		members, _, err := service.ListAccessListMembers(ctx, accessList1.GetName(), 0 /* default size*/, "")
		require.NoError(t, err)
		require.Empty(t, members)
	})

}

func TestAccessListMembersCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	// Create both access lists.
	accessList, err := service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1, accessList, cmpOpts...))
	accessList, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2, accessList, cmpOpts...))

	// There should be no access list members for either list.
	members, _, err := service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	// Listing members of a non existent list should produce an error.
	_, _, err = service.ListAccessListMembers(ctx, "non-existent", 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent\" doesn't exist"))

	// Verify access list members are not present.
	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "alice")
	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "bob")

	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.True(t, trace.IsNotFound(err))
	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.True(t, trace.IsNotFound(err))

	// Add access list members.
	member, err := service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member1, member, cmpOpts...))
	member, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member2, member, cmpOpts...))

	member, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member1, member, cmpOpts...))
	member, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList1Member2, member, cmpOpts...))

	// Add access list member for non existent list should produce an error.
	_, err = service.UpsertAccessListMember(ctx, newAccessListMember(t, "non-existent-list", "nobody"))
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	accessList2Member1 := newAccessListMember(t, accessList2.GetName(), "bob")
	accessList2Member2 := newAccessListMember(t, accessList2.GetName(), "jim")
	member, err = service.UpsertAccessListMember(ctx, accessList2Member1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2Member1, member, cmpOpts...))
	member, err = service.UpsertAccessListMember(ctx, accessList2Member2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(accessList2Member2, member, cmpOpts...))

	// Fetch a paginated list of access lists members
	var paginatedMembers []*accesslist.AccessListMember
	var nextToken string
	const pageSize = 1
	for {
		members, nextToken, err = service.ListAccessListMembers(ctx, accessList1.GetName(), pageSize, nextToken)
		require.NoError(t, err)

		paginatedMembers = append(paginatedMembers, members...)
		if nextToken == "" {
			break
		}
	}
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{accessList1Member1, accessList1Member2}, paginatedMembers, cmpOpts...))

	// Delete a member from an access list.
	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.NoError(t, err)

	require.NoError(t, service.DeleteAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName()))

	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.True(t, trace.IsNotFound(err))

	// Delete from a non-existent access list should return an error.
	err = service.DeleteAccessListMember(ctx, "non-existent-list", "nobody")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	// Delete an access list.
	err = service.DeleteAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)

	// Verify that the access list's members have been removed and that the other has not been affected.
	_, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.True(t, trace.IsNotFound(err), "missing access list should produce not found error during list")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList1.GetName()))

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, members)

	// Re-add access list 1 with its members.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	// Verify that the members were previously removed.
	members, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	_, err = service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	_, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)

	// try to update a member with the wrong revision.
	accessList1Member2.Metadata.Revision = "fake revision"
	_, err = service.UpdateAccessListMember(ctx, accessList1Member2)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected precondition failed error, got %v", err)

	// Delete all members from access list 1.
	require.NoError(t, service.DeleteAllAccessListMembersForAccessList(ctx, accessList1.GetName()))

	members, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, members)

	// Try to delete all members from a non-existent list.
	err = service.DeleteAllAccessListMembersForAccessList(ctx, "non-existent-list")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	members, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, members)

	// Try to delete an access list that doesn't exist.
	err = service.DeleteAccessList(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all access lists.
	err = service.DeleteAllAccessLists(ctx)
	require.NoError(t, err)

	// Verify that access lists are gone.
	_, _, err = service.ListAccessListMembers(ctx, accessList1.GetName(), 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList1.GetName()))

	_, _, err = service.ListAccessListMembers(ctx, accessList2.GetName(), 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list %q doesn't exist", accessList2.GetName()))
}

func TestAccessListReviewCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	// Create a couple access lists.
	accessList1 := newAccessList(t, "accessList1", clock)
	accessList2 := newAccessList(t, "accessList2", clock)

	accessList1OrigDate := accessList1.Spec.Audit.NextAuditDate
	accessList2OrigDate := accessList2.Spec.Audit.NextAuditDate

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		cmpopts.SortSlices(func(review1, review2 *accesslist.Review) bool {
			return review1.GetName() < review2.GetName()
		}),
	}

	// Create both access lists.
	_, err = service.UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)
	_, err = service.UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)

	accessList1Member1 := newAccessListMember(t, accessList1.GetName(), "member1")
	_, err = service.UpsertAccessListMember(ctx, accessList1Member1)
	require.NoError(t, err)
	accessList1Member2 := newAccessListMember(t, accessList1.GetName(), "member2")
	_, err = service.UpsertAccessListMember(ctx, accessList1Member2)
	require.NoError(t, err)
	accessList2Member1 := newAccessListMember(t, accessList2.GetName(), "member1")
	_, err = service.UpsertAccessListMember(ctx, accessList2Member1)
	require.NoError(t, err)
	accessList2Member2 := newAccessListMember(t, accessList2.GetName(), "member2")
	_, err = service.UpsertAccessListMember(ctx, accessList2Member2)
	require.NoError(t, err)

	// There should be no access list reviews for either list.
	reviews, _, err := service.ListAccessListReviews(ctx, accessList1.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 0, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	// Listing reviews of a non existent list should produce an error.
	_, _, err = service.ListAccessListReviews(ctx, "non-existent", 0, "")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent\" doesn't exist"))

	accessList1Review1 := newAccessListReview(t, accessList1.GetName(), "al1-review1")
	accessList1Review2 := newAccessListReview(t, accessList1.GetName(), "al1-review2")
	accessList1Review2.Spec.Changes.RemovedMembers = nil
	accessList2Review1 := newAccessListReview(t, accessList2.GetName(), "al2-review1")
	accessList2Review1.Spec.Changes.MembershipRequirementsChanged = nil
	accessList2Review1.Spec.Changes.RemovedMembers = nil
	accessList2Review1.Spec.Changes.ReviewFrequencyChanged = 0
	accessList2Review1.Spec.Changes.ReviewDayOfMonthChanged = 0
	var nextReviewDate time.Time

	// Add access list review.
	accessList1Review1, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList1Review1)
	require.NoError(t, err)

	// Verify changes to access list.
	accessList1Updated, err := service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList1OrigDate.Year(),
			accessList1OrigDate.Month()+time.Month(accessList1Updated.Spec.Audit.Recurrence.Frequency),
			int(accessList1Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList1Updated.Spec.Audit.NextAuditDate,
	)
	require.Empty(t, cmp.Diff(*(accessList1Review1.Spec.Changes.MembershipRequirementsChanged), accessList1Updated.Spec.MembershipRequires))
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewFrequencyChanged, accessList1Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewDayOfMonthChanged, accessList1Updated.Spec.Audit.Recurrence.DayOfMonth)
	// The Correct value is returned through the API.
	require.Equal(t, accessList1Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member1.GetName())
	require.True(t, trace.IsNotFound(err))
	_, err = service.GetAccessListMember(ctx, accessList1.GetName(), accessList1Member2.GetName())
	require.True(t, trace.IsNotFound(err))

	// Add another review
	accessList1Review2, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList1Review2)
	require.NoError(t, err)

	// Verify changes to the access list again.
	accessList1Updated, err = service.GetAccessList(ctx, accessList1.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList1OrigDate.Year(),
			accessList1OrigDate.Month()+time.Month(accessList1Updated.Spec.Audit.Recurrence.Frequency)*2,
			int(accessList1Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList1Updated.Spec.Audit.NextAuditDate,
	)

	// Attempting to apply changes already reflected in the access list should modify the original review.
	require.Nil(t, accessList1Review2.Spec.Changes.MembershipRequirementsChanged)
	require.Equal(t, 0, int(accessList1Review2.Spec.Changes.ReviewFrequencyChanged))
	require.Equal(t, 0, int(accessList1Review2.Spec.Changes.ReviewDayOfMonthChanged))

	// No changes should have been made.
	require.Empty(t, cmp.Diff(*(accessList1Review1.Spec.Changes.MembershipRequirementsChanged), accessList1Updated.Spec.MembershipRequires))
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewFrequencyChanged, accessList1Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList1Review1.Spec.Changes.ReviewDayOfMonthChanged, accessList1Updated.Spec.Audit.Recurrence.DayOfMonth)
	require.Equal(t, accessList1Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	// Review that doesn't change anything
	accessList2Review1, nextReviewDate, err = service.CreateAccessListReview(ctx, accessList2Review1)
	require.NoError(t, err)

	accessList2Updated, err := service.GetAccessList(ctx, accessList2.GetName())
	require.NoError(t, err)
	require.Equal(t,
		time.Date(accessList2OrigDate.Year(),
			accessList2OrigDate.Month()+time.Month(accessList2Updated.Spec.Audit.Recurrence.Frequency),
			int(accessList2Updated.Spec.Audit.Recurrence.DayOfMonth), 0, 0, 0, 0, time.UTC),
		accessList2Updated.Spec.Audit.NextAuditDate,
	)
	require.Empty(t, cmp.Diff(accessList2.Spec.MembershipRequires, accessList2Updated.Spec.MembershipRequires))
	require.Equal(t, accessList2.Spec.Audit.Recurrence.Frequency, accessList2Updated.Spec.Audit.Recurrence.Frequency)
	require.Equal(t, accessList2.Spec.Audit.Recurrence.DayOfMonth, accessList2Updated.Spec.Audit.Recurrence.DayOfMonth)
	require.Equal(t, accessList2Updated.Spec.Audit.NextAuditDate, nextReviewDate)

	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member1.GetName())
	require.NoError(t, err)
	_, err = service.GetAccessListMember(ctx, accessList2.GetName(), accessList2Member2.GetName())
	require.NoError(t, err)

	// Fetch a paginated list of access lists reviews
	var paginatedReviews []*accesslist.Review
	var nextToken string
	const pageSize = 1
	for {
		reviews, nextToken, err = service.ListAccessListReviews(ctx, accessList1.GetName(), pageSize, nextToken)
		require.NoError(t, err)

		paginatedReviews = append(paginatedReviews, reviews...)
		if nextToken == "" {
			break
		}
	}
	require.Empty(t, cmp.Diff([]*accesslist.Review{accessList1Review1, accessList1Review2}, paginatedReviews, cmpOpts...))

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 1, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.Review{accessList2Review1}, reviews, cmpOpts...))

	// Delete a review from an access list.
	require.NoError(t, service.DeleteAccessListReview(ctx, accessList2.GetName(), accessList2Review1.GetName()))

	reviews, _, err = service.ListAccessListReviews(ctx, accessList2.GetName(), 1, "")
	require.NoError(t, err)
	require.Empty(t, reviews)

	// Delete from a non-existent access list should return an error.
	err = service.DeleteAccessListReview(ctx, "non-existent-list", "no-review")
	require.ErrorIs(t, err, trace.NotFound("access_list \"non-existent-list\" doesn't exist"))

	// Delete a non-existent access list review.
	err = service.DeleteAccessListReview(ctx, accessList2.GetName(), "no-review")
	require.ErrorIs(t, err, trace.NotFound("access_list_review \"no-review\" doesn't exist"))

	// Try to delete all reviews from a non-existent list.
	// Delete all access list reviews.
	err = service.DeleteAllAccessListReviews(ctx)
	require.NoError(t, err)

	// Verify that access lists reviews are gone.
	_, _, err = service.ListAccessListReviews(ctx, accessList1.GetName(), 0, "")
	require.Empty(t, err)
}

func TestAccessListRequiresEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        accesslist.Requires
		b        accesslist.Requires
		expected bool
	}{
		{
			name:     "empty",
			expected: true,
		},
		{
			name: "both equal",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: true,
		},
		{
			name: "roles length",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "c", "d"},
			},
			expected: false,
		},
		{
			name: "roles content",
			a: accesslist.Requires{
				Roles: []string{"a", "b", "c"},
			},
			b: accesslist.Requires{
				Roles: []string{"a", "b", "d"},
			},
			expected: false,
		},
		{
			name: "trait length",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
					"trait3": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait key different",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait3": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait values length",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2", "val3"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
		{
			name: "trait values different",
			a: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val2"},
					"trait2": []string{"val1", "val2"},
				},
			},
			b: accesslist.Requires{
				Traits: trait.Traits{
					"trait1": []string{"val1", "val3"},
					"trait2": []string{"val1", "val2"},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, accessListRequiresEqual(test.a, test.b))
		})
	}
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now(),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}

func newAccessListMember(t *testing.T, accessList, name string) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       name,
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour * 24),
			Reason:     "a reason",
			AddedBy:    "dummy",
		},
	)
	require.NoError(t, err)

	return member
}

func newAccessListReview(t *testing.T, accessList, name string) *accesslist.Review {
	t.Helper()

	review, err := accesslist.NewReview(
		header.Metadata{
			Name: name,
		},
		accesslist.ReviewSpec{
			AccessList: accessList,
			Reviewers: []string{
				"user1",
				"user2",
			},
			ReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Notes:      "Some notes",
			Changes: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{
						"role1",
						"role2",
					},
					Traits: trait.Traits{
						"trait1": []string{
							"value1",
							"value2",
						},
						"trait2": []string{
							"value1",
							"value2",
						},
					},
				},
				RemovedMembers: []string{
					"member1",
					"member2",
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
		},
	)
	require.NoError(t, err)

	return review
}

func TestAccessListService_ListAllAccessListMembers(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	const numAccessLists = 10
	const numAccessListMembersPerAccessList = 250
	totalMembers := numAccessLists * numAccessListMembersPerAccessList

	// Create several access lists.
	expectedMembers := make([]*accesslist.AccessListMember, totalMembers)
	for i := 0; i < numAccessLists; i++ {
		alName := strconv.Itoa(i)
		_, err := service.UpsertAccessList(ctx, newAccessList(t, alName, clock))
		require.NoError(t, err)

		for j := 0; j < numAccessListMembersPerAccessList; j++ {
			member := newAccessListMember(t, alName, fmt.Sprintf("%03d", j))
			expectedMembers[i*numAccessListMembersPerAccessList+j] = member
			_, err := service.UpsertAccessListMember(ctx, member)
			require.NoError(t, err)
		}
	}

	allMembers := make([]*accesslist.AccessListMember, 0, totalMembers)
	var nextToken string
	for {
		var members []*accesslist.AccessListMember
		var err error
		members, nextToken, err = service.ListAllAccessListMembers(ctx, 0, nextToken)
		require.NoError(t, err)

		allMembers = append(allMembers, members...)

		if nextToken == "" {
			break
		}
	}

	require.Empty(t, cmp.Diff(expectedMembers, allMembers, cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision")))
}

func TestAccessListService_ListAllAccessListReviews(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := newAccessListService(t, mem, clock, true /* igsEnabled */)

	const numAccessLists = 10
	const numAccessListReviewsPerAccessList = 250
	totalReviews := numAccessLists * numAccessListReviewsPerAccessList

	// Create several access lists.
	expectedReviews := make([]*accesslist.Review, totalReviews)
	for i := 0; i < numAccessLists; i++ {
		alName := strconv.Itoa(i)
		_, err := service.UpsertAccessList(ctx, newAccessList(t, alName, clock))
		require.NoError(t, err)

		for j := 0; j < numAccessListReviewsPerAccessList; j++ {
			review, err := accesslist.NewReview(
				header.Metadata{
					Name: strconv.Itoa(j),
				},
				accesslist.ReviewSpec{
					AccessList: alName,
					Reviewers: []string{
						"user1",
					},
					ReviewDate: time.Now(),
				},
			)
			require.NoError(t, err)
			review, _, err = service.CreateAccessListReview(ctx, review)
			expectedReviews[i*numAccessListReviewsPerAccessList+j] = review
			require.NoError(t, err)
		}
	}

	allReviews := make([]*accesslist.Review, 0, totalReviews)
	var nextToken string
	for {
		var reviews []*accesslist.Review
		var err error
		reviews, nextToken, err = service.ListAllAccessListReviews(ctx, 0, nextToken)
		require.NoError(t, err)

		allReviews = append(allReviews, reviews...)

		if nextToken == "" {
			break
		}
	}

	require.Empty(t, cmp.Diff(expectedReviews, allReviews, cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"), cmpopts.SortSlices(
		func(r1, r2 *accesslist.Review) bool {
			return r1.GetName() < r2.GetName()
		}),
	))
}

func newAccessListService(t *testing.T, mem *memory.Memory, clock clockwork.Clock, igsEnabled bool) *AccessListService {
	t.Helper()

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: igsEnabled,
			AccessList: modules.AccessListFeature{
				CreateLimit: 1,
			},
		},
	})

	service, err := NewAccessListService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	return service
}
