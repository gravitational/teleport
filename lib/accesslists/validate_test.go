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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestAccessListHierarchyCircularRefsCheck(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	acl1 := newAccessList(t, "1", clock)
	acl2 := newAccessList(t, "2", clock)
	acl3 := newAccessList(t, "3", clock)

	// acl1 -> acl2 -> acl3
	acl1m1 := newAccessListMember(t, acl1.GetName(), acl2.GetName(), accesslist.MembershipKindList, clock)
	acl2.Status.MemberOf = append(acl2.Status.MemberOf, acl1.GetName())
	acl2m1 := newAccessListMember(t, acl2.GetName(), acl3.GetName(), accesslist.MembershipKindList, clock)
	acl3.Status.MemberOf = append(acl3.Status.MemberOf, acl2.GetName())

	// acl3 -> acl1
	acl3m1 := newAccessListMember(t, acl3.GetName(), acl1.GetName(), accesslist.MembershipKindList, clock)

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl1.GetName(): {acl1m1},
			acl2.GetName(): {acl2m1},
			acl3.GetName(): {},
		},
		accessLists: map[string]*accesslist.AccessList{
			acl1.GetName(): acl1,
			acl2.GetName(): acl2,
			acl3.GetName(): acl3,
		},
	}

	// Circular references should not be allowed.
	err := ValidateAccessListMember(ctx, acl3, acl3m1, accessListAndMembersGetter)
	//err = hierarchy.ValidateAccessListMember(acl3.GetName(), acl3m1)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because '%s' is already included as a Member or Owner in '%s'", acl1.Spec.Title, acl3.Spec.Title, acl3.Spec.Title, acl1.Spec.Title))

	// By removing acl3 as a member of acl2, the relationship should be valid.
	accessListAndMembersGetter.members[acl2.GetName()] = []*accesslist.AccessListMember{}
	accessListAndMembersGetter.accessLists[acl3.GetName()].Status.MemberOf = []string{}
	err = ValidateAccessListMember(ctx, acl3, acl3m1, accessListAndMembersGetter)
	require.NoError(t, err)

	// Circular references with Ownership should also be disallowed.
	acl4 := newAccessList(t, "4", clock)
	acl5 := newAccessList(t, "5", clock)

	// acl4 includes acl5 as a Member
	acl4m1 := newAccessListMember(t, acl4.GetName(), acl5.GetName(), accesslist.MembershipKindList, clock)
	acl5.Status.MemberOf = append(acl5.Status.MemberOf, acl4.GetName())

	// acl5 includes acl4 as an Owner.
	acl5.Spec.Owners = append(acl5.Spec.Owners, accesslist.Owner{
		Name:           acl4.GetName(),
		Description:    "asdf",
		MembershipKind: accesslist.MembershipKindList,
	})
	acl4.Status.OwnerOf = append(acl4.Status.OwnerOf, acl5.GetName())

	accessListAndMembersGetter = &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			acl4.GetName(): {acl4m1},
			acl5.GetName(): {},
		},
		accessLists: map[string]*accesslist.AccessList{
			acl4.GetName(): acl4,
			acl5.GetName(): acl5,
		},
	}

	err = ValidateAccessListWithMembers(ctx, nil, acl5, []*accesslist.AccessListMember{acl4m1}, accessListAndMembersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as an Owner of '%s' because '%s' is already included as a Member or Owner in '%s'", acl4.Spec.Title, acl5.Spec.Title, acl5.Spec.Title, acl4.Spec.Title))
}

