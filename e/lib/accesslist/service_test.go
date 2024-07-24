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
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	testUser          = "test-user"
	testUserWhere     = "test-user-where"
	testUserDenyWhere = "test-user-deny-where"
	testUserDenyAll   = "test-user-deny-all"
	ownerUser         = "owner-user"
	ownerUser2        = "owner-user2"
	member1           = "member1"
	member2           = "member2"
	member3           = "member3"
)

// cmpOpts are general cmpOpts for all comparisons.
var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.SortSlices(func(a, b *accesslist.AccessList) bool {
		return a.GetName() < b.GetName()
	}),
	cmpopts.SortSlices(func(a, b *accesslist.Review) bool {
		return a.GetName() < b.GetName()
	}),
}

func TestService_GetAccessLists(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	// a3 will have different ownership requirements.
	a3 := newAccessList(t, "3", c.clock)
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)

	// a4 will have a label attached.
	a4 := newAccessList(t, "4", c.clock)
	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a4.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	a1.Status.MemberCount = ptrToUint32(2)
	a2.Status.MemberCount = ptrToUint32(1)
	a3.Status.MemberCount = ptrToUint32(2)
	a4.Status.MemberCount = ptrToUint32(0)

	getResp, err = c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// owner should only see a1 and a2
	getResp, err = c.svc.GetAccessLists(c.ownerCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// userDenyWhereWhere shouldn't see anything even though it's an owner
	_, err = c.svc.GetAccessLists(c.userDenyAllCtx, &accesslistv1.GetAccessListsRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// member can see the access lists they belong to without counts.
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	a1.Status.MemberCount = nil
	a3.Status.MemberCount = nil
	getResp, err = c.svc.GetAccessLists(memberCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// userWhere can only see a4
	getResp, err = c.svc.GetAccessLists(c.userWhereCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a4}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))
}

func TestService_ListAccessLists(t *testing.T) {
	c := initSvc(t)

	accessLists := listAccessLists(c.userCtx, t, c.svc, 1)
	require.Empty(t, accessLists)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessList(t, "5", c.clock)
	a6 := newAccessList(t, "6", c.clock)

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	// a6 will have a label attached.
	a6.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a6.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, c.clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, c.clock)
	a5m1 := newAccessListMember(t, a5.GetName(), member1, c.clock)
	a5m2 := newAccessListMember(t, a5.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, []*accesslist.AccessListMember{
			a1m1, a1m2, a2m1, a3m1, a3m2, a4m1, a4m2, a5m1, a5m2,
		})

	a1.Status.MemberCount = ptrToUint32(2)
	a2.Status.MemberCount = ptrToUint32(1)
	a3.Status.MemberCount = ptrToUint32(2)
	a4.Status.MemberCount = ptrToUint32(2)
	a5.Status.MemberCount = ptrToUint32(2)
	a6.Status.MemberCount = ptrToUint32(0)

	accessLists = listAccessLists(c.userCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, accessLists, cmpOpts...))

	// owner should only see a1, a2, a4, a5
	accessLists = listAccessLists(c.ownerCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))

	// userDenyWhere should only see a1, a2, a4, a5
	accessLists = listAccessLists(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))

	// Add a label that should be denied to a5
	a5.SetStaticLabels(map[string]string{
		"denied": "true",
	})
	_, err := c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{
		AccessList: conv.ToProto(a5),
	})
	require.NoError(t, err)

	// userDenyWhere should no longer see a5
	accessLists = listAccessLists(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4}, accessLists, cmpOpts...))

	a1.Status.MemberCount = nil
	a3.Status.MemberCount = nil
	a4.Status.MemberCount = nil
	a5.Status.MemberCount = nil

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	accessLists = listAccessLists(memberCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	// Use the page size defaults
	accessLists = listAccessLists(memberCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	// User where should only see a6
	accessLists = listAccessLists(c.userWhereCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a6}, accessLists, cmpOpts...))
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
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessList(t, "5", c.clock)

	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a5.SetOrigin(types.OriginOkta)

	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a1.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// User tries to create a new access list that they own. Shouldn't work.
	_, err = c.svc.UpsertAccessList(c.ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a2.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// Owner cannot make any modifications
	a2.Spec.Audit.NextAuditDate = c.clock.Now().AddDate(100, 0, 0)
	_, err = c.svc.UpsertAccessList(c.ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))

	// Admin can make modifications
	a2.Spec.Audit.NextAuditDate = c.clock.Now().AddDate(100, 0, 0)
	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
	})

	// UserWhere cannot upsert a3
	_, err = c.svc.UpsertAccessList(c.userWhereCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.True(t, trace.IsAccessDenied(err))

	// UserWhere can upsert a4
	_, err = c.svc.UpsertAccessList(c.userWhereCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a4)})
	require.NoError(t, err)

	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a4.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// Even RBAC users can't create Okta sourced lists.
	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a5)})
	require.True(t, trace.IsAccessDenied(err))

	// Okta user can, though
	oktaIdentity := auth.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	_, err = c.svc.UpsertAccessList(oktaUserCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a5)})
	require.NoError(t, err)

	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a5.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// RBAC users can't update Okta sourced lists either.
	a5.Spec.Grants.Roles = append(a5.Spec.Grants.Roles, "new-role")
	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a5)})
	require.True(t, trace.IsAccessDenied(err))

	// Okta user can do that too.
	_, err = c.svc.UpsertAccessList(oktaUserCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a5)})
	require.NoError(t, err)

	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a5.GetName(), event.AccessListUpdate.Metadata.Id)
	})
}

func TestService_GetAccessList(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
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

	a1 := newAccessList(t, "1", c.clock)
	a1.Spec.Owners = eligibleOwnersWithStatus

	a2 := newAccessList(t, "2", c.clock)
	a2.Spec.Owners = eligibleOwnersWithStatus

	// a3 will have different ownership requirements.
	a3 := newAccessList(t, "3", c.clock)
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}
	a3.Spec.Owners = []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "owner user",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)],
		},
	}

	// a4 will have a label attached.
	a4 := newAccessList(t, "4", c.clock)
	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a4.Spec.Owners = a3.Spec.Owners
	a4.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)

	a1.Status.MemberCount = ptrToUint32(2)
	a2.Status.MemberCount = ptrToUint32(1)
	a3.Status.MemberCount = ptrToUint32(2)
	a4.Status.MemberCount = ptrToUint32(0)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	get, err := c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a4.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a4, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	// member2 can't see a2
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1.Status.MemberCount = nil

	get, err = c.svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	_, err = c.svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	a3.Status.MemberCount = nil

	get, err = c.svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	// Owner can see the member counts.
	a1.Status.MemberCount = ptrToUint32(2)

	get, err = c.svc.GetAccessList(c.ownerCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	get, err = c.svc.GetAccessList(c.ownerCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))

	// owner can't see a3
	_, err = c.svc.GetAccessList(c.ownerCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// userDenyWhere can't see a2
	_, err = c.svc.GetAccessList(c.userDenyWhereCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// userDenyWhere gets access denied for non-existent list
	_, err = c.svc.GetAccessList(c.userDenyWhereCtx, &accesslistv1.GetAccessListRequest{Name: "non-existent-list"})
	fmt.Printf("Error: %v\n", err)
	require.True(t, trace.IsAccessDenied(err))

	// userWhere can only see a4
	_, err = c.svc.GetAccessList(c.userWhereCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	get, err = c.svc.GetAccessList(c.userWhereCtx, &accesslistv1.GetAccessListRequest{Name: a4.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a4, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.Spec.Owners)), cmpOpts...))
}

