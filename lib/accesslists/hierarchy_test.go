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
	"sort"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
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
	require.ErrorIs(t, err, trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", member1, acl1.Spec.Title))
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
	require.ErrorIs(t, err, trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", member1, acl4.Spec.Title))
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
	require.NoError(t, err)
	// Should not have ownership.
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
	require.ErrorIs(t, err, trace.AccessDenied("User %q is currently locked", member1))
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
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, typ)
	require.ErrorIs(t, err, trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", u.GetName(), acl.GetName()))

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
	require.ErrorIs(t, err, trace.AccessDenied("User '%s's membership in Access List '%s' has expired", u.GetName(), acl.GetName()))
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