func TestAccessListHierarchyDepthCheck(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	numAcls := accesslist.MaxAllowedDepth + 2 // Extra 2 to test exceeding the max depth

	acls := make([]*accesslist.AccessList, numAcls)
	for i := range numAcls {
		acls[i] = newAccessList(t, fmt.Sprintf("acl%d", i+1), clock)
	}

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members:     make(map[string][]*accesslist.AccessListMember),
		accessLists: make(map[string]*accesslist.AccessList),
	}

	// Create members up to MaxAllowedDepth
	for i := range accesslist.MaxAllowedDepth {
		member := newAccessListMember(t, acls[i].GetName(), acls[i+1].GetName(), accesslist.MembershipKindList, clock)
		acls[i+1].Status.MemberOf = append(acls[i+1].Status.MemberOf, acls[i].GetName())
		accessListAndMembersGetter.members[acls[i].GetName()] = []*accesslist.AccessListMember{member}
		accessListAndMembersGetter.accessLists[acls[i].GetName()] = acls[i]
	}
	// Set remaining Access Lists' members to empty slices
	for i := accesslist.MaxAllowedDepth; i < numAcls; i++ {
		accessListAndMembersGetter.members[acls[i].GetName()] = []*accesslist.AccessListMember{}
		accessListAndMembersGetter.accessLists[acls[i].GetName()] = acls[i]
	}

	// Should be valid with existing member < MaxAllowedDepth
	err := ValidateAccessListMember(ctx, acls[accesslist.MaxAllowedDepth-1], accessListAndMembersGetter.members[acls[accesslist.MaxAllowedDepth-1].GetName()][0], accessListAndMembersGetter)
	require.NoError(t, err)

	// Now, attempt to add a member that increases the depth beyond MaxAllowedDepth
	extraMember := newAccessListMember(
		t,
		acls[accesslist.MaxAllowedDepth].GetName(),
		acls[accesslist.MaxAllowedDepth+1].GetName(),
		accesslist.MembershipKindList,
		clock,
	)

	// Validate adding this member should fail due to exceeding max depth
	err = ValidateAccessListMember(ctx, acls[accesslist.MaxAllowedDepth], extraMember, accessListAndMembersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", acls[accesslist.MaxAllowedDepth+1].Spec.Title, acls[accesslist.MaxAllowedDepth].Spec.Title, accesslist.MaxAllowedDepth))
}

func TestAccessListValidateWithMembers_basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	t.Run("type is validated", func(t *testing.T) {
		accessList := newAccessList(t, "test_access_list", clock)
		accessList.Spec.Type = "test_unknown_type"

		err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
		require.Error(t, err)
		require.ErrorContains(t, err, `unknown access list type "test_unknown_type"`)
	})

	for _, typ := range accesslist.AllTypes {
		t.Run("for type: "+string(typ), func(t *testing.T) {
			if typ == accesslist.DeprecatedDynamic {
				t.Skip("DeprecatedDynamic type can be skipped because it's defaulted to Default in CheckAndSetDefaults")
			}

			t.Run("valid", func(t *testing.T) {
				accessList := newAccessList(t, "test_access_list", clock)

				err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
				require.NoError(t, err)
			})

			t.Run("owners are required", func(t *testing.T) {
				accessList := newAccessList(t, "test_access_list", clock)
				accessList.Spec.Type = typ
				accessList.Spec.Owners = []accesslist.Owner{}

				err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
				require.Error(t, err)
				require.ErrorContains(t, err, "owners")
			})

			if typ.IsReviewable() {
				t.Run("audit is required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit = accesslist.Audit{}

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.Error(t, err)
					require.ErrorContains(t, err, "audit")
				})
				t.Run("audit.recurrence.frequency is required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.Frequency = 0

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.Error(t, err)
					require.ErrorContains(t, err, "audit recurrence frequency")
				})
				t.Run("audit.recurrence.day_of_month is required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.DayOfMonth = 0

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.Error(t, err)
					require.ErrorContains(t, err, "audit recurrence day of month")
				})
				t.Run("audit.recurrence.next_audit_date is required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit.NextAuditDate = time.Time{}

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.Error(t, err)
					require.ErrorContains(t, err, "next audit date")
				})
				t.Run("audit.notifications.start is required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Notifications.Start = 0

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.Error(t, err)
					require.ErrorContains(t, err, "audit notifications start")
				})
			} else {
				t.Run("audit is not required", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit = accesslist.Audit{}

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.NoError(t, err)
				})
				t.Run("audit can be set", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.NoError(t, err)
				})
				t.Run("audit can be partially set", func(t *testing.T) {
					accessList := newAccessList(t, "test_access_list", clock)
					accessList.Spec.Type = typ
					accessList.Spec.Audit = accesslist.Audit{}
					accessList.Spec.Audit.Recurrence.DayOfMonth = accesslist.FifteenthDayOfMonth
					accessList.Spec.Audit.Notifications.Start = 3 * time.Hour

					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, &mockAccessListAndMembersGetter{})
					require.NoError(t, err)
				})
			}
		})
	}

}