func TestService_GetAccessListsToReview(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	resp, err := c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.AccessLists)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessList(t, "5", c.clock)

	a1.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	a2.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a3.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a4.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a5.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{a1, a2, a3, a4, a5}, nil)

	c.setDate(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.AccessLists)

	c.setDate(time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))

	c.setDate(time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))
	c.setDate(time.Date(2024, 2, 16, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5}, mustFromProtoAll(t, resp.AccessLists...), cmpOpts...))
}

func TestService_UpsertAndGetAccessList_OwnersIneligibleReason(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	// Create an access list, with varying eligbility for owners.
	a1 := newAccessList(t, "1", c.clock)
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
	createdAccessList, err := c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
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
	getAccessList, err := c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1.Spec.Owners, mustFromProto(t, getAccessList, conv.WithOwnersIneligibleStatusField(getAccessList.Spec.Owners)).GetOwners(), cmpOpts...))
}

func TestService_UpsertAndGetAccessList_MembersIneligibleReason(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	// Create an access list.
	a1 := newAccessList(t, "1", c.clock)
	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	// Create some members with varying eligiblity.
	member_expired, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: member1,
		},
		accesslist.AccessListMemberSpec{
			AccessList:       a1.GetName(),
			Name:             member1,
			Joined:           c.clock.Now().UTC(),
			Expires:          c.clock.Now().UTC().Add(-24 * time.Hour),
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
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), member2, c.clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)]),
		// NOK membership_requires does not match
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), ownerUser, c.clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)]),
	}

	// Test that member's ineligible status got stripped before upsertion.
	for _, member := range membersToCreate {
		upsertedMember, err := c.svc.UpsertAccessListMember(c.userCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(member)})
		require.NoError(t, err)
		require.Empty(t, upsertedMember.Spec.IneligibleStatus)
	}

	// Check retrieved members list has determined the ineligible status field.
	getMembers, err := c.svc.ListAccessListMembers(c.userCtx, &accesslistv1.ListAccessListMembersRequest{PageSize: 0, AccessList: a1.GetName()})
	require.NoError(t, err)

	var members []*accesslist.AccessListMember
	for _, member := range getMembers.Members {
		members = append(members, mustFromMemberProto(t, member, conv.WithMemberIneligibleStatusField(member)))
	}
	require.Empty(t, cmp.Diff(membersToCreate, members, cmpOpts...))
}

