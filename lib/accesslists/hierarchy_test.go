package accesslists

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

// Mock implementation of AccessListMembersGetter.
type mockMembersGetter struct {
	members map[string][]*accesslist.AccessListMember
}

func (m *mockMembersGetter) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	members, exists := m.members[accessListName]
	if !exists {
		return nil, "", nil
	}
	return members, "", nil
}

const (
	ownerUser  = "ownerUser"
	ownerUser2 = "ownerUser2"
	member1    = "member1"
	member2    = "member2"
)

func TestNewAccessListHierarchy(t *testing.T) {
	clock := clockwork.NewFakeClock()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)
	acl4 := newAccessList(t, "4", clock)
	acl5 := newAccessList(t, "5", clock)

	// acl1 -> acl2 -> acl3
	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)

	// acl4 -> acl1,acl2
	acl4m1 := newAccessListMember(t, acl4.GetName(), acl1.GetName(), accesslist.MembershipKindList, clock)
	acl4m2 := newAccessListMember(t, acl4.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)

	acl5.Spec.Owners = append(acl5.Spec.Owners, accesslist.Owner{
		Name:           acl4.GetName(),
		Description:    "asdf",
		MembershipKind: accesslist.MembershipKindList,
	})

	membersGetter := &mockMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1},
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {},
			acl4.GetName(): {acl4m1, acl4m2},
			acl5.GetName(): {},
		},
	}

	// Hierarchy should be built successfully.
	hierarchy, err := NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3, acl4, acl5}, membersGetter)
	require.NoError(t, err)

	// Test IsDescendant
	isDescendant, err := hierarchy.IsDescendant(acl1.GetName(), acl2.GetName(), RelationshipKindMember)
	require.NoError(t, err)
	require.True(t, isDescendant)

	isDescendant, err = hierarchy.IsDescendant(acl1.GetName(), acl3.GetName(), RelationshipKindMember)
	require.NoError(t, err)
	require.True(t, isDescendant)

	isDescendant, err = hierarchy.IsDescendant(acl1.GetName(), acl4.GetName(), RelationshipKindMember)
	require.NoError(t, err)
	require.False(t, isDescendant)

	isDescendantOwner, err := hierarchy.IsDescendant(acl5.GetName(), acl4.GetName(), RelationshipKindOwner)
	require.NoError(t, err)
	// Unlike inherited Ownership on an Identity, being a 'descendant Owner list'
	// does not require a level of nesting within a Member list first.
	require.True(t, isDescendantOwner)

	isDescendantOwner, err = hierarchy.IsDescendant(acl5.GetName(), acl1.GetName(), RelationshipKindOwner)
	require.NoError(t, err)
	require.True(t, isDescendantOwner)
}

func TestAccessListHierarchyDepthCheck(t *testing.T) {
	clock := clockwork.NewFakeClock()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)
	acl4 := newAccessList(t, "4", clock)
	acl5 := newAccessList(t, "5", clock)
	acl6 := newAccessList(t, "6", clock)
	acl7 := newAccessList(t, "7", clock)
	acl8 := newAccessList(t, "8", clock)
	acl9 := newAccessList(t, "9", clock)
	acl10 := newAccessList(t, "10", clock)
	acl11 := newAccessList(t, "11", clock)
	acl12 := newAccessList(t, "12", clock)

	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)
	acl3m1 := newAccessListMember(t, acl3.GetName(), acl4.GetName(), accesslist.MembershipKindList, clock)
	acl4m1 := newAccessListMember(t, acl4.GetName(), acl5.GetName(), accesslist.MembershipKindList, clock)
	acl5m1 := newAccessListMember(t, acl5.GetName(), acl6.GetName(), accesslist.MembershipKindList, clock)
	acl6m1 := newAccessListMember(t, acl6.GetName(), acl7.GetName(), accesslist.MembershipKindList, clock)
	acl7m1 := newAccessListMember(t, acl7.GetName(), acl8.GetName(), accesslist.MembershipKindList, clock)
	acl8m1 := newAccessListMember(t, acl8.GetName(), acl9.GetName(), accesslist.MembershipKindList, clock)
	acl9m1 := newAccessListMember(t, acl9.GetName(), acl10.GetName(), accesslist.MembershipKindList, clock)
	acl10m1 := newAccessListMember(t, acl10.GetName(), acl11.GetName(), accesslist.MembershipKindList, clock)
	acl11m1 := newAccessListMember(t, acl11.GetName(), acl12.GetName(), accesslist.MembershipKindList, clock)

	membersGetter := &mockMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName():  {acl1m1},
			acl2.GetName():  {acl2m1},
			acl3.GetName():  {acl3m1},
			acl4.GetName():  {acl4m1},
			acl5.GetName():  {acl5m1},
			acl6.GetName():  {acl6m1},
			acl7.GetName():  {acl7m1},
			acl8.GetName():  {acl8m1},
			acl9.GetName():  {acl9m1},
			acl10.GetName(): {acl10m1},
			acl11.GetName(): {},
			acl12.GetName(): {},
		},
	}

	// Should create successfully.
	hierarchy, err := NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3, acl4, acl5, acl6, acl7, acl8, acl9, acl10, acl11, acl12}, membersGetter)
	require.NoError(t, err)

	// Validation should fail due to max depth.
	err = hierarchy.ValidateAccessListMember(acl11.GetName(), acl11m1)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", acl12.Spec.Title, acl11.Spec.Title, accesslist.MaxAllowedDepth))

	membersGetter.members[acl11.GetName()] = []*accesslist.AccessListMember{acl11m1}

	// After 'creating' the member that links acl6 to acl7, validation should fail as max depth is 11 (acl1 -> acl12).
	hierarchy, err = NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3, acl4, acl5, acl6, acl7, acl8, acl9, acl10, acl11, acl12}, membersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", acl12.Spec.Title, acl11.Spec.Title, accesslist.MaxAllowedDepth))
}

