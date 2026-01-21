/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"iter"
	"os"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// Mock implementation of AccessListAndMembersGetter.
type mockAccessListAndMembersGetter struct {
	accessLists map[string]*accesslist.AccessList
	members     map[string][]*accesslist.AccessListMember
}

func (m *mockAccessListAndMembersGetter) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	member, exists := m.members[accessListName]
	if !exists {
		return nil, trace.NotFound("access list %v member %v not found", accessListName, memberName)
	}
	for _, m := range member {
		if m.GetName() == memberName {
			return m, nil
		}
	}
	return nil, trace.NotFound("access list %v member %v not found", accessListName, memberName)
}

func (m *mockAccessListAndMembersGetter) GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error) {
	accessList, exists := m.accessLists[accessListName]
	if !exists {
		return nil, trace.NotFound("access list %v not found", accessListName)
	}
	return accessList, nil
}

func (m *mockAccessListAndMembersGetter) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	members, exists := m.members[accessListName]
	if !exists {
		return nil, "", nil
	}
	return members, "", nil
}

type mockLocksGetter struct {
	targets map[string][]types.Lock
}

func (m *mockLocksGetter) GetLock(ctx context.Context, name string) (types.Lock, error) {
	panic("not implemented")
}

func (m *mockLocksGetter) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {

	var locks []types.Lock
	for _, target := range targets {
		locks = append(locks, m.targets[target.User]...)
	}
	return locks, nil

}

func (m *mockLocksGetter) ListLocks(ctx context.Context, limit int, startKey string, filter *types.LockFilter) ([]types.Lock, string, error) {
	if limit > 0 || startKey != "" {
		return nil, "", trace.NotImplemented("limit and start are not supported")
	}

	if filter == nil {
		return nil, "", trace.BadParameter("missing filter")
	}

	var locks []types.Lock
	for _, target := range filter.Targets {
		locks = append(locks, m.targets[target.User]...)
	}
	return locks, "", nil
}

func (m *mockLocksGetter) RangeLocks(ctx context.Context, start, end string, filter *types.LockFilter) iter.Seq2[types.Lock, error] {
	if start != "" || end != "" {
		return stream.Fail[types.Lock](trace.NotImplemented("start and end are not supported"))
	}

	if filter == nil {
		return stream.Fail[types.Lock](trace.BadParameter("missing filter"))
	}

	var sliceStreams []stream.Stream[types.Lock]

	for _, target := range filter.Targets {
		sliceStreams = append(sliceStreams, stream.Slice(m.targets[target.User]))
	}

	return stream.Chain(sliceStreams...)
}

const (
	ownerUser  = "ownerUser"
	ownerUser2 = "ownerUser2"
	member1    = "member1"
	member2    = "member2"
)

func Test_userLockedError_IsUserLocked(t *testing.T) {
	userLockedErr := newUserLockedError("alice")
	rawAccessDeniedErr := trace.AccessDenied("Raw AccessDenied error")

	require.True(t, IsUserLocked(userLockedErr))
	require.False(t, IsUserLocked(rawAccessDeniedErr))
	require.True(t, trace.IsAccessDenied(userLockedErr))
}