func TestService_DeleteAccessList(t *testing.T) {
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2}, nil)

	a1.Status.MemberCount = ptrToUint32(0)
	a2.Status.MemberCount = ptrToUint32(0)

	get, err := c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	_, err = c.svc.DeleteAccessList(c.userWhereCtx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	_, err = c.svc.DeleteAccessList(c.userCtx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a1.GetName(), event.AccessListDelete.Metadata.Id)
	})

	_, err = c.svc.DeleteAccessList(c.userCtx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.True(t, trace.IsNotFound(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	_, err = c.svc.DeleteAccessList(c.userWhereCtx, &accesslistv1.DeleteAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a2.GetName(), event.AccessListDelete.Metadata.Id)
	})

	// Delete non existent access list if auth err is non nill
	_, err = c.svc.DeleteAccessList(c.userDenyWhereCtx, &accesslistv1.DeleteAccessListRequest{Name: "non-existent access list"})
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
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

type usageReporter struct {
	events []usagereporter.Anonymizable
}

func (u *usageReporter) AnonymizeAndSubmit(events ...usagereporter.Anonymizable) {
	u.events = append(u.events, events...)
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

func (a *fakeAuth) GetRole(ctx context.Context, name string) (types.Role, error) {
	return &types.RoleV6{}, nil
}

func (a *fakeAuth) GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error) {
	return &types.UserV2{}, nil
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

type testEnvironment struct {
	identity    services.Identity
	accessLists services.AccessLists
}

type testSvcComponents struct {
	userCtx          context.Context
	userWhereCtx     context.Context
	userDenyWhereCtx context.Context
	userDenyAllCtx   context.Context
	ownerCtx         context.Context
	svc              *Service
	clock            clockwork.FakeClock
	emitter          *eventstest.ChannelEmitter
	usageEvents      *usageEventsClient
	usageReporter    *usageReporter
	testEnv          *testEnvironment
}

type testSvcOptions struct {
	disabledReconciler bool
}

type svcOpts func(*testSvcOptions)

func withDisabledReconciler() svcOpts {
	return func(o *testSvcOptions) {
		o.disabledReconciler = true
	}
}

func initSvc(t *testing.T, opts ...svcOpts) testSvcComponents {
	var options testSvcOptions
	for _, opt := range opts {
		opt(&options)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
			Cloud: true,
		},
	})

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewIdentityService(backend)
	storage, err := local.NewAccessListService(backend, clock, local.WithRunWhileLockedRetryInterval(-1*time.Millisecond))
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
	emitter := eventstest.NewChannelEmitter(10)
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

	clt := client{
		Access:        accessService,
		Identity:      userSvc,
		AccessLists:   storage,
		EventsService: eventService,
	}

	role, err := auth.CreateRole(ctx, clt, "access-lists", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList, types.KindUser},
					Verbs:     services.RW(),
				},
			},
		},
	})
	require.NoError(t, err)

	roleWhere, err := auth.CreateRole(ctx, clt, "access-lists-where", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList},
					Verbs:     services.RW(),
					Where: builder.Equals(
						builder.Identifier(`resource.metadata.labels["test-label"]`),
						builder.String("test"),
					).String(),
				},
			},
		},
	})
	require.NoError(t, err)

	roleDenyWhere, err := auth.CreateRole(ctx, clt, "access-lists-deny-where", types.RoleSpecV6{
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList},
					Verbs:     services.RW(),
					Where: builder.Equals(
						builder.Identifier(`resource.metadata.labels["denied"]`),
						builder.String("true"),
					).String(),
				},
			},
		},
	})
	require.NoError(t, err)

	roleDenyAll, err := auth.CreateRole(ctx, clt, "access-lists-deny-all", types.RoleSpecV6{
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList},
					Verbs:     services.RW(),
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

	userWhere, err := types.NewUser(testUserWhere)
	require.NoError(t, err)
	userWhere.AddRole(roleWhere.GetName())

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

	userDenyWhere, err := types.NewUser(testUserDenyWhere)
	require.NoError(t, err)
	userDenyWhere.SetRoles(ownerRoles)
	userDenyWhere.SetTraits(ownerTraits)
	userDenyWhere.AddRole(roleDenyWhere.GetName())

	userDenyAll, err := types.NewUser(testUserDenyAll)
	require.NoError(t, err)
	userDenyAll.SetRoles(ownerRoles)
	userDenyAll.SetTraits(ownerTraits)
	userDenyAll.AddRole(roleDenyAll.GetName())

	user, err = userSvc.CreateUser(ctx, user)
	require.NoError(t, err)
	userWhere, err = userSvc.CreateUser(ctx, userWhere)
	require.NoError(t, err)
	userDenyWhere, err = userSvc.CreateUser(ctx, userDenyWhere)
	require.NoError(t, err)
	userDenyAll, err = userSvc.CreateUser(ctx, userDenyAll)
	require.NoError(t, err)
	owner, err = userSvc.CreateUser(ctx, owner)
	require.NoError(t, err)
	_, err = userSvc.CreateUser(ctx, owner2)
	require.NoError(t, err)

	locks := local.NewAccessService(backend)

	usageEvents := &usageEventsClient{}
	usageReporter := &usageReporter{}
	svc, err := NewService(
		ctx,
		ServiceConfig{
			Authorizer:        authorizer,
			AccessLists:       storage,
			LockGetter:        locks,
			AccessListReviews: storage,
			Emitter:           emitter,
			UsageEvents:       usageEvents,
			UsageReporter:     usageReporter,
			Clock:             clock,
			Cache:             &clt,
			AuthServer:        &fakeAuth{},
			Backend:           backend,
			disableReconciler: options.disabledReconciler,
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

	return testSvcComponents{
		userCtx:          genUserContext(ctx, user.GetName(), user.GetRoles(), nil),
		userWhereCtx:     genUserContext(ctx, userWhere.GetName(), userWhere.GetRoles(), nil),
		userDenyWhereCtx: genUserContext(ctx, userDenyWhere.GetName(), userDenyWhere.GetRoles(), userDenyWhere.GetTraits()),
		userDenyAllCtx:   genUserContext(ctx, userDenyAll.GetName(), userDenyAll.GetRoles(), userDenyAll.GetTraits()),
		ownerCtx:         genUserContext(ctx, owner.GetName(), owner.GetRoles(), owner.GetTraits()),
		svc:              svc,
		clock:            clock,
		emitter:          emitter,
		usageEvents:      usageEvents,
		usageReporter:    usageReporter,
		testEnv: &testEnvironment{
			identity:    userSvc,
			accessLists: storage,
		},
	}
}

func (c *testSvcComponents) setDate(date time.Time) {
	now := c.clock.Now()
	c.clock.Advance(date.Sub(now))
}

func TestService_CountAccessListMembers(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a1m3 := newAccessListMember(t, a1.GetName(), member3, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a1m3, a2m1, a3m1, a3m2})

	resp, err := c.svc.CountAccessListMembers(c.userCtx, &accesslistv1.CountAccessListMembersRequest{AccessListName: a1.GetName()})
	require.NoError(t, err)
	require.Equal(t, uint32(3), resp.Count)

	resp, err = c.svc.CountAccessListMembers(c.ownerCtx, &accesslistv1.CountAccessListMembersRequest{AccessListName: a2.GetName()})
	require.NoError(t, err)
	require.Equal(t, uint32(1), resp.Count)

	resp, err = c.svc.CountAccessListMembers(c.userCtx, &accesslistv1.CountAccessListMembersRequest{AccessListName: a3.GetName()})
	require.NoError(t, err)
	require.Equal(t, uint32(2), resp.Count)

	resp, err = c.svc.CountAccessListMembers(c.userCtx, &accesslistv1.CountAccessListMembersRequest{AccessListName: a4.GetName()})
	require.NoError(t, err)
	require.Equal(t, uint32(0), resp.Count)

	memberCtx := genUserContext(context.Background(), member1, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	_, err = c.svc.CountAccessListMembers(memberCtx, &accesslistv1.CountAccessListMembersRequest{AccessListName: a1.GetName()})
	require.True(t, trace.IsAccessDenied(err))
}

func TestService_ListAccessListMembers(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	// a4 will have a label attached.
	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a4.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, c.clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2, a4m1, a4m2})

	// Admin should be able to list everything
	members := listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m1, a1m2}, members, cmpOpts...))

	// owner should be able to see members for a2
	members = listAllAccessListMembers(c.ownerCtx, t, c.svc, a2.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a2m1, a2m2}, members, cmpOpts...))

	// userDenyWhere should not be able to see members for a2
	_, err := c.svc.ListAccessListMembers(c.userDenyWhereCtx, &accesslistv1.ListAccessListMembersRequest{
		PageSize:   0,
		PageToken:  "",
		AccessList: a2.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	// owner should not be able to see members for a3
	_, err = c.svc.ListAccessListMembers(c.ownerCtx, &accesslistv1.ListAccessListMembersRequest{
		PageSize:   0,
		PageToken:  "",
		AccessList: a3.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should not be able to see members for a2
	_, err = c.svc.ListAccessListMembers(c.userWhereCtx, &accesslistv1.ListAccessListMembersRequest{
		PageSize:   0,
		PageToken:  "",
		AccessList: a2.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should be able to see members for a4
	members = listAllAccessListMembers(c.userWhereCtx, t, c.svc, a4.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a4m1, a4m2}, members, cmpOpts...))
}

func TestService_GetAccessListMember(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	// a1 will have a label attached.
	a1.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// Admin should be able to get members
	member, err := c.svc.GetAccessListMember(c.userCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m1, mustFromMemberProto(t, member), cmpOpts...))

	// owner should be able to see members for a1
	member, err = c.svc.GetAccessListMember(c.ownerCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m2, mustFromMemberProto(t, member), cmpOpts...))

	// userDenyWhere should not be able to see members for a1
	_, err = c.svc.GetAccessListMember(c.userDenyWhereCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should be able to see members for a2
	member, err = c.svc.GetAccessListMember(c.userWhereCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a2.GetName(), MemberName: a2m2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2m2, mustFromMemberProto(t, member), cmpOpts...))
}

type client struct {
	services.Access
	services.Identity
	services.AccessLists
	*local.EventsService
}

func TestService_UpsertAccessListMember(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	_, err := c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a1.GetName(), event.AccessListCreate.Metadata.Id)
	})

	_, err = c.svc.UpsertAccessList(c.userCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a2.GetName(), event.AccessListCreate.Metadata.Id)
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)

	require.Equal(t, testUser, a1m1.Spec.AddedBy)

	_, err = c.svc.UpsertAccessListMember(c.userWhereCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.True(t, trace.IsAccessDenied(err))

	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)

	_, err = c.svc.UpsertAccessListMember(c.userWhereCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a2m1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a2.GetName(), event.AccessListMemberCreate.Metadata.Id)
	})

	got, err := c.svc.UpsertAccessListMember(c.ownerCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberCreate.Metadata.Id)
	})

	// Added by should be overridden from "test-user" to "owner-user"
	want := a1m1
	want.Spec.AddedBy = ownerUser

	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	got, err = c.svc.GetAccessListMember(c.userCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	// Update from admin should still retain the old added by, reason, and joined
	oldJoined := a1m1.Spec.Joined
	oldReason := a1m1.Spec.Reason
	want.Spec.Reason = "some new reason"
	want.Spec.Joined = c.clock.Now().Add(time.Hour * 24)
	_, err = c.svc.UpsertAccessListMember(c.userCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberUpdate.Metadata.Id)
	})

	want.Spec.Joined = oldJoined
	want.Spec.Reason = oldReason
	got, err = c.svc.GetAccessListMember(c.userCtx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	// Owner can't add themselves as a member
	_, err = c.svc.UpsertAccessListMember(c.ownerCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(newAccessListMember(t, a1.GetName(), ownerUser, c.clock))})
	require.ErrorIs(t, err, trace.AccessDenied("user cannot add themselves to an access list"))
	expectEvent(t, events.AccessListMemberCreateFailureCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.False(t, event.Success)
	})

	// User with KindUser access can add themselves as a member.
	_, err = c.svc.UpsertAccessListMember(c.userCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(newAccessListMember(t, a1.GetName(), testUser, c.clock))})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberCreate.Metadata.Id)
	})
}

