// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	testUser   = "test-user"
	ownerUser  = "owner-user"
	ownerUser2 = "owner-user2"
	member1    = "member1"
	member2    = "member2"
	member3    = "member3"
)

// cmpOpts are general cmpOpts for all comparisons.
var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	cmpopts.SortSlices(func(a, b *accesslist.AccessList) bool {
		return a.GetName() < b.GetName()
	}),
	cmpopts.SortSlices(func(a, b *accesslist.Review) bool {
		return a.GetName() < b.GetName()
	}),
}

func TestService_GetAccessLists(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	// a3 will have different ownership requirements.
	a3 := newAccessList(t, "3", clock)
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, clock)

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, ctx, svc, emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// owner should only see a1 and a2
	getResp, err = svc.GetAccessLists(ownerCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	getResp, err = svc.GetAccessLists(memberCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))
}

func TestService_ListAccessLists(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	accessLists := listAccessLists(ctx, t, svc, 1)
	require.Empty(t, accessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)
	a4 := newAccessList(t, "4", clock)
	a5 := newAccessList(t, "5", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, clock)
	a5m1 := newAccessListMember(t, a5.GetName(), member1, clock)
	a5m2 := newAccessListMember(t, a5.GetName(), member2, clock)

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, ctx, svc, emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4, a5}, []*accesslist.AccessListMember{
			a1m1, a1m2, a2m1, a3m1, a3m2, a4m1, a4m2, a5m1, a5m2,
		})

	accessLists = listAccessLists(ctx, t, svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5}, accessLists, cmpOpts...))

	// owner should only see a1, a2, a4, a5
	accessLists = listAccessLists(ownerCtx, t, svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	accessLists = listAccessLists(memberCtx, t, svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	// Use the page size defaults
	accessLists = listAccessLists(memberCtx, t, svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))
}

func listAccessLists(ctx context.Context, t *testing.T, svc *Service, pageSize int) []*accesslist.AccessList {
	t.Helper()

	var nextToken string
	var accessLists []*accesslist.AccessList
	for {
		resp, err := svc.ListAccessLists(ctx, &accesslistv1.ListAccessListsRequest{
			PageSize:  int32(pageSize),
			NextToken: nextToken,
		})
		require.NoError(t, err)

		for _, accessList := range resp.AccessLists {
			accessLists = append(accessLists, mustFromProto(t, accessList))
		}

		nextToken = resp.NextToken
		if nextToken == "" {
			break
		}
	}

	return accessLists
}

func TestService_UpsertAccessList(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, ownerCtx, svc, clock, emitter, usageEvents := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a1.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// User tries to create a new access list that they own. Shouldn't work.
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListCreateFailureCode, emitter, func(event *apievents.AccessListCreate) {
		require.False(t, event.Success)
	})

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a2.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// Owner should be able to modify the audit.
	a2.Spec.Audit.NextAuditDate = clock.Now().AddDate(100, 0, 0)
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListUpdateSuccessCode, emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
	})

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	// Owner should be able to modify the membership requires.
	a2.Spec.MembershipRequires.Roles = append(a2.Spec.MembershipRequires.Roles, "new")
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListUpdateSuccessCode, emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
	})

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	// Owner should not be able to modify anything else. We'll test by changing ownership roles.
	a2.Spec.OwnershipRequires.Roles = append(a2.Spec.OwnershipRequires.Roles, "new")
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListUpdateFailureCode, emitter, func(event *apievents.AccessListUpdate) {
		require.False(t, event.Success)
	})
}

func TestService_GetAccessList(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	eligibleOwnersWithStatus := []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "owner user",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		},
		{
			Name:             ownerUser2,
			Description:      "owner user 2",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		},
	}

	a1 := newAccessList(t, "1", clock)
	a1.Spec.Owners = eligibleOwnersWithStatus

	a2 := newAccessList(t, "2", clock)
	a2.Spec.Owners = eligibleOwnersWithStatus

	// a3 will have different ownership requirements.
	a3 := newAccessList(t, "3", clock)
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}
	a3.Spec.Owners = []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "owner user",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)],
		},
	}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	// member2 can't see a2
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	get, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	_, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	get, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	// owner can't see a3
	_, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.True(t, trace.IsAccessDenied(err))
}