func TestAccessListHierarchyIsOwner(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)
	acl4 := newAccessList(t, "4", clock)

	// acl1 -> acl2 -> acl3 as members
	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl2.Status.MemberOf = append(acl2.Status.MemberOf, acl1.GetName())
	acl1m2 := newAccessListMember(t, acl1.GetName(), member1, accesslist.MembershipKindUser, clock)
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)
	acl3.Status.MemberOf = append(acl3.Status.MemberOf, acl2.GetName())
	acl4m1 := newAccessListMember(t, acl4.GetName(), member2, accesslist.MembershipKindUser, clock)

	// acl4 -> acl1 as owner
	acl4.Spec.Owners = append(acl4.Spec.Owners, accesslist.Owner{
		Name:           acl1.GetName(),
		Description:    "asdf",
		MembershipKind: accesslist.MembershipKindList,
	})
	acl1.Status.OwnerOf = append(acl1.Status.OwnerOf, acl4.GetName())

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1, acl1m2},
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {},
			acl4.GetName(): {acl4m1},
		},
		accessLists: map[string]*accesslist.AccessList{
			acl1.GetName(): acl1,
			acl2.GetName(): acl2,
			acl3.GetName(): acl3,
			acl4.GetName(): acl4,
		},
	}

	// User which does not meet acl1's Membership requirements.
	stubUserNoRequires, err := types.NewUser(member1)
	require.NoError(t, err)

	ownershipType, err := IsAccessListOwner(ctx, stubUserNoRequires, acl4, accessListAndMembersGetter, nil, clock)
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	// Should not have inherited ownership due to missing OwnershipRequires.
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, ownershipType)

	// User which only meets acl1's Membership requirements.
	stubUserMeetsMemberRequires, err := types.NewUser(member1)
	require.NoError(t, err)
	stubUserMeetsMemberRequires.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	stubUserMeetsMemberRequires.SetRoles([]string{"mrole1", "mrole2"})

	ownershipType, err = IsAccessListOwner(ctx, stubUserMeetsMemberRequires, acl4, accessListAndMembersGetter, nil, clock)
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, ownershipType)

	// User which meets acl1's Membership and acl1's Ownership requirements.
	stubUserMeetsAllRequires, err := types.NewUser(member1)
	require.NoError(t, err)
	stubUserMeetsAllRequires.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
		"otrait1": {"ovalue1", "ovalue2"},
		"otrait2": {"ovalue3", "ovalue4"},
	})
	stubUserMeetsAllRequires.SetRoles([]string{"mrole1", "mrole2", "orole1", "orole2"})

	ownershipType, err = IsAccessListOwner(ctx, stubUserMeetsAllRequires, acl4, accessListAndMembersGetter, nil, clock)
	require.NoError(t, err)
	// Should have inherited ownership from acl1's inclusion in acl4's Owners.
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, ownershipType)

	stubUserMeetsAllRequires.SetName(member2)
	ownershipType, err = IsAccessListOwner(ctx, stubUserMeetsAllRequires, acl4, accessListAndMembersGetter, nil, clock)
	// Should not have ownership.
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, ownershipType)
}

func TestAccessListIsMember(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	acl1 := newAccessList(t, "1", clock)
	acl1m1 := newAccessListMember(t, acl1.GetName(), member1, accesslist.MembershipKindUser, clock)

	locksGetter := &mockLocksGetter{
		targets: map[string][]types.Lock{},
	}
	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1},
		},
		accessLists: map[string]*accesslist.AccessList{
			acl1.GetName(): acl1,
		},
	}

	stubMember1, err := types.NewUser(member1)
	require.NoError(t, err)
	stubMember1.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	stubMember1.SetRoles([]string{"mrole1", "mrole2"})

	membershipType, err := IsAccessListMember(ctx, stubMember1, acl1, accessListAndMembersGetter, locksGetter, clock)
	require.NoError(t, err)
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, membershipType)

	// When user is Locked, should not be considered a Member.
	lock, err := types.NewLock("user-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: member1,
		},
	})
	require.NoError(t, err)
	locksGetter.targets[member1] = []types.Lock{lock}

	membershipType, err = IsAccessListMember(ctx, stubMember1, acl1, accessListAndMembersGetter, locksGetter, clock)
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, membershipType)
}

func TestAccessListIsMember_RequirementsAndExpiry(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()
	acl := newAccessList(t, "acl", clock)

	// single user member
	member := newAccessListMember(t, "acl", "u", accesslist.MembershipKindUser, clock)
	aclGetter := &mockAccessListAndMembersGetter{
		accessLists: map[string]*accesslist.AccessList{"acl": acl},
		members:     map[string][]*accesslist.AccessListMember{"acl": {member}},
	}

	u, _ := types.NewUser("u")
	u.SetRoles([]string{"wrong-role"})
	u.SetTraits(map[string][]string{})
	locks := &mockLocksGetter{}

	// Missing membershipRequires should be AccessDenied
	typ, err := IsAccessListMember(ctx, u, acl, aclGetter, locks, clock)
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)

	// Give correct traits/roles, but expire the membership
	u.SetRoles([]string{"mrole1", "mrole2"})
	u.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	// advance clock past Expires
	clock.Advance(48 * time.Hour)

	typ, err = IsAccessListMember(ctx, u, acl, aclGetter, locks, clock)
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))
}