func TestService_DeleteAccessListMember(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	// a1 will have a label attached.
	a1.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
		[]*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// userWhere can't delete from a1
	_, err := c.svc.DeleteAccessListMember(c.userWhereCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// Admin should be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.userCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})

	members := listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m2}, members, cmpOpts...))

	// userDenyWhere should not be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.userDenyWhereCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// owner should be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.ownerCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})
	members = listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, members)

	// userWhere can delete from a2
	_, err = c.svc.DeleteAccessListMember(c.userWhereCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a2.GetName(), MemberName: a2m1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a2.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})
}

func TestService_DeleteAllAccessListMembersForAccessList(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	// a1 will have a label attached.
	a1.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a3 will have a label attached.
	a3.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
		[]*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2})

	// userWhere can't delete from a1
	_, err := c.svc.DeleteAllAccessListMembersForAccessList(c.userWhereCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a1.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// Admin should be able to delete members
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a1.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members := listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, members)

	// userDenyWhere can't delete from a1
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userDenyWhereCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a1.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	// owner should be able to delete members
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.ownerCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a2.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members = listAllAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 1)
	require.Empty(t, members)

	// UserWhere should be able to delete members from a3
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userWhereCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a3.GetName()})
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})
}

func TestService_UpsertAccessListWithMembers_IneligibleStatus(t *testing.T) {
	c := initSvc(t)

	// Create a list.
	testlist := newAccessList(t, "testlist", c.clock)

	// Replace owners with a ineligible new owner.
	testlist.SetOwners([]accesslist.Owner{{Name: testUser}})

	// Replace members with a ineligible new member.
	member := newAccessListMember(t, testlist.GetName(), ownerUser, c.clock)

	resp, err := c.svc.UpsertAccessListWithMembers(c.userCtx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(testlist),
		Members: conv.ToMembersProto([]*accesslist.AccessListMember{
			member,
		}),
	})
	require.NoError(t, err)

	require.Len(t, resp.AccessList.Spec.GetOwners(), 1)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS, resp.AccessList.Spec.GetOwners()[0].IneligibleStatus)

	require.Len(t, resp.Members, 1)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS, resp.Members[0].GetSpec().IneligibleStatus)
}