func TestAccessListValidateWithMembers_members(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// We're creating a hierarchy with a depth of 10, and then trying to add it as a Member of a 'root' Access List. This should fail.
	rootAcl := newAccessList(t, "root", clock)
	nestedAcls := make([]*accesslist.AccessList, 0, accesslist.MaxAllowedDepth)
	for i := range accesslist.MaxAllowedDepth + 1 {
		acl := newAccessList(t, fmt.Sprintf("acl-%d", i), clock)
		nestedAcls = append(nestedAcls, acl)
	}
	rootAclMember := newAccessListMember(t, rootAcl.GetName(), nestedAcls[0].GetName(), accesslist.MembershipKindList, clock)
	members := make([]*accesslist.AccessListMember, 0, accesslist.MaxAllowedDepth-1)
	for i := range accesslist.MaxAllowedDepth {
		member := newAccessListMember(t, nestedAcls[i].GetName(), nestedAcls[i+1].GetName(), accesslist.MembershipKindList, clock)
		nestedAcls[i+1].Status.MemberOf = append(nestedAcls[i+1].Status.MemberOf, nestedAcls[i].GetName())
		members = append(members, member)
	}

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{
			rootAcl.GetName(): {},
		},
		accessLists: map[string]*accesslist.AccessList{
			rootAcl.GetName(): rootAcl,
		},
	}
	for i := range accesslist.MaxAllowedDepth + 1 {
		if i < accesslist.MaxAllowedDepth {
			accessListAndMembersGetter.members[nestedAcls[i].GetName()] = []*accesslist.AccessListMember{members[i]}
		}
		accessListAndMembersGetter.accessLists[nestedAcls[i].GetName()] = nestedAcls[i]
	}

	// Should validate successfully, as acl-0 -> acl-10 is a valid hierarchy of depth 10.
	err := ValidateAccessListWithMembers(ctx, nil, rootAcl, []*accesslist.AccessListMember{}, accessListAndMembersGetter)
	require.NoError(t, err)
	err = ValidateAccessListWithMembers(ctx, nil, nestedAcls[0], []*accesslist.AccessListMember{accessListAndMembersGetter.members[nestedAcls[0].GetName()][0]}, accessListAndMembersGetter)
	require.NoError(t, err)

	// Calling `ValidateAccessListWithMembers`, with `rootAclm1`, should fail, as it would exceed the maximum nesting depth.
	err = ValidateAccessListWithMembers(ctx, nil, rootAcl, []*accesslist.AccessListMember{rootAclMember}, accessListAndMembersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", nestedAcls[0].Spec.Title, rootAcl.Spec.Title, accesslist.MaxAllowedDepth))

	const Length = accesslist.MaxAllowedDepth/2 + 1

	// Next, we're creating two separate hierarchies, each with a depth of `MaxAllowedDepth/2`. When testing the validation, we'll try to connect the two hierarchies, which should fail.
	nestedAcls1 := make([]*accesslist.AccessList, 0, Length)
	for i := 0; i <= Length; i++ {
		acl := newAccessList(t, fmt.Sprintf("acl1-%d", i), clock)
		nestedAcls1 = append(nestedAcls1, acl)
	}

	// Create the second hierarchy.
	nestedAcls2 := make([]*accesslist.AccessList, 0, Length)
	for i := 0; i <= Length; i++ {
		acl := newAccessList(t, fmt.Sprintf("acl2-%d", i), clock)
		nestedAcls2 = append(nestedAcls2, acl)
	}

	accessListAndMembersGetter = &mockAccessListAndMembersGetter{
		members:     map[string][]*accesslist.AccessListMember{},
		accessLists: map[string]*accesslist.AccessList{},
	}

	// Create the members for the first hierarchy.
	for i := range Length {
		member := newAccessListMember(t, nestedAcls1[i].GetName(), nestedAcls1[i+1].GetName(), accesslist.MembershipKindList, clock)
		nestedAcls1[i+1].Status.MemberOf = append(nestedAcls1[i+1].Status.MemberOf, nestedAcls1[i].GetName())
		accessListAndMembersGetter.members[nestedAcls1[i].GetName()] = []*accesslist.AccessListMember{member}
		accessListAndMembersGetter.accessLists[nestedAcls1[i].GetName()] = nestedAcls1[i]
	}

	// Create the members for the second hierarchy.
	for i := range Length {
		member := newAccessListMember(t, nestedAcls2[i].GetName(), nestedAcls2[i+1].GetName(), accesslist.MembershipKindList, clock)
		nestedAcls2[i+1].Status.MemberOf = append(nestedAcls2[i+1].Status.MemberOf, nestedAcls2[i].GetName())
		accessListAndMembersGetter.members[nestedAcls2[i].GetName()] = []*accesslist.AccessListMember{member}
		accessListAndMembersGetter.accessLists[nestedAcls2[i].GetName()] = nestedAcls2[i]
	}

	// For the first hierarchy
	nestedAcls1Last := nestedAcls1[len(nestedAcls1)-1]
	accessListAndMembersGetter.accessLists[nestedAcls1Last.GetName()] = nestedAcls1Last

	// For the second hierarchy
	nestedAcls2Last := nestedAcls2[len(nestedAcls2)-1]
	accessListAndMembersGetter.accessLists[nestedAcls2Last.GetName()] = nestedAcls2Last

	// Should validate successfully when adding another list, as both hierarchies are valid.
	err = ValidateAccessListWithMembers(ctx, nil, nestedAcls1Last, []*accesslist.AccessListMember{newAccessListMember(t, nestedAcls1Last.GetName(), nestedAcls2Last.GetName(), accesslist.MembershipKindList, clock)}, accessListAndMembersGetter)
	require.NoError(t, err)
	err = ValidateAccessListWithMembers(ctx, nil, nestedAcls2Last, []*accesslist.AccessListMember{newAccessListMember(t, nestedAcls2Last.GetName(), nestedAcls1Last.GetName(), accesslist.MembershipKindList, clock)}, accessListAndMembersGetter)
	require.NoError(t, err)

	// Now, we'll try to connect the two hierarchies, which should fail.
	err = ValidateAccessListWithMembers(ctx, nil, nestedAcls1Last, []*accesslist.AccessListMember{newAccessListMember(t, nestedAcls1Last.GetName(), nestedAcls2[0].GetName(), accesslist.MembershipKindList, clock)}, accessListAndMembersGetter)
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", nestedAcls2[0].Spec.Title, nestedAcls1[len(nestedAcls1)-1].Spec.Title, accesslist.MaxAllowedDepth))
}