func TestService_GetAccessListsToReview(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	resp, err := svc.GetAccessListsToReview(ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)
	a4 := newAccessList(t, "4", clock)
	a5 := newAccessList(t, "5", clock)

	a1.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	a2.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a3.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a4.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a5.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil, []*accesslist.AccessList{a1, a2, a3, a4, a5}, nil)

	svc.clock = clockwork.NewFakeClockAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	resp, err = svc.GetAccessListsToReview(ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.AccessLists)

	svc.clock = clockwork.NewFakeClockAt(time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC))

	resp, err = svc.GetAccessListsToReview(ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))

	svc.clock = clockwork.NewFakeClockAt(time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC))

	resp, err = svc.GetAccessListsToReview(ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))

	svc.clock = clockwork.NewFakeClockAt(time.Date(2024, 2, 16, 0, 0, 0, 0, time.UTC))

	resp, err = svc.GetAccessListsToReview(ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))
}

func TestService_UpsertAndGetAccessList_OwnersIneligibleReason(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock, _, _ := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	// Create an access list, with varying eligbility for owners.
	a1 := newAccessList(t, "1", clock)
	a1.Spec.Owners = []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "OK existing user",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)],
		},
		{
			Name:             member1,
			Description:      "NOK ownermemship_requires does not match",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)],
		},
	}

	// Test that owner's ineligible status got stripped before upsertion.
	createdAccessList, err := svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "OK existing user",
			IneligibleStatus: "",
		},
		{
			Name:             member1,
			Description:      "NOK ownermemship_requires does not match",
			IneligibleStatus: "",
		},
	}, mustFromProto(t, createdAccessList).GetOwners(), cmpOpts...))

	// Check retrieved access list owners has determined the ineligible status field.
	getAccessList, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1.Spec.Owners, mustFromProto(t, getAccessList, conv.WithOwnersIneligibleStatusField(getAccessList.Spec.Owners)).GetOwners(), cmpOpts...))
}

func TestService_UpsertAndGetAccessList_MembersIneligibleReason(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock, _, _ := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	// Create an access list.
	a1 := newAccessList(t, "1", clock)
	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	// Create some members with varying eligiblity.
	member_expired, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: member1,
		},
		accesslist.AccessListMemberSpec{
			AccessList:       a1.GetName(),
			Name:             member1,
			Joined:           clock.Now().UTC(),
			Expires:          clock.Now().UTC().Add(-24 * time.Hour),
			Reason:           "expired",
			AddedBy:          testUser,
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED)],
		},
	)
	require.NoError(t, err)

	membersToCreate := []*accesslist.AccessListMember{
		// NOK member is expired
		member_expired,
		// OK member
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), member2, clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)]),
		// NOK membership_requires does not match
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), ownerUser, clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)]),
	}

	// Test that member's ineligible status got stripped before upsertion.
	for _, member := range membersToCreate {
		upsertedMember, err := svc.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(member)})
		require.NoError(t, err)
		require.Empty(t, upsertedMember.Spec.IneligibleStatus)
	}

	// Check retrieved members list has determined the ineligible status field.
	getMembers, err := svc.ListAccessListMembers(ctx, &accesslistv1.ListAccessListMembersRequest{PageSize: 0, AccessList: a1.GetName()})
	require.NoError(t, err)

	var members []*accesslist.AccessListMember
	for _, member := range getMembers.Members {
		members = append(members, mustFromMemberProto(t, member, conv.WithMemberIneligibleStatusField(member)))
	}
	require.Empty(t, cmp.Diff(membersToCreate, members, cmpOpts...))
}

func TestService_DeleteAccessList(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, _, svc, clock, emitter, usageEvents := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, usageEvents, []*accesslist.AccessList{a1}, nil)

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a1.GetName(), event.AccessListDelete.Metadata.Id)
	})

	_, err = svc.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.True(t, trace.IsNotFound(err))
	expectEvent(t, events.AccessListDeleteFailureCode, emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})
}