func TestService_UpsertAccessListWithMembers(t *testing.T) {
	// disable reconciler to avoid extra update events
	c := initSvc(t, withDisabledReconciler())

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	aOkta := newAccessList(t, "okta", c.clock)

	aOkta.SetOrigin(types.OriginOkta)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member3, c.clock)
	a2m2 := newAccessListMemberWithIneligibleReason(t, a2.GetName(), "user4", c.clock, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST)])

	oktaIdentity := auth.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1})

	upsertAccessListWithMembers := func(t *testing.T, ctx context.Context, accessList *accesslist.AccessList,
		members []*accesslist.AccessListMember, wantErrFn require.ErrorAssertionFunc,
	) {
		oldAccessListResp, err := c.svc.GetAccessList(c.userCtx, &accesslistv1.GetAccessListRequest{
			Name: accessList.GetName(),
		})
		if err != nil && !trace.IsNotFound(err) {
			require.NoError(t, err)
		}

		var membersCreated, membersUpdated, membersDeleted int

		var oldAccessList *accesslist.AccessList
		if oldAccessListResp != nil {
			oldAccessList, err = conv.FromProto(oldAccessListResp)
			require.NoError(t, err)
		}
		var checkAccessListModificationEvent bool
		if !cmp.Equal(oldAccessList, accessList, ignoreEphemeralFields...) {
			checkAccessListModificationEvent = true
		}

		accessListCreated := false
		if oldAccessList == nil {
			accessListCreated = true
			membersCreated = len(members)
		} else {
			oldMembers, err := c.svc.getAccessListMemberMap(ctx, accessList.GetName())
			require.NoError(t, err)

			for _, member := range members {
				if _, ok := oldMembers[member.GetName()]; ok {
					membersUpdated++
					delete(oldMembers, member.GetName())
				} else {
					membersCreated++
				}
			}
			membersDeleted = len(oldMembers)
		}

		_, err = c.svc.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(accessList),
			Members:    conv.ToMembersProto(members),
		})
		wantErrFn(t, err)

		if checkAccessListModificationEvent {
			if accessListCreated {
				if err == nil {
					expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
						require.True(t, event.Success)
					})
				} else {
					expectEvent(t, events.AccessListCreateFailureCode, c.emitter, func(event *apievents.AccessListCreate) {
						require.False(t, event.Success)
					})
				}
			} else {
				if err == nil {
					expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
						require.True(t, event.Success)
					})
				} else {
					expectEvent(t, events.AccessListUpdateFailureCode, c.emitter, func(event *apievents.AccessListUpdate) {
						require.False(t, event.Success)
					})
				}
			}
		}

		if err != nil {
			return
		}

		if checkAccessListModificationEvent {
			if accessListCreated {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
					require.Equal(t, accessList.GetName(), event.AccessListCreate.Metadata.Id)
				})
			} else {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
					require.Equal(t, accessList.GetName(), event.AccessListUpdate.Metadata.Id)
				})
			}
		}

		if membersCreated > 0 {
			expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
				require.True(t, event.Success)
				require.Len(t, event.AccessListMemberMetadata.Members, membersCreated)
			})
			for i := 0; i < membersCreated; i++ {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
					require.Equal(t, accessList.GetName(), event.AccessListMemberCreate.Metadata.Id)
				})
			}
		}
		if membersUpdated > 0 {
			expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
				require.True(t, event.Success)
				require.Len(t, event.AccessListMemberMetadata.Members, membersUpdated)
			})
			for i := 0; i < membersUpdated; i++ {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
					require.Equal(t, accessList.GetName(), event.AccessListMemberUpdate.Metadata.Id)
				})
			}
		}
		if membersDeleted > 0 {
			expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
				require.True(t, event.Success)
				require.Len(t, event.AccessListMemberMetadata.Members, membersDeleted)
			})
			for i := 0; i < membersDeleted; i++ {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
					require.Equal(t, accessList.GetName(), event.AccessListMemberDelete.Metadata.Id)
				})
			}
		}
	}

	// Sanity check
	membersA1 := listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 2)
	require.Len(t, membersA1, 2)

	membersA2 := listAllAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
	require.Len(t, membersA2, 1)

	t.Run("create a new access list with members", func(t *testing.T) {
		a4 := newAccessList(t, "4", c.clock)
		upsertAccessListWithMembers(t, c.userCtx, a4, []*accesslist.AccessListMember{
			newAccessListMember(t, a4.GetName(), member1, c.clock),
			newAccessListMember(t, a4.GetName(), member2, c.clock),
		}, require.NoError)

		membersA4 := listAllAccessListMembers(c.userCtx, t, c.svc, a4.GetName(), 3)
		require.Len(t, membersA4, 2)
	})

	t.Run("remove one member", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.userCtx, a1, []*accesslist.AccessListMember{
			a1m1,
		}, require.NoError)

		// One member should have been deleted
		membersA1 = listAllAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 2)
		require.Len(t, membersA1, 1)
	})

	t.Run("add one member", func(t *testing.T) {
		// Add one member to a2
		upsertAccessListWithMembers(t, c.userCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
		}, require.NoError)

		// One member should have been added
		membersA2 = listAllAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Len(t, membersA2, 2)
	})

	t.Run("remove all members", func(t *testing.T) {
		// If not members are provided all members should be deleted
		upsertAccessListWithMembers(t, c.userCtx, a2, nil, require.NoError)

		// All members should have been deleted
		membersA2 = listAllAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Empty(t, membersA2)
	})

	t.Run("owner can't modify owners", func(t *testing.T) {
		// Make a copy of a3.
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Owners = append(a3.Spec.Owners, accesslist.Owner{
			Name: "dummy",
		})
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify grant roles", func(t *testing.T) {
		// Make a copy of a3.
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Grants.Roles = append(a3.Spec.Grants.Roles, "dummy")
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify grant traits", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Grants.Traits["dummy"] = []string{"value1", "value2"}
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify membership requires", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"some-new-role"},
		}
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify audit", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Audit.NextAuditDate = a3.Spec.Audit.NextAuditDate.Add(24 * time.Hour * 365)
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("userDenyWhere can't modify members", func(t *testing.T) {
		_, err := c.svc.UpsertAccessListWithMembers(c.userWhereCtx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a3),
			Members: []*accesslistv1.Member{
				conv.ToMemberProto(newAccessListMember(t, a3.GetName(), member1, c.clock)),
				conv.ToMemberProto(newAccessListMember(t, a3.GetName(), member2, c.clock)),
			},
		})
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("owner can modify members", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.ownerCtx, a3, []*accesslist.AccessListMember{
			newAccessListMember(t, a3.GetName(), member1, c.clock),
			newAccessListMember(t, a3.GetName(), member2, c.clock),
		}, require.NoError)

		// One member should have been deleted
		membersA3 := listAllAccessListMembers(c.userCtx, t, c.svc, a3.GetName(), 3)
		require.Len(t, membersA3, 2)
	})

	t.Run("member can't modify members", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		upsertAccessListWithMembers(t, memberCtx, a3, []*accesslist.AccessListMember{
			newAccessListMember(t, a3.GetName(), member1, c.clock),
			newAccessListMember(t, a3.GetName(), member2, c.clock),
		}, func(t require.TestingT, err error, i ...interface{}) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't add itself as a member", func(t *testing.T) {
		ownerMember := newAccessListMember(t, a2.GetName(), ownerUser, c.clock)

		// Owner tries to add itself as a member.
		upsertAccessListWithMembers(t, c.ownerCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
		}, func(t require.TestingT, err error, i ...interface{}) {
			require.ErrorIs(t, err, trace.AccessDenied("user cannot add themselves to an access list"))
		})

		// Admin user adds owner to access list.
		upsertAccessListWithMembers(t, c.userCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
		}, require.NoError)

		// Owner adds another user, including themselves, which is okay since it already exists.
		// We'll modify joined, which is a field that should be preserved. This will verify that the modification
		// preserves the correct values.
		oldJoin := ownerMember.Spec.Joined
		ownerMember.Spec.Joined = time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)
		upsertAccessListWithMembers(t, c.ownerCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
			newAccessListMember(t, a2.GetName(), "new-user1", c.clock),
		}, require.NoError)
		ownerMember.Spec.Joined = oldJoin

		resp, err := c.svc.GetAccessListMember(c.userCtx, &accesslistv1.GetAccessListMemberRequest{
			AccessList: ownerMember.Spec.AccessList,
			MemberName: ownerMember.Metadata.Name,
		})
		require.NoError(t, err)
		protoMember, err := conv.FromMemberProto(resp)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(ownerMember, protoMember, cmpOpts...), "ownerMember x storage comparison mismatch")

		// Owner attempts to modify their own user.
		ownerMember.Spec.Expires = c.clock.Now()
		upsertAccessListWithMembers(t, c.ownerCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
			newAccessListMember(t, a2.GetName(), "new-user1", c.clock),
		}, func(t require.TestingT, err error, i ...interface{}) {
			require.ErrorIs(t, err, trace.AccessDenied("user cannot add themselves to an access list"))
		})

		// Test user attempts to add their own user, which is okay because they have KindUser RBAC access.
		upsertAccessListWithMembers(t, c.userCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
			newAccessListMember(t, a2.GetName(), "new-user1", c.clock),
			newAccessListMember(t, a2.GetName(), testUser, c.clock),
		}, require.NoError)

		membersA2 = listAllAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Len(t, membersA2, 5)
	})

	t.Run("user where can't create access list without appropriate where clause being satisfied", func(t *testing.T) {
		a5 := conv.ToProto(newAccessList(t, "6", c.clock))

		_, err := c.svc.UpsertAccessListWithMembers(c.userWhereCtx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: a5,
		})
		require.True(t, trace.IsAccessDenied(err))

		expectEvent(t, events.AccessListCreateFailureCode, c.emitter, func(event *apievents.AccessListCreate) {
			require.False(t, event.Success)
		})
	})

	t.Run("create a new access list with members", func(t *testing.T) {
		a5 := newAccessList(t, "5", c.clock)

		// a5 will have a label attached.
		a5.SetStaticLabels(map[string]string{
			"test-label": "test",
		})

		upsertAccessListWithMembers(t, c.userCtx, a5, []*accesslist.AccessListMember{
			newAccessListMember(t, a5.GetName(), member1, c.clock),
			newAccessListMember(t, a5.GetName(), member2, c.clock),
		}, require.NoError)

		membersA5 := listAllAccessListMembers(c.userCtx, t, c.svc, a5.GetName(), 3)
		require.Len(t, membersA5, 2)
	})

	t.Run("userDenyWhere can't create or update an access list with where clause satisfied", func(t *testing.T) {
		a6 := newAccessList(t, "6", c.clock)

		// a6 will have a label attached.
		a6.SetStaticLabels(map[string]string{
			"denied": "true",
		})

		_, err := c.svc.UpsertAccessListWithMembers(c.userDenyWhereCtx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a6),
		})
		require.True(t, trace.IsAccessDenied(err))

		expectEvent(t, events.AccessListCreateFailureCode, c.emitter, func(event *apievents.AccessListCreate) {
			require.False(t, event.Success)
		})

		upsertAccessListWithMembers(t, c.userCtx, a6, []*accesslist.AccessListMember{
			newAccessListMember(t, a6.GetName(), member1, c.clock),
			newAccessListMember(t, a6.GetName(), member2, c.clock),
		}, require.NoError)

		a6.Spec.Description = "some new description"
		_, err = c.svc.UpsertAccessListWithMembers(c.userDenyWhereCtx, &accesslistv1.UpsertAccessListWithMembersRequest{
			AccessList: conv.ToProto(a6),
		})
		require.True(t, trace.IsAccessDenied(err))

		expectEvent(t, events.AccessListUpdateFailureCode, c.emitter, func(event *apievents.AccessListUpdate) {
			require.False(t, event.Success)
		})
	})

	t.Run("okta user can create Okta sourced access lists", func(t *testing.T) {
		upsertAccessListWithMembers(t, oktaUserCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, c.clock),
		}, require.NoError)
	})

	t.Run("owner can modify members for Okta sourced access lists", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.ownerCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, c.clock),
			newAccessListMember(t, aOkta.GetName(), member3, c.clock),
		}, require.NoError)

		// One member should have been added
		membersA3 := listAllAccessListMembers(c.userCtx, t, c.svc, aOkta.GetName(), 3)
		require.Len(t, membersA3, 3)
	})

	t.Run("user can modify membership requirements for Okta sourced access lists.", func(t *testing.T) {
		aOkta.Spec.MembershipRequires.Roles = append(aOkta.Spec.MembershipRequires.Roles, "some-new-role")
		upsertAccessListWithMembers(t, c.userCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, c.clock),
			newAccessListMember(t, aOkta.GetName(), member3, c.clock),
		}, require.NoError)
	})

	t.Run("owner can modify okta sourced audit properties", func(t *testing.T) {
		resp, err := c.svc.accessLists.GetAccessList(c.userCtx, "okta")
		require.NoError(t, err)
		resp.Spec.Audit.NextAuditDate = c.svc.clock.Now().Add(time.Hour * 24 * 10)
		_, err = c.svc.accessLists.UpsertAccessList(c.ownerCtx, resp)
		require.NoError(t, err)
	})
}