func Test_ValidateAccessListWithMembers_audit(t *testing.T) {
	ctx := context.Background()

	accessListName := "test_list"
	var accessList *accesslist.AccessList

	accessListAndMembersGetter := &mockAccessListAndMembersGetter{
		members: map[string][]*accesslist.AccessListMember{},
		accessLists: map[string]*accesslist.AccessList{
			accessListName: accessList,
		},
	}

	t.Run("audit frequency", func(t *testing.T) {
		accessList = newAccessList(t, accessListName, clockwork.NewFakeClockAt(time.Now()))
		t.Run("must be non-zero for reviewable access lists", func(t *testing.T) {
			for _, typ := range []accesslist.Type{accesslist.Default} {
				t.Run(string(typ), func(t *testing.T) {
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.Frequency = 0
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.ErrorContains(t, err, "frequency")
				})
			}
		})
		t.Run("can be zero for non-reviewable access lists", func(t *testing.T) {
			for _, typ := range []accesslist.Type{accesslist.SCIM, accesslist.Static} {
				t.Run(string(typ), func(t *testing.T) {
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.Frequency = 0
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.NoError(t, err)
				})
			}
		})

		t.Run("if set must be a valid value for all access list types", func(t *testing.T) {
			for _, typ := range accesslist.AllTypes {
				t.Run(string(typ), func(t *testing.T) {
					if typ == accesslist.DeprecatedDynamic {
						t.Skip("deprecated dynamic type is not handled here as it's supposed to be changed in CheckAndSetDefaults; see [validateType]")
					}
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.Frequency = 399
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.ErrorContains(t, err, "frequency")
				})
			}
		})
	})

	t.Run("audit day_of_month", func(t *testing.T) {
		accessList = newAccessList(t, accessListName, clockwork.NewFakeClockAt(time.Now()))
		t.Run("must be non-zero for reviewable access lists", func(t *testing.T) {
			for _, typ := range []accesslist.Type{accesslist.Default} {
				t.Run(string(typ), func(t *testing.T) {
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.DayOfMonth = 0
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.ErrorContains(t, err, "day of month")
				})
			}
		})
		t.Run("can be zero for non-reviewable access lists", func(t *testing.T) {
			for _, typ := range []accesslist.Type{accesslist.SCIM, accesslist.Static} {
				t.Run(string(typ), func(t *testing.T) {
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.DayOfMonth = 0
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.NoError(t, err)
				})
			}
		})

		t.Run("if set must be a valid value for all access list types", func(t *testing.T) {
			for _, typ := range accesslist.AllTypes {
				t.Run(string(typ), func(t *testing.T) {
					if typ == accesslist.DeprecatedDynamic {
						t.Skip("deprecated dynamic type is not handled here as it's supposed to be changed in CheckAndSetDefaults; see [validateType]")
					}
					accessList.Spec.Type = typ
					accessList.Spec.Audit.Recurrence.DayOfMonth = 40
					err := ValidateAccessListWithMembers(ctx, nil, accessList, nil, accessListAndMembersGetter)
					require.ErrorContains(t, err, "day of month")
				})
			}
		})
	})
}