type usageEventsClient struct {
	events []*usageeventsv1.UsageEventOneOf
}

func (u *usageEventsClient) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	u.events = append(u.events, req.Event)
	return nil
}

type fakeAuth struct{}

func (a *fakeAuth) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	return []types.AccessRequest{}, nil
}

func (a *fakeAuth) SubmitAccessReview(ctx context.Context, req types.AccessReviewSubmission) (types.AccessRequest, error) {
	return &types.AccessRequestV3{}, nil
}

func (a *fakeAuth) GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return &types.AccessRequestAllowedPromotions{}, nil
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func (c *testClient) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, passwordless bool) (*types.MFADevice, string, error) {
	return nil, "", nil
}

func initSvc(t *testing.T) (userContext context.Context, ownerContext context.Context, svc *Service, clock clockwork.Clock, emitter *eventstest.ChannelEmitter, usageEvents *usageEventsClient) {
	ctx := context.Background()
	clock = clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewIdentityService(backend)

	require.NoError(t, clusterConfigSvc.SetAuthPreference(ctx, types.DefaultAuthPreference()))
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	require.NoError(t, clusterConfigSvc.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, clusterConfigSvc.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
	emitter = eventstest.NewChannelEmitter(10)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		services.Identity
	}
	clt := client{
		Access:   accessService,
		Identity: userSvc,
	}

	role, err := auth.CreateRole(ctx, clt, "access-lists", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, clt, "mrole1", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, clt, "mrole2", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, clt, "orole1", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, clt, "orole2", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := types.NewUser(testUser)
	require.NoError(t, err)
	user.AddRole(role.GetName())

	ownerRoles := []string{"orole1", "orole2"}
	ownerTraits := map[string][]string{
		"otrait1": {"ovalue1", "ovalue2"},
		"otrait2": {"ovalue3", "ovalue4"},
	}
	owner, err := types.NewUser(ownerUser)
	require.NoError(t, err)
	owner.SetRoles(ownerRoles)
	owner.SetTraits(ownerTraits)

	owner2, err := types.NewUser(ownerUser2)
	require.NoError(t, err)
	owner2.SetRoles(ownerRoles)
	owner2.SetTraits(ownerTraits)

	user, err = userSvc.CreateUser(ctx, user)
	require.NoError(t, err)
	owner, err = userSvc.CreateUser(ctx, owner)
	require.NoError(t, err)
	_, err = userSvc.CreateUser(ctx, owner2)
	require.NoError(t, err)

	storage, err := local.NewAccessListService(backend, clock)
	require.NoError(t, err)

	locks := local.NewAccessService(backend)

	usageEvents = &usageEventsClient{}
	svc, err = NewService(ServiceConfig{
		Authorizer:          authorizer,
		AccessLists:         storage,
		LockGetter:          locks,
		AccessListReviews:   storage,
		Emitter:             emitter,
		UsageEvents:         usageEvents,
		Clock:               clock,
		CachedUsersServices: userSvc,
		AuthServer:          &fakeAuth{},
	})
	require.NoError(t, err)

	// Force pagination for testing purposes.
	svc.userPageSize = 1

	memberRoles := []string{"mrole1", "mrole2"}
	memberTraits := map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	}

	member1, err := types.NewUser(member1)
	require.NoError(t, err)
	member1.SetRoles(memberRoles)
	member1.SetTraits(memberTraits)
	_, err = userSvc.CreateUser(ctx, member1)
	require.NoError(t, err)

	member2, err := types.NewUser(member2)
	require.NoError(t, err)
	member2.SetRoles(memberRoles)
	member2.SetTraits(memberTraits)
	_, err = userSvc.CreateUser(ctx, member2)
	require.NoError(t, err)

	member3, err := types.NewUser(member3)
	require.NoError(t, err)
	member3.SetRoles(memberRoles)
	member3.SetTraits(memberTraits)
	_, err = userSvc.CreateUser(ctx, member3)
	require.NoError(t, err)

	return genUserContext(ctx, user.GetName(), []string{role.GetName()}, nil),
		genUserContext(ctx, owner.GetName(), ownerRoles, ownerTraits), svc, clock, emitter, usageEvents
}