func TestService_AuthOrIsOwner(t *testing.T) {
	c := initSvc(t)
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	nonExistentUser := genUserContext(context.Background(), "doesnt-exist", []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{a1, a2}, nil)

	tests := []struct {
		name           string
		ctx            context.Context
		accessListName string
		wantErr        require.ErrorAssertionFunc
	}{
		{
			name:           "admin context",
			ctx:            c.userCtx,
			accessListName: a1.GetName(),
			wantErr:        require.NoError,
		},
		{
			name:           "owner context",
			ctx:            c.ownerCtx,
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
			_, err := c.svc.authOrIsOwner(test.ctx, test.accessListName, types.VerbRead)
			test.wantErr(t, err)
		})
	}
}

func TestBatchAccessListMemberMetadata(t *testing.T) {
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

func TestService_CreateAccessListReview(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	// a1 will have a label attached.
	a1.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a3.SetOrigin(types.OriginOkta)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2})

	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())

	// RBAC user with where clause can't create a review.
	_, err := c.svc.CreateAccessListReview(c.userWhereCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review1ForA1),
	})
	require.True(t, trace.IsAccessDenied(err))

	// RBAC user can create a review.
	_, err = c.svc.CreateAccessListReview(c.userCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review1ForA1),
	})
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a1.GetName())
	})

	// userDenyWhere can't create reviews.
	review := newAccessListReview(t, a1.GetName())
	review.Spec.Changes.RemovedMembers = []string{a1m1.GetName()}

	_, err = c.svc.CreateAccessListReview(c.userDenyWhereCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review),
	})
	require.True(t, trace.IsAccessDenied(err))

	// Owner can only create reviews with member changes.
	review = newAccessListReview(t, a1.GetName())
	review.Spec.Changes.RemovedMembers = []string{a1m1.GetName()}

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review),
	})
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a1.GetName())
	})

	// Owner can't create reviews with any other changes.
	review = newAccessListReview(t, a1.GetName())
	review.Spec.Changes.ReviewDayOfMonthChanged = accesslist.LastDayOfMonth

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review),
	})
	require.ErrorIs(t, err, trace.AccessDenied("user cannot modify the access list as part of the review"))

	expectEvent(t, events.AccessListReviewFailureCode, c.emitter, func(event *apievents.AccessListReview) {
		require.False(t, event.Success)
	})

	review = newAccessListReview(t, a1.GetName())
	review.Spec.Changes.MembershipRequirementsChanged = &accesslist.Requires{
		Roles: []string{"some-new-role"},
	}

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review),
	})
	require.ErrorIs(t, err, trace.AccessDenied("user cannot modify the access list as part of the review"))

	expectEvent(t, events.AccessListReviewFailureCode, c.emitter, func(event *apievents.AccessListReview) {
		require.False(t, event.Success)
	})

	review1ForA2 := newAccessListReview(t, a2.GetName())

	// RBAC user with where clause can create a review if access list meets where clause.
	_, err = c.svc.CreateAccessListReview(c.userWhereCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review1ForA2),
	})
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a2.GetName())
	})

	// Create the Okta sourced access list.
	oktaIdentity := auth.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	createAccessListsAndMembers(t, oktaUserCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a3}, []*accesslist.AccessListMember{a3m1, a3m2})

	review1ForA3 := newAccessListReview(t, a3.GetName())

	// User is allowed to change the various fields that can be changed in an Okta sourced access list.
	review1ForA3.Spec.Changes.RemovedMembers = []string{a3m1.GetName()}
	_, err = c.svc.CreateAccessListReview(c.userCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review1ForA3),
	})
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a3.GetName())
	})

	// User can change membership requirements.
	review1ForA3.Spec.Changes.MembershipRequirementsChanged = &accesslist.Requires{
		Roles: []string{"some-role"},
	}
	review1ForA3.Spec.Changes.RemovedMembers = nil

	_, err = c.svc.CreateAccessListReview(c.userCtx, &accesslistv1.CreateAccessListReviewRequest{
		Review: conv.ToReviewProto(review1ForA3),
	})
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a3.GetName())
	})
}