func TestAccessListIsMember_NestedRequirements(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := t.Context()
	locks := &mockLocksGetter{}

	t.Run("nested lists with requirements at multiple levels", func(t *testing.T) {
		rootList := newAccessList(t, "root", clock)
		rootList.Spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"root-role"},
		}

		middleList := newAccessList(t, "middle", clock)
		middleList.Spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"middle-role"},
		}

		leafList := newAccessList(t, "leaf", clock)
		leafList.Spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"leaf-role"},
		}

		const userName = "alice"

		userMember := newAccessListMember(t, leafList.GetName(), userName, accesslist.MembershipKindUser, clock)
		leafInMiddle := newAccessListMember(t, middleList.GetName(), leafList.GetName(), accesslist.MembershipKindList, clock)
		middleInRoot := newAccessListMember(t, rootList.GetName(), middleList.GetName(), accesslist.MembershipKindList, clock)

		aclGetter := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				"root":   rootList,
				"middle": middleList,
				"leaf":   leafList,
			},
			members: map[string][]*accesslist.AccessListMember{
				"root":   {middleInRoot},
				"middle": {leafInMiddle},
				"leaf":   {userMember},
			},
		}

		user, err := types.NewUser(userName)
		require.NoError(t, err)
		allRoles := slices.Concat(
			rootList.Spec.MembershipRequires.Roles,
			middleList.Spec.MembershipRequires.Roles,
			leafList.Spec.MembershipRequires.Roles,
		)
		user.SetRoles(allRoles)

		typ, err := IsAccessListMember(ctx, user, rootList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, typ)

		typ, err = IsAccessListMember(ctx, user, middleList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, typ)

		typ, err = IsAccessListMember(ctx, user, leafList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, typ)

		// User missing middle role
		missingMiddleRoles := slices.Concat(
			rootList.Spec.MembershipRequires.Roles,
			leafList.Spec.MembershipRequires.Roles,
		)
		user.SetRoles(missingMiddleRoles)

		typ, err = IsAccessListMember(ctx, user, rootList, aclGetter, locks, clock)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
	})

	t.Run("nested lists with expired list membership in the middle", func(t *testing.T) {
		rootList := newAccessList(t, "root", clock)
		middleList := newAccessList(t, "middle", clock)
		leafList := newAccessList(t, "leaf", clock)

		const userName = "alice"

		userMember := newAccessListMember(t, leafList.GetName(), userName, accesslist.MembershipKindUser, clock)
		// middle -> leaf membership expires in 12 hours while the other memberships expire in 24h
		leafInMiddle := newAccessListMember(t, middleList.GetName(), leafList.GetName(), accesslist.MembershipKindList, clockwork.NewFakeClockAt(clock.Now().Add(-12*time.Hour)))
		middleInRoot := newAccessListMember(t, rootList.GetName(), middleList.GetName(), accesslist.MembershipKindList, clock)

		aclGetter := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				"root":   rootList,
				"middle": middleList,
				"leaf":   leafList,
			},
			members: map[string][]*accesslist.AccessListMember{
				"root":   {middleInRoot},
				"middle": {leafInMiddle},
				"leaf":   {userMember},
			},
		}

		user, err := types.NewUser(userName)
		require.NoError(t, err)
		user.SetRoles(rootList.Spec.MembershipRequires.Roles)
		user.SetTraits(rootList.Spec.MembershipRequires.Traits)

		typ, err := IsAccessListMember(ctx, user, rootList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, typ)

		// advancing the clock makes the middle -> leaf membership expire
		clock.Advance(14 * time.Hour)

		typ, err = IsAccessListMember(ctx, user, rootList, aclGetter, locks, clock)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
	})

	t.Run("cyclic graph, no membership", func(t *testing.T) {
		firstList := newAccessList(t, "first", clock)
		secondList := newAccessList(t, "second", clock)
		thirdList := newAccessList(t, "third", clock)

		firstArc := newAccessListMember(t, firstList.GetName(), secondList.GetName(), accesslist.MembershipKindList, clock)
		secondArc := newAccessListMember(t, secondList.GetName(), thirdList.GetName(), accesslist.MembershipKindList, clock)
		thirdArc := newAccessListMember(t, thirdList.GetName(), firstList.GetName(), accesslist.MembershipKindList, clock)

		aclGetter := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				firstList.GetName():  firstList,
				secondList.GetName(): secondList,
				thirdList.GetName():  thirdList,
			},
			members: map[string][]*accesslist.AccessListMember{
				firstList.GetName():  {firstArc},
				secondList.GetName(): {secondArc},
				thirdList.GetName():  {thirdArc},
			},
		}

		user, err := types.NewUser("alice")
		require.NoError(t, err)
		// Make sure the user meets the membership requirements.
		user.SetRoles(firstList.Spec.MembershipRequires.Roles)
		user.SetTraits(firstList.Spec.MembershipRequires.Traits)

		typ, err := IsAccessListMember(ctx, user, firstList, aclGetter, locks, clock)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
		typ, err = IsAccessListMember(ctx, user, secondList, aclGetter, locks, clock)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
		typ, err = IsAccessListMember(ctx, user, thirdList, aclGetter, locks, clock)
		require.Error(t, err)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
	})

	t.Run("cyclic graph, user membership", func(t *testing.T) {
		firstList := newAccessList(t, "first", clock)
		secondList := newAccessList(t, "second", clock)
		thirdList := newAccessList(t, "third", clock)

		user, err := types.NewUser("alice")
		require.NoError(t, err)
		// Make sure the user meets the membership requirements.
		user.SetRoles(firstList.Spec.MembershipRequires.Roles)
		user.SetTraits(firstList.Spec.MembershipRequires.Traits)

		firstArc := newAccessListMember(t, firstList.GetName(), secondList.GetName(), accesslist.MembershipKindList, clock)
		secondArc := newAccessListMember(t, secondList.GetName(), thirdList.GetName(), accesslist.MembershipKindList, clock)
		thirdArc := newAccessListMember(t, thirdList.GetName(), firstList.GetName(), accesslist.MembershipKindList, clock)
		userMembership := newAccessListMember(t, thirdList.GetName(), user.GetName(), accesslist.MembershipKindUser, clock)

		aclGetter := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				firstList.GetName():  firstList,
				secondList.GetName(): secondList,
				thirdList.GetName():  thirdList,
			},
			members: map[string][]*accesslist.AccessListMember{
				firstList.GetName():  {firstArc},
				secondList.GetName(): {secondArc},
				thirdList.GetName():  {thirdArc, userMembership},
			},
		}

		typ, err := IsAccessListMember(ctx, user, firstList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, typ)
		typ, err = IsAccessListMember(ctx, user, secondList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, typ)
		typ, err = IsAccessListMember(ctx, user, thirdList, aclGetter, locks, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, typ)
	})
}