func TestService_ListAccessListMembers(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2})

	// Admin should be able to list everything
	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m1, a1m2}, members, cmpOpts...))

	// owner should be able to see members for a2
	members = listAllAccessListMembers(ownerCtx, t, svc, a2.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a2m1, a2m2}, members, cmpOpts...))

	// owner should not be able to see members for a3
	_, err := svc.ListAccessListMembers(ownerCtx, &accesslistv1.ListAccessListMembersRequest{
		PageSize:   0,
		PageToken:  "",
		AccessList: a3.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))
}

func TestService_GetAccessListMember(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil,
		[]*accesslist.AccessList{a1}, []*accesslist.AccessListMember{a1m1, a1m2})

	// Admin should be able to get members
	member, err := svc.GetAccessListMember(ctx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m1, mustFromMemberProto(t, member), cmpOpts...))

	// owner should be able to see members for a1
	member, err = svc.GetAccessListMember(ownerCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m2, mustFromMemberProto(t, member), cmpOpts...))
}

func TestService_UpsertAccessListMember(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, ownerCtx, svc, clock, emitter, usageEvents := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	_, err := svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a1.GetName(), event.AccessListCreate.Metadata.Id)
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)

	require.Equal(t, testUser, a1m1.Spec.AddedBy)

	got, err := svc.UpsertAccessListMember(ownerCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberCreate.Metadata.Id)
	})

	// Added by should be overridden from "test-user" to "owner-user"
	want := a1m1
	want.Spec.AddedBy = ownerUser

	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	got, err = svc.GetAccessListMember(ctx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	// Update from admin should still retain the old added by, reason, and joined
	oldJoined := a1m1.Spec.Joined
	want.Spec.Reason = "some new reason"
	want.Spec.Joined = clock.Now().Add(time.Hour * 24)
	_, err = svc.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, emitter, func(event *apievents.AccessListMemberUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberUpdate.Metadata.Id)
	})

	want.Spec.Joined = oldJoined
	got, err = svc.GetAccessListMember(ctx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))
}

func TestService_DeleteAccessListMember(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, ownerCtx, svc, clock, emitter, usageEvents := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, usageEvents,
		[]*accesslist.AccessList{a1}, []*accesslist.AccessListMember{a1m1, a1m2})

	// Admin should be able to delete members
	_, err := svc.DeleteAccessListMember(ctx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})

	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m2}, members, cmpOpts...))

	// owner should be able to delete members
	_, err = svc.DeleteAccessListMember(ownerCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})
	members = listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, members)
}