func TestAccessListHierarchyCircularRefsCheck(t *testing.T) {
	clock := clockwork.NewFakeClock()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)

	// acl1 -> acl2 -> acl3
	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)

	// acl3 -> acl1
	acl3m1 := newAccessListMember(t, acl3.GetName(), acl1.GetName(), accesslist.MembershipKindList, clock)

	membersGetter := &mockMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1},
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {},
		},
	}

	// Hierarchy should be built successfully.
	hierarchy, err := NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3}, membersGetter)
	require.NoError(t, err)

	// Circular references should not be allowed.
	err = hierarchy.ValidateAccessListMember(acl3.GetName(), acl3m1)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because '%s' is already included as a Member or Owner in '%s'", acl1.Spec.Title, acl3.Spec.Title, acl3.Spec.Title, acl1.Spec.Title))

	membersGetter.members[acl3.GetName()] = []*accesslist.AccessListMember{acl3m1}

	// After 'creating' the member that links acl3 to acl1, validation should fail due to circular reference.
	_, err = NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3}, membersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because '%s' is already included as a Member or Owner in '%s'", acl1.Spec.Title, acl3.Spec.Title, acl3.Spec.Title, acl1.Spec.Title))

	// Circular references with Ownership should also be disallowed.
	acl4 := newAccessList(t, "4", clock)
	acl5 := newAccessList(t, "5", clock)

	// acl4 includes acl5 as a Member
	acl4m1 := newAccessListMember(t, acl4.GetName(), acl5.GetName(), accesslist.MembershipKindList, clock)

	// acl5 includes acl4 as an Owner.
	acl5.Spec.Owners = append(acl5.Spec.Owners, accesslist.Owner{
		Name:           acl4.GetName(),
		Description:    "asdf",
		MembershipKind: accesslist.MembershipKindList,
	})

	membersGetter = &mockMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl4.GetName(): {acl4m1},
			acl5.GetName(): {},
		},
	}

	_, err = NewHierarchy(context.Background(), []*accesslist.AccessList{acl4, acl5}, membersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as an Owner of '%s' because '%s' is already included as a Member or Owner in '%s'", acl4.Spec.Title, acl5.Spec.Title, acl5.Spec.Title, acl4.Spec.Title))
}

func TestAccessListHierarchyIsOwner(t *testing.T) {
	clock := clockwork.NewFakeClock()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)
	acl4 := newAccessList(t, "4", clock)

	// acl1 -> acl2 -> acl3 as members
	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl1m2 := newAccessListMember(t, acl1.GetName(), member1, accesslist.MembershipKindUser, clock)
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)
	acl4m1 := newAccessListMember(t, acl4.GetName(), member2, accesslist.MembershipKindUser, clock)

	// acl4 -> acl1 as owner
	acl4.Spec.Owners = append(acl4.Spec.Owners, accesslist.Owner{
		Name:           acl1.GetName(),
		Description:    "asdf",
		MembershipKind: accesslist.MembershipKindList,
	})

	membersGetter := &mockMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1, acl1m2},
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {},
			acl4.GetName(): {acl4m1},
		},
	}

	// Hierarchy should be built successfully.
	hierarchy, err := NewHierarchy(context.Background(), []*accesslist.AccessList{acl1, acl2, acl3, acl4}, membersGetter)
	require.NoError(t, err)

	// User which does not meet acl1's Membership requirements.
	stubUserNoRequires, err := types.NewUser(member1)
	require.NoError(t, err)

	ownershipType, err := hierarchy.IsAccessListOwner(stubUserNoRequires, acl4.GetName())
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("User '%s' does not meet the membership requirements for Access List '%s'", member1, acl1.Spec.Title))
	// Should not have inherited ownership due to missing OwnershipRequires.
	require.Equal(t, MembershipOrOwnershipTypeNone, ownershipType)

	// User which only meets acl1's Membership requirements.
	stubUserMeetsMemberRequires, err := types.NewUser(member1)
	require.NoError(t, err)
	stubUserMeetsMemberRequires.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	stubUserMeetsMemberRequires.SetRoles([]string{"mrole1", "mrole2"})

	ownershipType, err = hierarchy.IsAccessListOwner(stubUserMeetsMemberRequires, acl4.GetName())
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("User '%s' does not meet the ownership requirements for Access List '%s'", member1, acl4.Spec.Title))
	require.Equal(t, MembershipOrOwnershipTypeNone, ownershipType)

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

	ownershipType, err = hierarchy.IsAccessListOwner(stubUserMeetsAllRequires, acl4.GetName())
	require.NoError(t, err)
	// Should have inherited ownership from acl1's inclusion in acl4's Owners.
	require.Equal(t, MembershipOrOwnershipTypeInherited, ownershipType)

	stubUserMeetsAllRequires.SetName(member2)
	ownershipType, err = hierarchy.IsAccessListOwner(stubUserMeetsAllRequires, acl4.GetName())
	require.NoError(t, err)
	// Should not have ownership.
	require.Equal(t, MembershipOrOwnershipTypeNone, ownershipType)
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

func newAccessListMember(t *testing.T, accessListName, memberName string, memberKind string, clock clockwork.Clock) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     accessListName,
			Name:           memberName,
			Joined:         clock.Now().UTC(),
			Expires:        clock.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        "maxim.dietz@goteleport.com",
			MembershipKind: memberKind,
		},
	)
	require.NoError(t, err)

	return member
}