func TestGetOwners(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// Create Access Lists
	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)

	// Set up owners
	// acl1 is owned by user "ownerA" and access list acl2
	acl1.Spec.Owners = []accesslist.Owner{
		{
			Name:           "ownerA",
			MembershipKind: accesslist.MembershipKindUser,
		},
		{
			Name:           acl2.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	}
	acl2.Status.OwnerOf = append(acl2.Status.OwnerOf, acl1.GetName())

	// acl2 is owned by user "ownerB" and access list aclC
	acl2.Spec.Owners = []accesslist.Owner{
		{
			Name:           "ownerB",
			MembershipKind: accesslist.MembershipKindUser,
		},
		{
			Name:           acl3.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	}
	acl3.Status.OwnerOf = append(acl3.Status.OwnerOf, acl2.GetName())

	// acl3 is owned by user "ownerC"
	acl3.Spec.Owners = []accesslist.Owner{
		{
			Name:           "ownerC",
			MembershipKind: accesslist.MembershipKindUser,
		},
	}

	// Set up members for owner lists
	// aclB has member "memberB"
	acl2m1 := newAccessListMember(t, acl2.GetName(), "memberB", accesslist.MembershipKindUser, clock)
	// aclC has member "memberC"
	acl3m1 := newAccessListMember(t, acl3.GetName(), "memberC", accesslist.MembershipKindUser, clock)

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {acl3m1},
		},
	}

	// Test GetOwners for acl1
	owners, err := GetOwnersFor(ctx, acl1, accessListAndMembersGetter)
	require.NoError(t, err)

	// Expected owners:
	// - Direct owner: "ownerA"
	// - Inherited owners via acl2 (since acl2 is an owner of acl1):
	//   - Members of acl2: "memberB"
	// Note: Owners of acl2 ("ownerB") and members/owners of acl3 are not inherited by acl1

	expectedOwners := map[string]bool{
		"ownerA":         true, // Direct owner of acl1
		acl2m1.GetName(): true, // Member of acl2 (owner list of acl1)
	}

	actualOwners := make(map[string]bool)
	for _, owner := range owners {
		actualOwners[owner.Name] = true
	}

	require.Equal(t, expectedOwners, actualOwners, "Owners do not match expected owners")

	// Test GetOwners for acl2
	owners, err = GetOwnersFor(ctx, acl2, accessListAndMembersGetter)
	require.NoError(t, err)

	// Expected owners:
	// - Direct owner: "ownerB"
	// - Inherited owners via acl3 (since acl3 is an owner of acl2):
	//   - Members of acl3: "memberC"

	expectedOwners = map[string]bool{
		"ownerB":         true, // Direct owner of acl2
		acl3m1.GetName(): true, // Member of acl3 (owner list of acl2)
	}

	actualOwners = make(map[string]bool)
	for _, owner := range owners {
		actualOwners[owner.Name] = true
	}

	require.Equal(t, expectedOwners, actualOwners, "Owners do not match expected owners")
}