func TestService_DeleteAllAccessListMembersForAccessList(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, ownerCtx, svc, clock, emitter, usageEvents := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, usageEvents,
		[]*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// Admin should be able to delete members
	_, err := svc.DeleteAllAccessListMembersForAccessList(ctx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, members)

	// owner should be able to delete members
	_, err = svc.DeleteAllAccessListMembersForAccessList(ownerCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a2.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members = listAllAccessListMembers(ctx, t, svc, a2.GetName(), 1)
	require.Empty(t, members)
}

func TestService_UpsertAccessListWithMembers(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx, _, svc, clock, emitter, usageEvents := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member3, clock)
	a2m2 := newAccessListMemberWithIneligibleReason(t, a2.GetName(), "user4", clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST)])

	createAccessListsAndMembers(t, ctx, svc, emitter, usageEvents, []*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1})

	// Sanity check
	membersA1 := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 2)
	require.Len(t, membersA1, 2)

	membersA2 := listAllAccessListMembers(ctx, t, svc, a2.GetName(), 2)
	require.Len(t, membersA2, 1)

	t.Run("remove one member", func(t *testing.T) {
		_, err := svc.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a1),
			Members:    conv.ToMembersProto([]*accesslist.AccessListMember{a1m1}),
		})
		require.NoError(t, err)
		expectEvent(t, events.AccessListUpdateSuccessCode, emitter, func(event *apievents.AccessListUpdate) {
			require.True(t, event.Success)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
			require.Equal(t, a1.GetName(), event.AccessListUpdate.Metadata.Id)
		})

		expectEvent(t, events.AccessListMemberUpdateSuccessCode, emitter, func(event *apievents.AccessListMemberUpdate) {
			require.True(t, event.Success)
			require.Len(t, event.AccessListMemberMetadata.Members, 1)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
			require.Equal(t, a1.GetName(), event.AccessListMemberUpdate.Metadata.Id)
		})

		expectEvent(t, events.AccessListMemberDeleteSuccessCode, emitter, func(event *apievents.AccessListMemberDelete) {
			require.True(t, event.Success)
			require.Len(t, event.AccessListMemberMetadata.Members, 1)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
			require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
		})

		// One member should have been deleted
		membersA1 = listAllAccessListMembers(ctx, t, svc, a1.GetName(), 2)
		require.Len(t, membersA1, 1)
	})

	t.Run("add one member", func(t *testing.T) {
		// Add one member to a2
		_, err := svc.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a2),
			Members:    conv.ToMembersProto([]*accesslist.AccessListMember{a2m1, a2m2}),
		})
		require.NoError(t, err)
		expectEvent(t, events.AccessListUpdateSuccessCode, emitter, func(event *apievents.AccessListUpdate) {
			require.True(t, event.Success)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
			require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
		})

		expectEvent(t, events.AccessListMemberCreateSuccessCode, emitter, func(event *apievents.AccessListMemberCreate) {
			require.True(t, event.Success)
			require.Len(t, event.AccessListMemberMetadata.Members, 1)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
			require.Equal(t, a2.GetName(), event.AccessListMemberCreate.Metadata.Id)
		})

		expectEvent(t, events.AccessListMemberUpdateSuccessCode, emitter, func(event *apievents.AccessListMemberUpdate) {
			require.True(t, event.Success)
			require.Len(t, event.AccessListMemberMetadata.Members, 1)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
			require.Equal(t, a2.GetName(), event.AccessListMemberUpdate.Metadata.Id)
		})

		// One member should have been added
		membersA2 = listAllAccessListMembers(ctx, t, svc, a2.GetName(), 2)
		require.Len(t, membersA2, 2)
	})

	t.Run("remove all members", func(t *testing.T) {
		// If not members are provided all members should be deleted
		_, err := svc.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a2),
		})
		require.NoError(t, err)
		expectEvent(t, events.AccessListUpdateSuccessCode, emitter, func(event *apievents.AccessListUpdate) {
			require.True(t, event.Success)
		})
		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
			require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
		})

		expectEvent(t, events.AccessListMemberDeleteSuccessCode, emitter, func(event *apievents.AccessListMemberDelete) {
			require.True(t, event.Success)
			require.Len(t, event.AccessListMemberMetadata.Members, 2)
		})
		for i := 0; i < 2; i++ {
			expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
				require.Equal(t, a2.GetName(), event.AccessListMemberDelete.Metadata.Id)
			})
		}

		// All members should have been deleted
		membersA2 = listAllAccessListMembers(ctx, t, svc, a2.GetName(), 2)
		require.Empty(t, membersA2)
	})
}

func TestService_AuthOrIsOwner(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	nonExistentUser := genUserContext(context.Background(), "doesnt-exist", []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil, []*accesslist.AccessList{a1, a2}, nil)

	tests := []struct {
		name           string
		ctx            context.Context
		accessListName string
		wantErr        require.ErrorAssertionFunc
	}{
		{
			name:           "admin context",
			ctx:            ctx,
			accessListName: a1.GetName(),
			wantErr:        require.NoError,
		},
		{
			name:           "owner context",
			ctx:            ownerCtx,
			accessListName: a1.GetName(),
			wantErr:        require.NoError,
		},
		{
			name:           "member context",
			ctx:            memberCtx,
			accessListName: a1.GetName(),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:           "non-existent user context",
			ctx:            nonExistentUser,
			accessListName: a1.GetName(),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// This test must not be parallel to avoid testing issues with lock interaction and
			// the fake clock being used for the underlying tests.
			test.wantErr(t, svc.authOrIsOwner(test.ctx, test.accessListName, types.VerbRead))
		})
	}
}

func TestBatchAccessListMemberMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		numberOfEvents  int
		expectedBatches int
	}{
		{
			name:            "empty",
			numberOfEvents:  0,
			expectedBatches: 0,
		},
		{
			name:            "single batch",
			numberOfEvents:  50,
			expectedBatches: 1,
		},
		{
			name:            "two batches",
			numberOfEvents:  51,
			expectedBatches: 2,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var members []*apievents.AccessListMember
			if test.numberOfEvents > 0 {
				members = make([]*apievents.AccessListMember, test.numberOfEvents)
				for i := 0; i < test.numberOfEvents; i++ {
					members[i] = &apievents.AccessListMember{
						MemberName: fmt.Sprintf("%d", i),
					}
				}
			}

			batches := batchAccessListMemberMetadata("test-access-list", members)
			require.Len(t, batches, test.expectedBatches)

			for i := 0; i < test.expectedBatches; i++ {
				startIndex := i * eventMemberBatches
				endIndex := startIndex + eventMemberBatches
				if endIndex > test.numberOfEvents {
					endIndex = test.numberOfEvents
				}

				batchIndex := 0
				for j := startIndex; j < endIndex; j++ {
					require.Empty(t, cmp.Diff(members[j], batches[i].Members[batchIndex]))
					batchIndex++
				}
			}
		})
	}
}

func TestService_ListAccessListReviews(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil, []*accesslist.AccessList{a1, a2}, nil)

	require.Empty(t, listAllAccessListReviews(ctx, t, svc, a1.GetName(), 1))
	require.Empty(t, listAllAccessListReviews(ctx, t, svc, a2.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())
	review2ForA1 := newAccessListReview(t, a1.GetName())
	review3ForA1 := newAccessListReview(t, a1.GetName())
	review1ForA2 := newAccessListReview(t, a2.GetName())
	review2ForA2 := newAccessListReview(t, a2.GetName())

	review3ForA1.Spec.Changes.MembershipRequirementsChanged = &accesslist.Requires{
		Roles: []string{"new-role1", "new-role2"},
		Traits: map[string][]string{
			"new-trait1": {"value1", "value2"},
			"new-trait2": {"value1", "value2"},
		},
	}

	createReviews(ctx, t, svc, emitter, review1ForA1, review2ForA1, review3ForA1)
	createReviews(ownerCtx, t, svc, emitter, review1ForA2, review2ForA2)

	reviews := listAllAccessListReviews(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA1, review2ForA1, review3ForA1}, reviews, cmpOpts...))
	reviews = listAllAccessListReviews(ctx, t, svc, a2.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA2, review2ForA2}, reviews, cmpOpts...))
}

func TestService_DeleteAccessListReviews(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock, emitter, _ := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	createAccessListsAndMembers(t, ctx, svc, emitter, nil, []*accesslist.AccessList{a1, a2}, nil)

	require.Empty(t, listAllAccessListReviews(ctx, t, svc, a1.GetName(), 1))
	require.Empty(t, listAllAccessListReviews(ctx, t, svc, a2.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())
	review2ForA1 := newAccessListReview(t, a1.GetName())
	review3ForA1 := newAccessListReview(t, a1.GetName())
	review1ForA2 := newAccessListReview(t, a2.GetName())
	review2ForA2 := newAccessListReview(t, a2.GetName())

	createReviews(ctx, t, svc, emitter, review1ForA1, review2ForA1, review3ForA1)
	createReviews(ownerCtx, t, svc, emitter, review1ForA2, review2ForA2)

	_, err := svc.DeleteAccessListReview(ownerCtx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	_, err = svc.DeleteAccessListReview(ctx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	})
	require.NoError(t, err)

	reviews := listAllAccessListReviews(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review2ForA1, review3ForA1}, reviews, cmpOpts...))
}

