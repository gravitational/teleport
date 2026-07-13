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
	"github.com/gravitational/teleport/lib/scopes"
)

// Mock implementation of AccessListAndMembersGetter.
type mockAccessListAndMembersGetter struct {
	accessLists map[NormalizedSQN]*accesslist.AccessList
	members     map[NormalizedSQN][]*accesslist.AccessListMember
}

func (m *mockAccessListAndMembersGetter) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	return m.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessList: accessListName,
		MemberName: memberName,
	}.Build())
}

func (m *mockAccessListAndMembersGetter) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	accessListName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	memberName := NormalizedSQN(scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	})
	members, exists := m.members[accessListName]
	if !exists {
		return nil, trace.NotFound("access list %s member %s not found", accessListName.String(), memberName.String())
	}
	for _, m := range members {
		memberSQN, err := MemberScopeQualifiedName(m)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if memberSQN == memberName {
			return m, nil
		}
	}
	return nil, trace.NotFound("access list %s member %s not found", accessListName.String(), memberName.String())
}

func (m *mockAccessListAndMembersGetter) GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error) {
	return m.GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Name: accessListName,
	}.Build())
}

func (m *mockAccessListAndMembersGetter) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	accessListName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
	accessList, exists := m.accessLists[accessListName]
	if !exists {
		return nil, trace.NotFound("access list %s not found", accessListName.String())
	}
	return accessList, nil
}

func (m *mockAccessListAndMembersGetter) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return m.ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessList: accessListName,
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
	}.Build())
}

func (m *mockAccessListAndMembersGetter) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	accessListName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	members, exists := m.members[accessListName]
	if !exists {
		return nil, "", nil
	}
	return members, "", nil
}

func mockAccessLists(accessLists ...*accesslist.AccessList) map[NormalizedSQN]*accesslist.AccessList {
	out := make(map[NormalizedSQN]*accesslist.AccessList, len(accessLists))
	for _, accessList := range accessLists {
		out[ScopeQualifiedName(accessList)] = accessList
	}
	return out
}

func mockAccessListMembers(t testing.TB, members ...*accesslist.AccessListMember) map[NormalizedSQN][]*accesslist.AccessListMember {
	out := make(map[NormalizedSQN][]*accesslist.AccessListMember)
	for _, member := range members {
		listName, err := ParentListOf(member)
		require.NoError(t, err)
		out[listName] = append(out[listName], member)
	}
	return out
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
		members:     mockAccessListMembers(t, acl1m1, acl1m2, acl2m1, acl4m1),
		accessLists: mockAccessLists(acl1, acl2, acl3, acl4),
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

// TestIsAccessListOwnerScopedNameCollision asserts that IsAccessListOwner does
// not consider a user to be an owner of a scoped access list if they are
// actually a member of an unscoped access list with the same unqualified name
// as a legimate scoped owner list.
func TestIsAccessListOwnerScopedNameCollision(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	unscopedOwnerList := newAccessList(t, "owners", clock)
	scopedOwnerList := newAccessList(t, "owners", clock)
	scopedOwnerList.Scope = "/eng"
	targetList := newAccessList(t, "target", clock)
	targetList.Scope = "/eng/app"
	targetList.Spec.Owners = []accesslist.Owner{{
		Name:           ScopeQualifiedName(scopedOwnerList).String(),
		MembershipKind: accesslist.MembershipKindScopedList,
	}}

	unscopedOwnerMember := newAccessListMember(t, unscopedOwnerList.GetName(), member1, accesslist.MembershipKindUser, clock)
	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members:     mockAccessListMembers(t, unscopedOwnerMember),
		accessLists: mockAccessLists(unscopedOwnerList, scopedOwnerList, targetList),
	}

	stubUser, err := types.NewUser(member1)
	require.NoError(t, err)
	stubUser.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
		"otrait1": {"ovalue1", "ovalue2"},
		"otrait2": {"ovalue3", "ovalue4"},
	})
	stubUser.SetRoles([]string{"mrole1", "mrole2", "orole1", "orole2"})

	// Sanity check the user is legimately a member of the unscoped access list.
	_, err = IsAccessListMember(ctx, stubUser, unscopedOwnerList, accessListAndMembersGetter, nil, clock)
	require.NoError(t, err)

	// IsAccessListOwner must return an error when checking if the user is an owner of the target list.
	_, err = IsAccessListOwner(ctx, stubUser, targetList, accessListAndMembersGetter, nil, clock)
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// If we make the unscoped owner list an owner, the user becomes a legimate owner.
	targetList.Spec.Owners = append(targetList.Spec.Owners, accesslist.Owner{
		Name:           unscopedOwnerList.GetName(),
		MembershipKind: accesslist.MembershipKindList,
	})
	assignmentType, err := IsAccessListOwner(ctx, stubUser, targetList, accessListAndMembersGetter, nil, clock)
	require.NoError(t, err)
	require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, assignmentType)
}