func TestGetInheritedGrants(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	aclroot := newAccessList(t, "root", clock)
	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)

	// aclroot has a trait for owners - "root-owner-trait", and a role for owners - "root-owner-role"
	aclroot.Spec.OwnerGrants = accesslist.Grants{
		Traits: map[string][]string{
			"root-owner-trait": {"root-owner-value"},
		},
		Roles: []string{"root-owner-role"},
	}

	// acl1 has a trait for members - "1-member-trait", and a role for members - "1-member-role"
	acl1.Spec.Grants = accesslist.Grants{
		Traits: map[string][]string{
			"1-member-trait": {"1-member-value"},
		},
		Roles: []string{"1-member-role"},
	}

	// acl2 has no traits or roles
	acl2.Spec.Grants = accesslist.Grants{}

	aclroot.SetOwners([]accesslist.Owner{
		{
			Name:           acl1.GetName(),
			MembershipKind: accesslist.MembershipKindList,
		},
	})
	acl1.Status.OwnerOf = append(acl1.Status.OwnerOf, aclroot.GetName())

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		accessLists: map[string]*accesslist.AccessList{
			aclroot.GetName(): aclroot,
			acl1.GetName():    acl1,
			acl2.GetName():    acl2,
		},
	}

	// acl1 is an Owner of aclroot, and acl2 is a Member of acl1.
	acl2.Status.MemberOf = append(acl2.Status.MemberOf, acl1.GetName())

	// so, members of acl2 should inherit aclroot's owner grants, and acl1's member grants.
	expectedGrants := &accesslist.Grants{
		Traits: map[string][]string{
			"1-member-trait":   {"1-member-value"},
			"root-owner-trait": {"root-owner-value"},
		},
		Roles: []string{"1-member-role", "root-owner-role"},
	}

	grants, err := GetInheritedGrants(ctx, acl2, accessListAndMembersGetter)
	require.NoError(t, err)
	require.Equal(t, expectedGrants, grants)
}