func listAllAccessListMembers(ctx context.Context, t *testing.T, service *Service, accessListName string, pageSize int) []*accesslist.AccessListMember {
	t.Helper()

	var nextToken string
	var members []*accesslist.AccessListMember
	for {
		resp, err := service.ListAccessListMembers(ctx, &accesslistv1.ListAccessListMembersRequest{
			PageSize:   int32(pageSize),
			PageToken:  nextToken,
			AccessList: accessListName,
		})
		require.NoError(t, err)

		for _, member := range resp.Members {
			members = append(members, mustFromMemberProto(t, member))
		}

		nextToken = resp.NextPageToken
		if nextToken == "" {
			break
		}
	}

	return members
}

func createReviews(ctx context.Context, t *testing.T, svc *Service, emitter *eventstest.ChannelEmitter, reviews ...*accesslist.Review) {
	t.Helper()

	user, err := authz.UserFromContext(ctx)
	require.NoError(t, err)
	username := user.GetIdentity().Username

	for _, review := range reviews {
		resp, err := svc.CreateAccessListReview(ctx, &accesslistv1.CreateAccessListReviewRequest{
			Review: conv.ToReviewProto(review),
		})
		require.NoError(t, err)

		expectEvent(t, events.AccessListReviewSuccessCode, emitter, func(event *apievents.AccessListReview) {
			require.True(t, event.Success)
			require.Equal(t, username, event.UpdatedBy)

			if review.Spec.Changes.MembershipRequirementsChanged != nil {
				require.NotNil(t, event.AccessListReviewMetadata.MembershipRequirementsChanged)
				require.Equal(t, review.Spec.Changes.MembershipRequirementsChanged.Roles, event.AccessListReviewMetadata.MembershipRequirementsChanged.Roles)

				var expected map[string][]string
				var traits map[string][]string
				if len(review.Spec.Changes.MembershipRequirementsChanged.Traits) > 0 {
					expected = map[string][]string{}
					for trait, values := range review.Spec.Changes.MembershipRequirementsChanged.Traits {
						expected[trait] = sortedStrings(values)
					}

					traits = map[string][]string{}
					for trait, values := range event.MembershipRequirementsChanged.Traits {
						traits[trait] = sortedStrings(strings.Split(values, ","))
					}
				}
				require.Equal(t, expected, traits)
			} else {
				require.Nil(t, event.AccessListReviewMetadata.MembershipRequirementsChanged)
			}

			require.Equal(t, review.Spec.Changes.ReviewFrequencyChanged.String(), event.ReviewFrequencyChanged)
			require.Equal(t, review.Spec.Changes.ReviewDayOfMonthChanged.String(), event.ReviewDayOfMonthChanged)
			require.Equal(t, review.Spec.Changes.RemovedMembers, event.RemovedMembers)
		})

		// Update info for the review.
		review.Spec.Reviewers = []string{username}
		review.Spec.ReviewDate = svc.clock.Now()
		review.SetName(resp.ReviewName)
	}
}

func sortedStrings(src []string) []string {
	duplicate := make([]string, len(src))
	copy(duplicate, src)
	sort.Strings(duplicate)
	return duplicate
}

func listAllAccessListReviews(ctx context.Context, t *testing.T, service *Service, accessListName string, pageSize int) []*accesslist.Review {
	t.Helper()

	var nextToken string
	var reviews []*accesslist.Review
	for {
		resp, err := service.ListAccessListReviews(ctx, &accesslistv1.ListAccessListReviewsRequest{
			PageSize:   int32(pageSize),
			NextToken:  nextToken,
			AccessList: accessListName,
		})
		require.NoError(t, err)

		for _, review := range resp.Reviews {
			reviews = append(reviews, mustFromReviewProto(t, review))
		}

		nextToken = resp.NextToken
		if nextToken == "" {
			break
		}
	}

	return reviews
}

func genUserContext(ctx context.Context, username string, groups []string, traits map[string][]string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
			Groups:   groups,
			Traits:   traits,
		},
	})
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
					Name:        ownerUser,
					Description: "owner user",
				},
				{
					Name:        ownerUser2,
					Description: "owner user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now().Add(time.Hour * 8700),
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

func newAccessListMember(t *testing.T, accessListName, memberName string, clock clockwork.Clock) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessListName,
			Name:       memberName,
			Joined:     clock.Now().UTC(),
			Expires:    clock.Now().UTC().Add(24 * time.Hour),
			Reason:     "because",
			AddedBy:    testUser,
		},
	)
	require.NoError(t, err)

	return member
}