func TestService_ListAccessListReviews(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	// a3 will have a label attached.
	a3.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2, a3}, nil)

	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1))
	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a2.GetName(), 1))
	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a3.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())
	review2ForA1 := newAccessListReview(t, a1.GetName())
	review3ForA1 := newAccessListReview(t, a1.GetName())
	review1ForA2 := newAccessListReview(t, a2.GetName())
	review2ForA2 := newAccessListReview(t, a2.GetName())
	review1ForA3 := newAccessListReview(t, a3.GetName())
	review2ForA3 := newAccessListReview(t, a3.GetName())

	review3ForA1.Spec.Changes.MembershipRequirementsChanged = &accesslist.Requires{
		Roles: []string{"new-role1", "new-role2"},
		Traits: map[string][]string{
			"new-trait1": {"value1", "value2"},
			"new-trait2": {"value1", "value2"},
		},
	}

	createReviews(c.userCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA1,
		review2ForA1,
		review3ForA1,
	})
	createReviews(c.ownerCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA2,
		review2ForA2,
	})
	createReviews(c.ownerCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA3,
		review2ForA3,
	})

	reviews := listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA1, review2ForA1, review3ForA1}, reviews, cmpOpts...))
	reviews = listAllAccessListReviews(c.userCtx, t, c.svc, a2.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA2, review2ForA2}, reviews, cmpOpts...))

	_, err := c.svc.ListAccessListReviews(c.userWhereCtx, &accesslistv1.ListAccessListReviewsRequest{
		AccessList: a1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))
	reviews = listAllAccessListReviews(c.userWhereCtx, t, c.svc, a3.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA3, review2ForA3}, reviews, cmpOpts...))
}

func TestService_DeleteAccessListReviews(t *testing.T) {
	c := initSvc(t)

	// Set the clock to a fixed date. This will ensure that the expected days past calculation later on
	// will be predictable.
	c.clock.Advance(c.clock.Since(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	// a3 will have a label attached.
	a3.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2, a3}, nil)

	// Advance the clock so that the new reviews are past the review date.
	c.clock.Advance(time.Hour * 24 * 366) // 24 hours past the access list review date.

	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1))
	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a2.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())
	review2ForA1 := newAccessListReview(t, a1.GetName())
	review3ForA1 := newAccessListReview(t, a1.GetName())
	review1ForA2 := newAccessListReview(t, a2.GetName())
	review2ForA2 := newAccessListReview(t, a2.GetName())
	review1ForA3 := newAccessListReview(t, a3.GetName())
	review2ForA3 := newAccessListReview(t, a3.GetName())

	createReviews(c.userCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA1,
		review2ForA1,
		review3ForA1,
	})
	createReviews(c.ownerCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA2,
		review2ForA2,
	})
	createReviews(c.ownerCtx, t, c.svc, c.emitter, c.usageEvents, []*accesslist.Review{
		review1ForA3,
		review2ForA3,
	})

	_, err := c.svc.DeleteAccessListReview(c.userWhereCtx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.DeleteAccessListReview(c.ownerCtx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.DeleteAccessListReview(c.userCtx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	})
	require.NoError(t, err)

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewDelete) {
		require.Equal(t, a1.GetName(), event.AccessListReviewDelete.Metadata.Id)
		require.Equal(t, review1ForA1.GetName(), event.AccessListReviewDelete.AccessListReviewId)
	})

	reviews := listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review2ForA1, review3ForA1}, reviews, cmpOpts...))

	_, err = c.svc.DeleteAccessListReview(c.userWhereCtx, &accesslistv1.DeleteAccessListReviewRequest{
		AccessListName: review1ForA3.Spec.AccessList,
		ReviewName:     review1ForA3.GetName(),
	})
	require.NoError(t, err)

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewDelete) {
		require.Equal(t, a3.GetName(), event.AccessListReviewDelete.Metadata.Id)
		require.Equal(t, review1ForA3.GetName(), event.AccessListReviewDelete.AccessListReviewId)
	})
}

func member(t *testing.T, metadata header.Metadata, spec accesslist.AccessListMemberSpec) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(metadata, spec)
	require.NoError(t, err)
	return member
}