func TestGetInheritedRequires(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	type testListSpec struct {
		name     string
		requires accesslist.Requires
		memberOf []string
	}

	tests := []struct {
		name       string
		lists      []testListSpec
		targetName string
		expected   *accesslist.Requires
	}{
		{
			name: "NilLeafRequires_Chain3",
			lists: []testListSpec{
				{
					name: "root",
					requires: accesslist.Requires{
						Roles: []string{"root-role"},
						Traits: map[string][]string{
							"env": {"prod"},
						},
					},
				},
				{
					name: "intermediate",
					requires: accesslist.Requires{
						Roles: []string{"intermediate-role"},
						Traits: map[string][]string{
							"env":  {"testing"},
							"team": {"backend"},
						},
					},
					memberOf: []string{"root"},
				},
				{
					name:     "leaf",
					requires: accesslist.Requires{},
					memberOf: []string{"intermediate"},
				},
			},
			targetName: "leaf",
			expected: &accesslist.Requires{
				Roles: []string{"intermediate-role", "root-role"},
				Traits: trait.Traits{
					"env":  {"prod", "testing"},
					"team": {"backend"},
				},
			},
		},
		{
			name: "LeafHasOwnRequires_Chain3",
			lists: []testListSpec{
				{
					name: "root",
					requires: accesslist.Requires{
						Roles: []string{"root-role"},
						Traits: trait.Traits{
							"env": {"prod"},
						},
					},
				},
				{
					name: "intermediate",
					requires: accesslist.Requires{
						Roles: []string{"app-role"},
						Traits: trait.Traits{
							"env":  {"staging"},
							"team": {"backend"},
						},
					},
					memberOf: []string{"root"},
				},
				{
					name: "leaf",
					requires: accesslist.Requires{
						Roles: []string{"leaf-role"},
						Traits: trait.Traits{
							"team":   {"infra"},
							"region": {"us-east-1"},
						},
					},
					memberOf: []string{"intermediate"},
				},
			},
			targetName: "leaf",
			expected: &accesslist.Requires{
				Roles: []string{"app-role", "leaf-role", "root-role"},
				Traits: trait.Traits{
					"env":    {"prod", "staging"},
					"team":   {"backend", "infra"},
					"region": {"us-east-1"},
				},
			},
		},
		{
			name: "NilLeafRequires_Diamond",
			lists: []testListSpec{
				{
					name: "root",
					requires: accesslist.Requires{
						Roles: []string{"root-role"},
						Traits: trait.Traits{
							"env": {"prod"},
						},
					},
				},
				{
					name: "parent-a",
					requires: accesslist.Requires{
						Roles: []string{"a-role"},
						Traits: trait.Traits{
							"env":  {"staging"},
							"team": {"a-team"},
						},
					},
					memberOf: []string{"root"},
				},
				{
					name: "parent-b",
					requires: accesslist.Requires{
						Roles: []string{"b-role"},
						Traits: trait.Traits{
							"team":   {"b-team"},
							"region": {"eu-west-1"},
						},
					},
					memberOf: []string{"root"},
				},
				{
					name:     "leaf",
					requires: accesslist.Requires{},
					memberOf: []string{"parent-a", "parent-b"},
				},
			},
			targetName: "leaf",
			expected: &accesslist.Requires{
				Roles: []string{"a-role", "b-role", "root-role"},
				Traits: trait.Traits{
					"env":    {"prod", "staging"},
					"team":   {"a-team", "b-team"},
					"region": {"eu-west-1"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			listsByID := make(map[string]*accesslist.AccessList, len(tc.lists))
			listsByName := make(map[string]*accesslist.AccessList, len(tc.lists))
			for _, ls := range tc.lists {
				acl, err := accesslist.NewAccessList(
					header.Metadata{
						Name: uuid.NewString(),
					},
					accesslist.Spec{
						Title:              ls.name,
						Description:        ls.name,
						MembershipRequires: ls.requires,
					},
				)
				require.NoError(t, err)
				listsByID[acl.GetName()] = acl
				listsByName[ls.name] = acl
			}

			members := make(map[string][]*accesslist.AccessListMember)
			for _, ls := range tc.lists {
				child := listsByName[ls.name]
				for _, parentName := range ls.memberOf {
					parent := listsByName[parentName]
					child.Status.MemberOf = append(child.Status.MemberOf, parent.GetName())
					members[parent.GetName()] = append(members[parent.GetName()], newAccessListMember(t, parent.GetName(), child.GetName(), accesslist.MembershipKindList, clock))
				}
			}

			getter := &mockAccessListAndMembersGetter{
				accessLists: listsByID,
				members:     members,
			}

			leaf := listsByName[tc.targetName]
			originalLeafRequires := leaf.Spec.MembershipRequires.Clone()

			requires, err := GetInheritedMembershipRequires(ctx, leaf, getter)
			require.NoError(t, err)
			// Should be expected
			require.Equal(t, tc.expected, requires)
			// Original should not be mutated
			require.Equal(t, originalLeafRequires, leaf.Spec.MembershipRequires)
		})
	}
}

func TestGetMembersFor_FlattensAndStopsOnCycles(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	// A -> B -> C -> B (cycle)
	a := newAccessList(t, "A", clock)
	b := newAccessList(t, "B", clock)
	c := newAccessList(t, "C", clock)

	getter := &mockAccessListAndMembersGetter{
		accessLists: map[string]*accesslist.AccessList{
			"A": a, "B": b, "C": c,
		},
		members: map[string][]*accesslist.AccessListMember{
			"A": {newAccessListMember(t, "A", "userA", accesslist.MembershipKindUser, clock),
				newAccessListMember(t, "A", "B", accesslist.MembershipKindList, clock)},
			"B": {newAccessListMember(t, "B", "userB", accesslist.MembershipKindUser, clock),
				newAccessListMember(t, "B", "C", accesslist.MembershipKindList, clock)},
			"C": {newAccessListMember(t, "C", "userC", accesslist.MembershipKindUser, clock),
				newAccessListMember(t, "C", "B", accesslist.MembershipKindList, clock)}, // cycle back
		},
	}

	members, err := GetMembersFor(ctx, "A", getter)
	require.NoError(t, err)

	names := make([]string, 0, len(members))
	for _, m := range members {
		names = append(names, m.GetName())
	}
	sort.Strings(names)

	// Should be userA, userB, userC exactly once each
	require.Equal(t, []string{"userA", "userB", "userC"}, names)
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title:       name,
			Description: "test access list",
			Owners: []accesslist.Owner{
				{Name: ownerUser, Description: "owner user", MembershipKind: accesslist.MembershipKindUser},
				{Name: ownerUser2, Description: "owner user 2", MembershipKind: accesslist.MembershipKindUser},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now().Add(time.Hour * 24 * 365),
				Notifications: accesslist.Notifications{
					Start: 336 * time.Hour, // Two weeks.
				},
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

func newAccessListMember(t *testing.T, accessListName, memberName string, memberKind string, clk clockwork.Clock) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: memberName},
		accesslist.AccessListMemberSpec{
			AccessList:     accessListName,
			Name:           memberName,
			Joined:         clk.Now().UTC(),
			Expires:        clk.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        "tester",
			MembershipKind: memberKind,
		})
	require.NoError(t, err)
	return member
}

func generateAccessList(name string) *accesslist.AccessList {
	return &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name: name,
			},
		},
	}
}