// TestMembershipScopedNameCollision asserts that GetHierarchyForUser and
// IsAccessListMember keep direct memberships separate for scoped and unscoped
// access lists with the same unqualified name.
func TestMembershipScopedNameCollision(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	unscopedList := newAccessList(t, "team", clock)
	scopedList := newAccessList(t, "team", clock)
	scopedList.Scope = "/eng"
	parentList := newAccessList(t, "parent", clock)
	parentList.Scope = "/eng/test"

	scopedListName := ScopeQualifiedName(scopedList)
	parentListName := ScopeQualifiedName(parentList)

	unscopedList.Status.ScopedMemberOf = []string{parentListName.String()}
	scopedList.Status.ScopedMemberOf = []string{parentListName.String()}

	unscopedUserMember := newAccessListMember(t, unscopedList.GetName(), member1, accesslist.MembershipKindUser, clock)
	scopedUserMember, err := accesslist.NewAccessListMemberWithScope(
		header.Metadata{Name: member2},
		accesslist.AccessListMemberSpec{
			AccessList:     scopedListName.String(),
			Name:           member2,
			Joined:         clock.Now().UTC(),
			Expires:        clock.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        "tester",
			MembershipKind: accesslist.MembershipKindUser,
		},
		scopedList.GetScope(),
	)
	require.NoError(t, err)
	unscopedListMember, err := accesslist.NewAccessListMemberWithScope(
		header.Metadata{Name: unscopedList.GetName()},
		accesslist.AccessListMemberSpec{
			AccessList:     parentListName.String(),
			Name:           unscopedList.GetName(),
			Joined:         clock.Now().UTC(),
			Expires:        clock.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        "tester",
			MembershipKind: accesslist.MembershipKindList,
		},
		parentList.GetScope(),
	)
	require.NoError(t, err)
	scopedListMember, err := accesslist.NewAccessListMemberWithScope(
		header.Metadata{Name: scopedListName.String()},
		accesslist.AccessListMemberSpec{
			AccessList:     parentListName.String(),
			Name:           scopedListName.String(),
			Joined:         clock.Now().UTC(),
			Expires:        clock.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        "tester",
			MembershipKind: accesslist.MembershipKindScopedList,
		},
		parentList.GetScope(),
	)
	require.NoError(t, err)

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members:     mockAccessListMembers(t, unscopedUserMember, scopedUserMember, unscopedListMember, scopedListMember),
		accessLists: mockAccessLists(unscopedList, scopedList, parentList),
	}

	unscopedUser, err := types.NewUser(member1)
	require.NoError(t, err)
	unscopedUser.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	unscopedUser.SetRoles([]string{"mrole1", "mrole2"})

	scopedUser, err := types.NewUser(member2)
	require.NoError(t, err)
	scopedUser.SetTraits(map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	scopedUser.SetRoles([]string{"mrole1", "mrole2"})

	locksGetter := &mockLocksGetter{
		targets: map[string][]types.Lock{},
	}

	t.Run("GetHierarchyForUser", func(t *testing.T) {
		hierarchy, err := NewHierarchy(HierarchyConfig{
			AccessListsService: accessListAndMembersGetter,
			Clock:              clock,
		})
		require.NoError(t, err)

		// GetHierarchyForUser must not consider a user to be a member of the scoped
		// access list if they are actually a member of the unscoped access list with
		// the same unqualified name.
		memberOf, ownerOf, err := hierarchy.GetHierarchyForUser(ctx, scopedList, unscopedUser)
		require.NoError(t, err)
		require.Empty(t, memberOf)
		require.Empty(t, ownerOf)

		// Sanity check the user is legitimately a member of the unscoped access list.
		memberOf, ownerOf, err = hierarchy.GetHierarchyForUser(ctx, unscopedList, unscopedUser)
		require.NoError(t, err, trace.DebugReport(err))
		require.Equal(t, []*accesslist.AccessList{unscopedList, parentList}, memberOf)
		require.Empty(t, ownerOf)

		// GetHierarchyForUser must not consider a user to be a member of the unscoped
		// access list if they are actually a member of the scoped access list with
		// the same unqualified name.
		memberOf, ownerOf, err = hierarchy.GetHierarchyForUser(ctx, unscopedList, scopedUser)
		require.NoError(t, err)
		require.Empty(t, memberOf)
		require.Empty(t, ownerOf)

		// Sanity check the user is legitimately a member of the scoped access list.
		memberOf, ownerOf, err = hierarchy.GetHierarchyForUser(ctx, scopedList, scopedUser)
		require.NoError(t, err)
		require.Equal(t, []*accesslist.AccessList{scopedList, parentList}, memberOf)
		require.Empty(t, ownerOf)
	})

	t.Run("IsAccessListMember", func(t *testing.T) {
		// IsAccessListMember must not consider a user to be a member of the scoped
		// access list if they are actually a member of the unscoped access list with
		// the same unqualified name.
		_, err := IsAccessListMember(ctx, unscopedUser, scopedList, accessListAndMembersGetter, locksGetter, clock)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))

		// Sanity check the user is legitimately a member of the unscoped access list.
		assignmentType, err := IsAccessListMember(ctx, unscopedUser, unscopedList, accessListAndMembersGetter, locksGetter, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, assignmentType)

		// GetHierarchyForUser must not consider a user to be a member of the unscoped
		// access list if they are actually a member of the scoped access list with
		// the same unqualified name.
		_, err = IsAccessListMember(ctx, scopedUser, unscopedList, accessListAndMembersGetter, locksGetter, clock)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))

		// Sanity check the user is legitimately a member of the scoped access list.
		assignmentType, err = IsAccessListMember(ctx, scopedUser, scopedList, accessListAndMembersGetter, locksGetter, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT, assignmentType)

		// IsAccessListMember should consider both users to be an inherited member of the common parent list.
		assignmentType, err = IsAccessListMember(ctx, unscopedUser, parentList, accessListAndMembersGetter, locksGetter, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, assignmentType)
		assignmentType, err = IsAccessListMember(ctx, scopedUser, parentList, accessListAndMembersGetter, locksGetter, clock)
		require.NoError(t, err)
		require.Equal(t, accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED, assignmentType)
	})
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
		members:     mockAccessListMembers(t, acl1m1),
		accessLists: mockAccessLists(acl1),
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
		accessLists: mockAccessLists(acl),
		members:     mockAccessListMembers(t, member),
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
			accessLists: mockAccessLists(rootList, middleList, leafList),
			members:     mockAccessListMembers(t, middleInRoot, leafInMiddle, userMember),
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
			accessLists: mockAccessLists(rootList, middleList, leafList),
			members:     mockAccessListMembers(t, middleInRoot, leafInMiddle, userMember),
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
		t.Skip("cyclic graph not supported yet")
		firstList := newAccessList(t, "first", clock)
		secondList := newAccessList(t, "second", clock)
		thirdList := newAccessList(t, "third", clock)

		firstArc := newAccessListMember(t, firstList.GetName(), secondList.GetName(), accesslist.MembershipKindList, clock)
		secondArc := newAccessListMember(t, secondList.GetName(), thirdList.GetName(), accesslist.MembershipKindList, clock)
		thirdArc := newAccessListMember(t, thirdList.GetName(), firstList.GetName(), accesslist.MembershipKindList, clock)

		aclGetter := &mockAccessListAndMembersGetter{
			accessLists: mockAccessLists(firstList, secondList, thirdList),
			members:     mockAccessListMembers(t, firstArc, secondArc, thirdArc),
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
		t.Skip("cyclic graph not supported yet")
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
			accessLists: mockAccessLists(firstList, secondList, thirdList),
			members:     mockAccessListMembers(t, firstArc, secondArc, thirdArc, userMembership),
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
		members: mockAccessListMembers(t, acl2m1, acl3m1),
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
	// and 2 scoped roles for owners
	aclroot.Spec.OwnerGrants = accesslist.Grants{
		Traits: map[string][]string{
			"root-owner-trait": {"root-owner-value"},
		},
		Roles: []string{"root-owner-role"},
		ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "root-scoped-role",
				Scope: "/root/bb",
			},
			{
				Role:  "root-scoped-role",
				Scope: "/root/aa",
			},
		},
	}

	// acl1 has a trait for members - "1-member-trait", and a role for members - "1-member-role"
	// and 2 scoped roles for members
	acl1.Spec.Grants = accesslist.Grants{
		Traits: map[string][]string{
			"1-member-trait": {"1-member-value"},
		},
		Roles: []string{"1-member-role"},
		ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "1-scoped-role",
				Scope: "/1/bb",
			},
			{
				Role:  "1-scoped-role",
				Scope: "/1/aa",
			},
		},
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
		accessLists: mockAccessLists(aclroot, acl1, acl2),
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
		ScopedRoles: []accesslist.ScopedRoleGrant{
			{
				Role:  "1-scoped-role",
				Scope: "/1/aa",
			},
			{
				Role:  "1-scoped-role",
				Scope: "/1/bb",
			},
			{
				Role:  "root-scoped-role",
				Scope: "/root/aa",
			},
			{
				Role:  "root-scoped-role",
				Scope: "/root/bb",
			},
		},
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

			listsByID := make(map[NormalizedSQN]*accesslist.AccessList, len(tc.lists))
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
				listsByID[ScopeQualifiedName(acl)] = acl
				listsByName[ls.name] = acl
			}

			members := make(map[NormalizedSQN][]*accesslist.AccessListMember)
			for _, ls := range tc.lists {
				child := listsByName[ls.name]
				for _, parentName := range ls.memberOf {
					parent := listsByName[parentName]
					child.Status.MemberOf = append(child.Status.MemberOf, parent.GetName())
					members[ScopeQualifiedName(parent)] = append(members[ScopeQualifiedName(parent)],
						newAccessListMember(t, parent.GetName(), child.GetName(), accesslist.MembershipKindList, clock))
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
		accessLists: mockAccessLists(a, b, c),
		members: mockAccessListMembers(t,
			newAccessListMember(t, "A", "userA", accesslist.MembershipKindUser, clock),
			newAccessListMember(t, "A", "B", accesslist.MembershipKindList, clock),
			newAccessListMember(t, "B", "userB", accesslist.MembershipKindUser, clock),
			newAccessListMember(t, "B", "C", accesslist.MembershipKindList, clock),
			newAccessListMember(t, "C", "userC", accesslist.MembershipKindUser, clock),
			newAccessListMember(t, "C", "B", accesslist.MembershipKindList, clock), // cycle back
		),
	}

	members, err := GetMembersForV2(ctx, ScopeQualifiedName(a), getter)
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

func generateNestedALs(level, directMembers int, rootListName, userName string) (map[NormalizedSQN]*accesslist.AccessList, map[NormalizedSQN][]*accesslist.AccessListMember) {
	accesslists := []*accesslist.AccessList{generateAccessList(rootListName)}
	members := make(map[NormalizedSQN][]*accesslist.AccessListMember)

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

		members[NormalizedSQN{Name: parentName}] = listMembers
	}

	alMap := make(map[NormalizedSQN]*accesslist.AccessList)
	for _, al := range accesslists {
		alMap[ScopeQualifiedName(al)] = al
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
	const mainAccessListName = "main-al"
	const testUserName = "test-user"

	lockGetter := &mockLocksGetter{}
	clock := clockwork.NewFakeClock()

	b.Run("no accessPaths", func(b *testing.B) {
		accessList := generateAccessList(mainAccessListName)
		mock := &mockAccessListAndMembersGetter{
			accessLists: mockAccessLists(accessList),
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				accessList,
				mock,
				lockGetter,
				clock)
			if !trace.IsAccessDenied(err) {
				b.Fatal(err)
			}
		}
	})

	b.Run("single-page direct member", func(b *testing.B) {
		accessList := generateAccessList(mainAccessListName)
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
			accessLists: mockAccessLists(accessList),
			members:     mockAccessListMembers(b, members...),
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				accessList,
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple-pages direct member", func(b *testing.B) {
		accessList := generateAccessList(mainAccessListName)
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
			accessLists: mockAccessLists(accessList),
			members:     mockAccessListMembers(b, members...),
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				accessList,
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