func TestPopulateMemberFields(t *testing.T) {
	tests := []struct {
		name          string
		currentTime   time.Time
		username      string
		oldMember     *accesslist.AccessListMember
		newMember     *accesslist.AccessListMember
		wantPreserved bool
		want          *accesslist.AccessListMember
	}{
		{
			name:        "no old member",
			currentTime: time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC),
			username:    "added-by",
			newMember: member(t, header.Metadata{Name: "member"}, accesslist.AccessListMemberSpec{
				Name:             "member",
				AccessList:       "access-list",
				Joined:           time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
				Reason:           "reason",
				AddedBy:          "dummy",
				IneligibleStatus: "ineligible",
			}),
			wantPreserved: false,
			want: member(t, header.Metadata{Name: "member"}, accesslist.AccessListMemberSpec{
				Name:             "member",
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC),
				Reason:           "reason",
				AddedBy:          "added-by",
				IneligibleStatus: "ineligible",
			}),
		},
		{
			name:        "old member preserved almost everything",
			currentTime: time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC),
			username:    "added-by-ignored",
			oldMember: member(t, header.Metadata{Name: "member", Labels: map[string]string{"label": "value"}}, accesslist.AccessListMemberSpec{
				Name:             "member",
				AccessList:       "original access-list",
				Joined:           time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
				Expires:          time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC),
				Reason:           "original reason",
				AddedBy:          "original dummy",
				IneligibleStatus: "original ineligible",
			}),
			newMember: member(t, header.Metadata{Name: "member", Labels: map[string]string{"label": "value"}}, accesslist.AccessListMemberSpec{
				Name:             "member",
				AccessList:       "new access-list",
				Joined:           time.Date(2020, 2, 2, 2, 2, 2, 2, time.UTC),
				Expires:          time.Date(2024, 2, 2, 2, 2, 2, 2, time.UTC),
				Reason:           "new reason",
				AddedBy:          "new dummy",
				IneligibleStatus: "new ineligible",
			}),
			wantPreserved: true,
			want: member(t, header.Metadata{Name: "member", Labels: map[string]string{"label": "value"}}, accesslist.AccessListMemberSpec{
				Name:             "member",
				AccessList:       "original access-list",
				Joined:           time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
				Expires:          time.Date(2024, 2, 2, 2, 2, 2, 2, time.UTC),
				Reason:           "original reason",
				AddedBy:          "original dummy",
				IneligibleStatus: "new ineligible",
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preserved, member := populateMemberFields(clockwork.NewFakeClockAt(test.currentTime), test.username, test.oldMember, test.newMember)
			require.Equal(t, test.wantPreserved, preserved)
			require.Empty(t, cmp.Diff(test.want, member, cmpOpts...))
		})
	}
}

func TestCanUpdateMembership(t *testing.T) {
	c := initSvc(t)

	tests := []struct {
		name      string
		userCtx   context.Context
		oldMember *accesslist.AccessListMember
		newMember *accesslist.AccessListMember
		wantErr   require.ErrorAssertionFunc
	}{
		{
			name:    "owner adds a new user",
			userCtx: genUserContext(context.Background(), ownerUser, nil, nil),
			newMember: member(t, header.Metadata{Name: "new-user"}, accesslist.AccessListMemberSpec{
				Name:             "new-user",
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: require.NoError,
		},
		{
			name:    "owner modifies a different user",
			userCtx: genUserContext(context.Background(), ownerUser, nil, nil),
			oldMember: member(t, header.Metadata{Name: "new-user"}, accesslist.AccessListMemberSpec{
				Name:             "new-user",
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			newMember: member(t, header.Metadata{Name: "new-user"}, accesslist.AccessListMemberSpec{
				Name:             "new-user",
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Expires:          time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: require.NoError,
		},
		{
			name:    "owner adds itself",
			userCtx: genUserContext(context.Background(), ownerUser, nil, nil),
			newMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:    "owner modifies itself",
			userCtx: genUserContext(context.Background(), ownerUser, nil, nil),
			oldMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			newMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "modified access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:    "owner doesn't modify itself",
			userCtx: genUserContext(context.Background(), ownerUser, nil, nil),
			oldMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			newMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			authCtx, err := c.svc.authorizer.Authorize(test.userCtx)
			require.NoError(t, err)
			username, err := getUsername(authCtx)
			require.NoError(t, err)

			test.wantErr(t, c.svc.canUpdateMembership(ctx, authCtx, username, test.oldMember, test.newMember))
		})
	}
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

func createReviews(ctx context.Context, t *testing.T, svc *Service, emitter *eventstest.ChannelEmitter, usageEvents *usageEventsClient,
	reviews []*accesslist.Review,
) {
	t.Helper()

	user, err := authz.UserFromContext(ctx)
	require.NoError(t, err)
	username := user.GetIdentity().Username

	for _, review := range reviews {
		// Get the original access list.
		accessList, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{
			Name: review.Spec.AccessList,
		})
		require.NoError(t, err)

		resp, err := svc.CreateAccessListReview(ctx, &accesslistv1.CreateAccessListReviewRequest{
			Review: conv.ToReviewProto(review),
		})
		require.NoError(t, err)

		// Calculate the expected number of days past.
		expectedDaysPast := int32(svc.clock.Now().Sub(accessList.Spec.Audit.NextAuditDate.AsTime()).Hours() / 24)

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

		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
			require.Equal(t, review.Spec.AccessList, event.AccessListReviewCreate.Metadata.Id)
			require.Equal(t, expectedDaysPast, event.AccessListReviewCreate.DaysPastNextAuditDate)
			require.Equal(t, review.Spec.Changes.MembershipRequirementsChanged != nil, event.AccessListReviewCreate.MembershipRequirementsChanged)
			require.Equal(t, review.Spec.Changes.ReviewFrequencyChanged.String() != "", event.AccessListReviewCreate.ReviewFrequencyChanged)
			require.Equal(t, review.Spec.Changes.ReviewDayOfMonthChanged.String() != "", event.AccessListReviewCreate.ReviewDayOfMonthChanged)
			require.Equal(t, int32(len(review.Spec.Changes.RemovedMembers)), event.AccessListReviewCreate.NumberOfRemovedMembers)
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
	// Default to an access list with the next audit date 1 year in the future.
	return newAccessListWithNextAuditDate(t, name, clock.Now().Add(time.Hour*24*365))
}

func newAccessListWithNextAuditDate(t *testing.T, name string, nextAuditDate time.Time) *accesslist.AccessList {
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
				{
					Name:        testUserDenyWhere,
					Description: "deny where user",
				},
				{
					Name:        testUserDenyAll,
					Description: "deny where user",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: nextAuditDate,
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

func ptrToUint32(val uint32) *uint32 {
	return &val
}

func TestAccessListEqual(t *testing.T) {
	tests := []struct {
		name      string
		first     *accesslist.AccessList
		second    *accesslist.AccessList
		wantEqual bool
	}{
		{
			name:      "both nil",
			first:     nil,
			second:    nil,
			wantEqual: true,
		},
		{
			name: "nil and empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles:  []string{},
						Traits: map[string][]string{},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles:  nil,
						Traits: nil,
					},
				},
			},
			wantEqual: true,
		},
		{
			name: "nil and no empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles: []string{"role1"},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles: nil,
					},
				},
			},
			wantEqual: false,
		},
		{
			name: "nil and no empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Traits: map[string][]string{"trait1": {"value1"}},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{},
				},
			},
			wantEqual: false,
		},
		{
			name: "ephemeral fields",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Owners: []accesslist.Owner{
						{
							IneligibleStatus: "ineligible",
						},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Owners: []accesslist.Owner{
						{
							IneligibleStatus: "not-ineligible",
						},
					},
				},
			},
			wantEqual: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := accessListEqual(tc.first, tc.second)
			require.Equal(t, tc.wantEqual, got)
		})
	}
}