func generateNestedALs(level, directMembers int, rootListName, userName string) (map[string]*accesslist.AccessList, map[string][]*accesslist.AccessListMember) {
	accesslists := []*accesslist.AccessList{generateAccessList(rootListName)}
	members := make(map[string][]*accesslist.AccessListMember)

	for i := range level - 1 {
		parentName := accesslists[i].GetName()
		name := "nested-al-" + strconv.Itoa(i)
		accesslists = append(accesslists, generateAccessList(name))
		listMembers := generateUserMembers(directMembers/2, name)
		listMembers = append(listMembers, &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: name,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     parentName,
				Name:           name,
				MembershipKind: accesslist.MembershipKindList,
			},
		})
		listMembers = append(listMembers, generateUserMembers(directMembers/2+directMembers%2, name)...)

		listMembers = append(listMembers, &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: userName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     parentName,
				Name:           userName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		})

		members[parentName] = listMembers
	}

	alMap := make(map[string]*accesslist.AccessList)
	for _, al := range accesslists {
		alMap[al.GetName()] = al
	}
	return alMap, members
}

func generateUserMembers(count int, alName string) []*accesslist.AccessListMember {
	var members []*accesslist.AccessListMember
	for i := range count {
		memberName := "member-" + strconv.Itoa(i)
		members = append(members, &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: memberName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     alName,
				Name:           memberName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		})
	}
	return members
}

func BenchmarkIsAccessListMember(b *testing.B) {
	if skip, _ := strconv.ParseBool(os.Getenv("BENCH_SKIP_MICRO")); skip {
		b.Skip("skipping micro benchmark")
	}
	const mainAccessListName = "main-al"
	const testUserName = "test-user"

	lockGetter := &mockLocksGetter{}
	clock := clockwork.NewFakeClock()

	b.Run("no accessPaths", func(b *testing.B) {
		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: {},
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if !trace.IsAccessDenied(err) {
				b.Fatal(err)
			}
		}
	})

	b.Run("single-page direct member", func(b *testing.B) {
		member := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: testUserName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     mainAccessListName,
				Name:           testUserName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		}
		generatedMembers := generateUserMembers(50, mainAccessListName)
		// We inject the member we are looking for in the middle of the member list
		members := append(generatedMembers[:25], member)
		members = append(members, generatedMembers[25:]...)

		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: members,
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple-pages direct member", func(b *testing.B) {
		member := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: testUserName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     mainAccessListName,
				Name:           testUserName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		}
		generatedMembers := generateUserMembers(500, mainAccessListName)
		// We inject the member we are looking for in the middle of the member list
		members := append(generatedMembers[:250], member)
		members = append(members, generatedMembers[250:]...)

		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: members,
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("single-page nested member", func(b *testing.B) {
		lists, members := generateNestedALs(5, 0, mainAccessListName, testUserName)
		mock := &mockAccessListAndMembersGetter{
			accessLists: lists,
			members:     members,
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple pages nested member", func(b *testing.B) {
		lists, members := generateNestedALs(5, 501, mainAccessListName, testUserName)
		mock := &mockAccessListAndMembersGetter{
			accessLists: lists,
			members:     members,
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