func newAccessListMemberWithIneligibleReason(t *testing.T, accessListName, memberName string, clock clockwork.Clock, ineligibleReason string) *accesslist.AccessListMember {
	t.Helper()

	member := newAccessListMember(t, accessListName, memberName, clock)
	member.Spec.IneligibleStatus = ineligibleReason

	return member
}

func mustFromProto(t *testing.T, accessList *accesslistv1.AccessList, opts ...conv.AccessListOption) *accesslist.AccessList {
	t.Helper()

	out, err := conv.FromProto(accessList, opts...)
	require.NoError(t, err)

	return out
}

func mustFromMemberProto(t *testing.T, member *accesslistv1.Member, opts ...conv.MemberOption) *accesslist.AccessListMember {
	t.Helper()

	out, err := conv.FromMemberProto(member, opts...)
	require.NoError(t, err)

	return out
}

func mustFromReviewProto(t *testing.T, review *accesslistv1.Review) *accesslist.Review {
	t.Helper()

	out, err := conv.FromReviewProto(review)
	require.NoError(t, err)

	return out
}

func mustFromProtoAll(t *testing.T, accessLists ...*accesslistv1.AccessList) []*accesslist.AccessList {
	t.Helper()

	var convertedAccessLists []*accesslist.AccessList
	for _, accessList := range accessLists {
		out, err := conv.FromProto(accessList)
		require.NoError(t, err)
		convertedAccessLists = append(convertedAccessLists, out)
	}

	return convertedAccessLists
}

func createAccessListsAndMembers(t *testing.T, ctx context.Context, service *Service, emitter *eventstest.ChannelEmitter,
	usageEvents *usageEventsClient, accessLists []*accesslist.AccessList, members []*accesslist.AccessListMember,
) {
	t.Helper()

	for _, al := range accessLists {
		_, err := service.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(al)})
		require.NoError(t, err)
		expectEvent(t, events.AccessListCreateSuccessCode, emitter, func(event *apievents.AccessListCreate) {
			require.True(t, event.Success)
		})
		if usageEvents != nil {
			expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
				require.Equal(t, al.GetName(), event.AccessListCreate.Metadata.Id)
			})
		}
	}

	for _, member := range members {
		_, err := service.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(member)})
		require.NoError(t, err)
		expectEvent(t, events.AccessListMemberCreateSuccessCode, emitter, func(event *apievents.AccessListMemberCreate) {
			require.True(t, event.Success)
			if usageEvents != nil {
				expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
					require.Equal(t, member.Spec.AccessList, event.AccessListMemberCreate.Metadata.Id)
				})
			}
		})
	}
}

func newAccessListReview(t *testing.T, accessListName string) *accesslist.Review {
	t.Helper()

	review, err := accesslist.NewReview(
		header.Metadata{
			Name: "dummy", // This will be overwritten by the service.
		},
		accesslist.ReviewSpec{
			AccessList: accessListName,
			Reviewers:  []string{"dummy"}, // This will be overwritten as well.
			ReviewDate: time.Now(),        // This will be overwritten by the service.
		},
	)
	require.NoError(t, err)

	return review
}

func expectEvent[T apievents.AuditEvent](t *testing.T, code string, emitter *eventstest.ChannelEmitter, fn func(T)) {
	t.Helper()

	select {
	case event := <-emitter.C():
		unwrapped, ok := event.(T)
		require.True(t, ok, "got unexpected type %T", event)
		require.Equal(t, code, unwrapped.GetCode())
		fn(unwrapped)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event")
	}
}

func expectUsageEvent[T any](t *testing.T, usageEvents *usageEventsClient, fn func(T)) {
	t.Helper()

	require.NotEmpty(t, usageEvents.events)

	event := usageEvents.events[0].Event
	unwrapped, ok := event.(T)
	require.True(t, ok, "got unexpected type %T", event)

	// Remove the tp event
	usageEvents.events = usageEvents.events[1:]

	fn(unwrapped)
}
