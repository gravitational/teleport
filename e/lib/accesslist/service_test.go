package accesslist

import (
	"context"
	"errors"
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
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
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
	externalMember1   = "externalMember1"
	externalMember2   = "membeexternalMember2r3"

	testDisplayNameTrait = "displayName"
	testEmailTrait       = "email"

	scopeTeamA = "/team-a"
	scopeTeamB = "/team-b"
)

// cmpOpts are general cmpOpts for all comparisons.
var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(accesslist.Status{}, "CurrentUserAssignments", "UserAssignments", "OwnerDisplays"),
	cmpopts.IgnoreFields(accesslist.AccessListMember{}, "Status"),
	cmpopts.SortSlices(func(a, b *accesslist.AccessList) bool {
		return a.GetName() < b.GetName()
	}),
	cmpopts.SortSlices(func(a, b *accesslist.Review) bool {
		return a.GetName() < b.GetName()
	}),
}

func TestService_GetAccessLists(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	// a3 will have different ownership requirements.
	a3 := newAccessList(t, "3", c.clock)
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	// a4 will have a label attached.
	a4 := newAccessList(t, "4", c.clock)
	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a4.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	a1.Status.MemberCount = ptrToUint32(2)
	a1.Status.MemberListCount = ptrToUint32(0)
	a2.Status.MemberCount = ptrToUint32(1)
	a2.Status.MemberListCount = ptrToUint32(0)
	a3.Status.MemberCount = ptrToUint32(2)
	a3.Status.MemberListCount = ptrToUint32(0)
	a4.Status.MemberCount = ptrToUint32(0)
	a4.Status.MemberListCount = ptrToUint32(0)

	getResp, err = c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4}, mustFromProtoAll(t, getResp.GetAccessLists()...), cmpOpts...))

	// owner should only see a1 and a2
	getResp, err = c.svc.GetAccessLists(c.ownerCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2}, mustFromProtoAll(t, getResp.GetAccessLists()...), cmpOpts...))

	// userDenyWhereWhere shouldn't see anything even though it's an owner
	_, err = c.svc.GetAccessLists(c.userDenyAllCtx, &accesslistv1.GetAccessListsRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// member can see the access lists they belong to without counts.
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	a1.Status.MemberCount = nil
	a1.Status.MemberListCount = nil
	a3.Status.MemberCount = nil
	a3.Status.MemberListCount = nil
	getResp, err = c.svc.GetAccessLists(memberCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3}, mustFromProtoAll(t, getResp.GetAccessLists()...), cmpOpts...))

	// userWhere can only see a4
	getResp, err = c.svc.GetAccessLists(c.userWhereCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a4}, mustFromProtoAll(t, getResp.GetAccessLists()...), cmpOpts...))
}

func TestService_ScopedAccessLists_ListFiltering(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Create same-named resources across scopes to catch collisions.
	teamA := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "shared"), c.clock)
	// This child list is intentionally owned by the caller too. List methods
	// use identity-based default scope filters, and a pinned user defaults to
	// their exact pinned scope unless they request a broader filter. So this
	// one should be filtered out by default.
	teamAChild := newScopedAccessList(t, scopedAccessListName(scopeTeamA+"/child", "child"), c.clock)
	// This is in an orthogonal scope and a caller pinned to scopeTeamA should
	// never see it.
	teamB := newScopedAccessList(t, scopedAccessListName(scopeTeamB, "shared"), c.clock)
	unscoped := newAccessList(t, "shared", c.clock)
	createAccessLists(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{teamA, teamAChild, teamB, unscoped})

	// An unscoped caller should only see the unscoped list by default.
	unscopedResp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{}.Build())
	require.NoError(t, err)
	require.Equal(t, []accesslists.NormalizedSQN{{Name: unscoped.GetName()}}, accessListSQNsFromProto(unscopedResp.GetAccessLists()))

	// An unscoped caller can see all lists by passing an explicit filter.
	unscopedResp, err = c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_ALL,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Equal(t,
		[]accesslists.NormalizedSQN{
			accesslists.ScopeQualifiedName(unscoped),
			accesslists.ScopeQualifiedName(teamA),
			accesslists.ScopeQualifiedName(teamAChild),
			accesslists.ScopeQualifiedName(teamB),
		},
		accessListSQNsFromProto(unscopedResp.GetAccessLists()),
	)

	// A scoped caller will only see lists in their exact scope by default.
	scopedOwnerCtx := genScopedUserContext(t.Context(), ownerUser, scopeTeamA)
	resp, err := c.svc.ListAccessListsV2(scopedOwnerCtx, accesslistv1.ListAccessListsV2Request_builder{}.Build())
	require.NoError(t, err)
	require.Equal(t,
		[]accesslists.NormalizedSQN{accesslists.ScopeQualifiedName(teamA)},
		accessListSQNsFromProto(resp.GetAccessLists()),
	)

	// The scoped caller can pass an explicit filter to request all lists, but
	// they are still restricted by their scope pin.
	resp, err = c.svc.ListAccessListsV2(scopedOwnerCtx, accesslistv1.ListAccessListsV2Request_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_ALL,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Equal(t,
		[]accesslists.NormalizedSQN{
			accesslists.ScopeQualifiedName(teamA),
			accesslists.ScopeQualifiedName(teamAChild),
		},
		accessListSQNsFromProto(resp.GetAccessLists()),
	)
}

func TestService_ScopedAccessLists_SameNameUpdateDeleteScopeIsolation(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Updating and deleting one scoped list must not affect a same-name list in
	// another scope.
	teamA := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "shared"), c.clock)
	teamA.Spec.Title = "team a"
	teamB := newScopedAccessList(t, scopedAccessListName(scopeTeamB, "shared"), c.clock)
	teamB.Spec.Title = "team b"
	createAccessLists(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{teamA, teamB})

	teamA.Spec.Title = "team a updated"
	_, err := c.svc.UpdateAccessList(c.userCtx, accesslistv1.UpdateAccessListRequest_builder{AccessList: conv.ToProto(teamA)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
		require.Equal(t, teamA.GetName(), event.Name)
		require.Equal(t, scopeTeamA, event.Scope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, teamA.GetName(), event.AccessListUpdate.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListUpdate.Metadata.Scope)
	})

	gotTeamA := getAccessListV2(t, c, accesslists.ScopeQualifiedName(teamA))
	require.Equal(t, "team a updated", gotTeamA.GetSpec().GetTitle())
	gotTeamB := getAccessListV2(t, c, accesslists.ScopeQualifiedName(teamB))
	require.Equal(t, "team b", gotTeamB.GetSpec().GetTitle())

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{
		Scope: scopeTeamA,
		Name:  "shared",
	}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.Equal(t, teamA.GetName(), event.Name)
		require.Equal(t, scopeTeamA, event.Scope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, teamA.GetName(), event.AccessListDelete.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListDelete.Metadata.Scope)
	})

	_, err = c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Scope: scopeTeamA, Name: "shared"}.Build())
	require.ErrorAs(t, err, new(*trace.NotFoundError))
	gotTeamB = getAccessListV2(t, c, accesslists.ScopeQualifiedName(teamB))
	require.Equal(t, "team b", gotTeamB.GetSpec().GetTitle())
}

func TestService_ScopedAccessListMembers_SameParentNameScopeIsolation(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Member operations must key by parent list scope and name, not just parent
	// list name.
	teamA := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "shared"), c.clock)
	teamB := newScopedAccessList(t, scopedAccessListName(scopeTeamB, "shared"), c.clock)
	createAccessLists(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{teamA, teamB})

	teamAMember := newScopedAccessListMember(t, accesslists.ScopeQualifiedName(teamA), accesslists.NormalizedSQN{Name: member1}, accesslist.MembershipKindUser, c.clock)
	teamBMember := newScopedAccessListMember(t, accesslists.ScopeQualifiedName(teamB), accesslists.NormalizedSQN{Name: member2}, accesslist.MembershipKindUser, c.clock)
	_, err := c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(teamAMember)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamA, event.AccessListMemberMetadata.AccessListScope)
		require.Equal(t, member1, event.AccessListMemberMetadata.Members[0].MemberName)
		require.Empty(t, event.AccessListMemberMetadata.Members[0].MemberScope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListMemberCreate.Metadata.Scope)
	})
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(teamBMember)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.Equal(t, teamB.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamB, event.AccessListMemberMetadata.AccessListScope)
		require.Equal(t, member2, event.AccessListMemberMetadata.Members[0].MemberName)
		require.Empty(t, event.AccessListMemberMetadata.Members[0].MemberScope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, teamB.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, scopeTeamB, event.AccessListMemberCreate.Metadata.Scope)
	})

	teamAMembers := listAccessListMembersV2(t, c, accesslists.ScopeQualifiedName(teamA))
	require.Equal(t, []string{member1}, memberNames(teamAMembers))
	teamBMembers := listAccessListMembersV2(t, c, accesslists.ScopeQualifiedName(teamB))
	require.Equal(t, []string{member2}, memberNames(teamBMembers))

	_, err = c.svc.GetAccessListMember(c.userCtx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessListScope: scopeTeamB,
		AccessList:      "shared",
		MemberName:      member1,
	}.Build())
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	teamAMember.Spec.Reason = "updated"
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(teamAMember)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamA, event.AccessListMemberMetadata.AccessListScope)
		require.Equal(t, member1, event.AccessListMemberMetadata.Members[0].MemberName)
		require.Empty(t, event.AccessListMemberMetadata.Members[0].MemberScope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberUpdate.Metadata.Id)
		require.Equal(t, teamA.GetScope(), event.AccessListMemberUpdate.Metadata.Scope)
		require.Equal(t, member1, event.AccessListMemberUpdate.MemberMetadata.Name)
		require.Empty(t, event.AccessListMemberUpdate.MemberMetadata.Scope)
	})

	_, err = c.svc.DeleteAccessListMember(c.userCtx, accesslistv1.DeleteAccessListMemberRequest_builder{
		AccessListScope: scopeTeamA,
		AccessList:      teamA.GetName(),
		MemberName:      teamAMember.GetName(),
	}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamA, event.AccessListMemberMetadata.AccessListScope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, teamA.GetName(), event.AccessListMemberDelete.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListMemberDelete.Metadata.Scope)
	})

	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{
		AccessListScope: scopeTeamB,
		AccessList:      teamB.GetName(),
	}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.Equal(t, teamB.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamB, event.AccessListMemberMetadata.AccessListScope)
	})
}

func TestService_UpsertAccessListWithMembers_ScopedNestedMemberNamesDoNotCollide(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Nested list members can share a name if they're in different scopes.
	// Make sure UpsertAccessListWithMembers handles this properly.
	parent := newScopedAccessList(t, scopedAccessListName(scopeTeamA+"/child", "parent"), c.clock)
	teamAGroup := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "group"), c.clock)
	teamAChildGroup := newScopedAccessList(t, scopedAccessListName(scopeTeamA+"/child", "group"), c.clock)
	createAccessLists(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{teamAGroup, teamAChildGroup})

	expectedMemberNames := []accesslists.NormalizedSQN{
		accesslists.ScopeQualifiedName(teamAGroup),
		accesslists.ScopeQualifiedName(teamAChildGroup),
	}

	memberTeamAGroup := newScopedAccessListMember(t, accesslists.ScopeQualifiedName(parent), accesslists.ScopeQualifiedName(teamAGroup), accesslist.MembershipKindScopedList, c.clock)
	memberTeamAChildGroup := newScopedAccessListMember(t, accesslists.ScopeQualifiedName(parent), accesslists.ScopeQualifiedName(teamAChildGroup), accesslist.MembershipKindScopedList, c.clock)
	resp, err := c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(parent),
		Members: []*accesslistv1.Member{
			conv.ToMemberProto(memberTeamAGroup),
			conv.ToMemberProto(memberTeamAChildGroup),
		},
	}.Build())
	require.NoError(t, err)
	require.Len(t, resp.GetMembers(), 2)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.Equal(t, parent.GetName(), event.Name)
		require.Equal(t, parent.GetScope(), event.Scope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, parent.GetName(), event.AccessListCreate.Metadata.Id)
		require.Equal(t, parent.GetScope(), event.AccessListCreate.Metadata.Scope)
	})
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.Equal(t, parent.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, parent.GetScope(), event.AccessListMemberMetadata.AccessListScope)

		memberNames := make([]accesslists.NormalizedSQN, 0, len(event.AccessListMemberMetadata.Members))
		for _, member := range event.AccessListMemberMetadata.Members {
			memberNames = append(memberNames, accesslists.NormalizedSQN{Name: member.MemberName, Scope: member.MemberScope})
		}
		require.ElementsMatch(t, expectedMemberNames, memberNames)
	})
	createdUsageMemberNames := make([]accesslists.NormalizedSQN, 0, len(expectedMemberNames))
	for range expectedMemberNames {
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
			require.Equal(t, parent.GetName(), event.AccessListMemberCreate.Metadata.Id)
			require.Equal(t, parent.GetScope(), event.AccessListMemberCreate.Metadata.Scope)
			createdUsageMemberNames = append(createdUsageMemberNames, accesslists.NormalizedSQN{
				Name:  event.AccessListMemberCreate.MemberMetadata.Name,
				Scope: event.AccessListMemberCreate.MemberMetadata.Scope,
			})
		})
	}
	require.ElementsMatch(t, expectedMemberNames, createdUsageMemberNames)

	resp, err = c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(parent),
		Members: []*accesslistv1.Member{
			conv.ToMemberProto(newScopedAccessListMember(t, accesslists.ScopeQualifiedName(parent), accesslists.ScopeQualifiedName(teamAGroup), accesslist.MembershipKindScopedList, c.clock)),
			conv.ToMemberProto(newScopedAccessListMember(t, accesslists.ScopeQualifiedName(parent), accesslists.ScopeQualifiedName(teamAChildGroup), accesslist.MembershipKindScopedList, c.clock)),
		},
	}.Build())
	require.NoError(t, err)
	require.Len(t, resp.GetMembers(), 2)
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
		require.Equal(t, parent.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, parent.GetScope(), event.AccessListMemberMetadata.AccessListScope)

		memberNames := make([]accesslists.NormalizedSQN, 0, len(event.AccessListMemberMetadata.Members))
		for _, member := range event.AccessListMemberMetadata.Members {
			memberNames = append(memberNames, accesslists.NormalizedSQN{Name: member.MemberName, Scope: member.MemberScope})
		}
		require.ElementsMatch(t, expectedMemberNames, memberNames)
	})
	updatedUsageMemberNames := make([]accesslists.NormalizedSQN, 0, len(expectedMemberNames))
	for range expectedMemberNames {
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
			require.Equal(t, parent.GetName(), event.AccessListMemberUpdate.Metadata.Id)
			require.Equal(t, parent.GetScope(), event.AccessListMemberUpdate.Metadata.Scope)
			updatedUsageMemberNames = append(updatedUsageMemberNames, accesslists.NormalizedSQN{
				Name:  event.AccessListMemberUpdate.MemberMetadata.Name,
				Scope: event.AccessListMemberUpdate.MemberMetadata.Scope,
			})
		})
	}
	require.ElementsMatch(t, expectedMemberNames, updatedUsageMemberNames)

	storedMembers := listAccessListMembersV2(t, c, accesslists.ScopeQualifiedName(parent))
	require.ElementsMatch(t,
		[]string{
			accesslists.ScopeQualifiedName(teamAGroup).String(),
			accesslists.ScopeQualifiedName(teamAChildGroup).String(),
		},
		memberNames(storedMembers),
	)
}

func TestService_OwnerScopePinIsolation(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// A caller pinned to a specific scope should not be able to manage lists
	// outside that scope, even if they are an owner.
	scopedOwnerCtx := genScopedUserContext(t.Context(), ownerUser, scopeTeamA)
	teamAGroup := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "group"), c.clock)
	teamAChild := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "child"), c.clock)
	teamBGroup := newScopedAccessList(t, scopedAccessListName(scopeTeamB, "group"), c.clock)
	createAccessLists(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{teamAGroup, teamAChild, teamBGroup})

	// Can only read the list in the caller's pinned scope.
	_, err := c.svc.GetAccessList(scopedOwnerCtx, accesslistv1.GetAccessListRequest_builder{
		Scope: scopeTeamA,
		Name:  "group",
	}.Build())
	require.NoError(t, err)
	_, err = c.svc.GetAccessList(scopedOwnerCtx, accesslistv1.GetAccessListRequest_builder{
		Scope: scopeTeamB,
		Name:  "group",
	}.Build())
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// Can only read members of a list in the caller's pinned scope.
	_, err = c.svc.ListAccessListMembers(scopedOwnerCtx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessListScope: scopeTeamA,
		AccessList:      "group",
	}.Build())
	require.NoError(t, err)
	_, err = c.svc.ListAccessListMembers(scopedOwnerCtx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessListScope: scopeTeamB,
		AccessList:      "group",
	}.Build())
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// Can only add a member to a list in the caller's pinned scope.
	teamAGroupMember := newScopedAccessListMember(t,
		accesslists.ScopeQualifiedName(teamAGroup), accesslists.ScopeQualifiedName(teamAChild), accesslist.MembershipKindScopedList, c.clock)
	_, err = c.svc.UpsertAccessListMember(scopedOwnerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(teamAGroupMember),
	}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.Equal(t, teamAGroup.GetName(), event.AccessListMemberMetadata.AccessListName)
		require.Equal(t, scopeTeamA, event.AccessListMemberMetadata.AccessListScope)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, teamAGroup.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListMemberCreate.Metadata.Scope)
	})
	_, err = c.svc.UpsertAccessListMember(scopedOwnerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(newScopedAccessListMember(t,
			accesslists.ScopeQualifiedName(teamBGroup), accesslists.NormalizedSQN{Name: "user"}, accesslist.MembershipKindUser, c.clock)),
	}.Build())
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// Can only review a list in the caller's pinned scope.
	review := newScopedAccessListReview(t, accesslists.ScopeQualifiedName(teamAGroup))
	review.Spec.Changes.ScopedRemovedMembers = []string{accesslists.ScopeQualifiedName(teamAChild).String()}
	reviewResp, err := c.svc.CreateAccessListReview(scopedOwnerCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review),
	}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.Equal(t, teamAGroup.GetName(), event.Name)
		require.Equal(t, scopeTeamA, event.Scope)
		require.Equal(t, review.Spec.Changes.ScopedRemovedMembers, event.ScopedRemovedMembers)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, teamAGroup.GetName(), event.AccessListReviewCreate.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListReviewCreate.Metadata.Scope)
		require.Equal(t, int32(1), event.AccessListReviewCreate.NumberOfRemovedMembers)
	})
	_, err = c.svc.CreateAccessListReview(scopedOwnerCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(newScopedAccessListReview(t, accesslists.ScopeQualifiedName(teamBGroup))),
	}.Build())
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	_, err = c.svc.DeleteAccessListReview(c.userCtx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListScope: scopeTeamA,
		AccessListName:  teamAGroup.GetName(),
		ReviewName:      reviewResp.GetReviewName(),
	}.Build())
	require.NoError(t, err)
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewDelete) {
		require.Equal(t, teamAGroup.GetName(), event.AccessListReviewDelete.Metadata.Id)
		require.Equal(t, scopeTeamA, event.AccessListReviewDelete.Metadata.Scope)
	})
}

func TestService_ListAccessLists(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	accessLists := listAccessLists(c.userCtx, t, c.svc, 1)
	require.Empty(t, accessLists)
	accessLists = listAccessListsV2(c.userCtx, t, c.svc, 1)
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

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a5m1 := newAccessListMember(t, a5.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a5m2 := newAccessListMember(t, a5.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, []*accesslist.AccessListMember{
			a1m1, a1m2, a2m1, a3m1, a3m2, a4m1, a4m2, a5m1, a5m2,
		})

	a1.Status.MemberCount = ptrToUint32(2)
	a1.Status.MemberListCount = ptrToUint32(0)
	a2.Status.MemberCount = ptrToUint32(1)
	a2.Status.MemberListCount = ptrToUint32(0)
	a3.Status.MemberCount = ptrToUint32(2)
	a3.Status.MemberListCount = ptrToUint32(0)
	a4.Status.MemberCount = ptrToUint32(2)
	a4.Status.MemberListCount = ptrToUint32(0)
	a5.Status.MemberCount = ptrToUint32(2)
	a5.Status.MemberListCount = ptrToUint32(0)
	a6.Status.MemberCount = ptrToUint32(0)
	a6.Status.MemberListCount = ptrToUint32(0)

	accessLists = listAccessLists(c.userCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(c.userCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, accessLists, cmpOpts...))

	// owner should only see a1, a2, a4, a5
	accessLists = listAccessLists(c.ownerCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(c.ownerCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))

	// userDenyWhere should only see a1, a2, a4, a5
	accessLists = listAccessLists(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4, a5}, accessLists, cmpOpts...))

	// Add a label that should be denied to a5
	a5.SetStaticLabels(map[string]string{
		"denied": "true",
	})
	_, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{
		AccessList: conv.ToProto(a5),
	}.Build())
	require.NoError(t, err)

	// userDenyWhere should no longer see a5
	accessLists = listAccessLists(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(c.userDenyWhereCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a4}, accessLists, cmpOpts...))

	a1.Status.MemberCount = nil
	a1.Status.MemberListCount = nil
	a3.Status.MemberCount = nil
	a3.Status.MemberListCount = nil
	a4.Status.MemberCount = nil
	a4.Status.MemberListCount = nil
	a5.Status.MemberCount = nil
	a5.Status.MemberListCount = nil

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	accessLists = listAccessLists(memberCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(memberCtx, t, c.svc, 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	// Use the page size defaults
	accessLists = listAccessLists(memberCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(memberCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	accessLists = listAccessLists(memberCtx, t, c.svc, -1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(memberCtx, t, c.svc, -1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a3, a4, a5}, accessLists, cmpOpts...))

	// User where should only see a6
	accessLists = listAccessLists(c.userWhereCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a6}, accessLists, cmpOpts...))
	accessLists = listAccessListsV2(c.userWhereCtx, t, c.svc, 0)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a6}, accessLists, cmpOpts...))
}

func TestService_ListAccessListsV2OwnerDisplaySearch(t *testing.T) {
	c := initSvc(t)

	owner, err := c.testEnv.identity.GetUser(t.Context(), ownerUser, false)
	require.NoError(t, err)
	owner.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
		"okta/email":       {"jane@example.com"},
	})
	_, err = c.testEnv.identity.UpdateUser(t.Context(), owner)
	require.NoError(t, err)

	janeList := newAccessList(t, "primary-prod", c.clock)
	janeList.Spec.Title = "prod database"
	janeList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUser},
	}
	legacyJaneList := newAccessList(t, "legacy-prod", c.clock)
	legacyJaneList.Spec.Title = "prod database"
	legacyJaneList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUnspecified},
	}
	otherList := newAccessList(t, "other-prod", c.clock)
	otherList.Spec.Title = "prod database"
	otherList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser2, MembershipKind: accesslist.MembershipKindUser},
	}
	// create a nested owner list to ensure that the search resolver can handle nested ownership.
	nestedOwner := newAccessList(t, ownerUser, c.clock)
	nestedOwner.Spec.Title = "owner list"
	nestedOwner.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser2, MembershipKind: accesslist.MembershipKindUser},
	}
	nestedOwnerList := newAccessList(t, "nested-owner-prod", c.clock)
	nestedOwnerList.Spec.Title = "prod database"
	nestedOwnerList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindList},
	}

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{janeList, legacyJaneList, otherList, nestedOwner, nestedOwnerList}, nil)

	testCases := []struct {
		name   string
		search string
	}{
		{
			name:   "display primary",
			search: "Jane",
		},
		{
			name:   "display secondary",
			search: "jane@example.com",
		},
		{
			name:   "mixed field and display terms",
			search: "prod Garcia",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
				PageSize: 100,
				Filter: accesslistv1.AccessListsFilter_builder{
					Search: tc.search,
				}.Build(),
			}.Build())
			require.NoError(t, err)
			// Only the lists owned by the specified user should be returned.
			// The other lists owned by either other users or nested lists should not be returned.
			gotNames := make([]string, 0, len(resp.GetAccessLists()))
			for _, accessList := range resp.GetAccessLists() {
				gotNames = append(gotNames, accessList.GetHeader().GetMetadata().GetName())
			}
			require.ElementsMatch(t, []string{janeList.GetName(), legacyJaneList.GetName()}, gotNames)
		})
	}
}

func TestService_ListAccessListsV2StoredFieldSearchDoesNotResolveOwners(t *testing.T) {
	var cache *observingAccessListCache
	c := initSvc(t, withCacheWrap(func(inner Cache) Cache {
		cache = &observingAccessListCache{Cache: inner}
		return cache
	}))

	first := newAccessList(t, "first-prod", c.clock)
	first.Spec.Title = "prod database"
	first.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUser},
	}
	first.Spec.Grants.Roles = []string{"db-access"}
	first.SetOrigin(types.OriginOkta)
	second := newAccessList(t, "second-prod", c.clock)
	second.Spec.Title = "prod database"
	second.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUser},
	}
	second.Spec.Grants.Roles = []string{"db-access"}
	second.SetOrigin(types.OriginOkta)
	_, err := c.testEnv.accessLists.UpsertAccessList(c.userCtx, first)
	require.NoError(t, err)
	_, err = c.testEnv.accessLists.UpsertAccessList(c.userCtx, second)
	require.NoError(t, err)

	req := accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 1,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "prod",
			Owners: []string{ownerUser},
			Roles:  []string{"db"},
			Origin: types.OriginOkta,
		}.Build(),
	}.Build()
	_, err = c.svc.ListAccessListsV2(c.userCtx, req)
	require.NoError(t, err)

	require.Zero(t, cache.listUsersCalls)
	require.NotEmpty(t, cache.listAccessListsRequests)
	for _, observed := range cache.listAccessListsRequests {
		require.Empty(t, observed.search)
		require.Equal(t, []string{ownerUser}, observed.owners)
		require.Equal(t, []string{"db"}, observed.roles)
		require.Equal(t, types.OriginOkta, observed.origin)
	}
	require.Equal(t, int32(1), req.GetPageSize())
	require.Empty(t, req.GetPageToken())
	require.Equal(t, "prod", req.GetFilter().GetSearch())
}

func TestService_ListAccessListsV2OwnerDisplaySearchResolvesOnlyConsultedTerms(t *testing.T) {
	var cache *observingAccessListCache
	c := initSvc(t, withCacheWrap(func(inner Cache) Cache {
		cache = &observingAccessListCache{Cache: inner}
		return cache
	}))

	accessList := newAccessList(t, "production", c.clock)
	accessList.Spec.Title = "prod database"
	accessList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUser},
	}
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{accessList}, nil)

	resp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 100,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "missing-one missing-two missing-three",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, resp.GetAccessLists())
	require.Equal(t, 1, cache.listUsersCalls)
	require.Len(t, cache.listAccessListsRequests, 1)
	require.Empty(t, cache.listAccessListsRequests[0].search)
}

func TestService_ListAccessListsV2OwnerDisplaySearchMemoizesAcrossCachePages(t *testing.T) {
	var cache *observingAccessListCache
	c := initSvc(t, withCacheWrap(func(inner Cache) Cache {
		cache = &observingAccessListCache{Cache: inner}
		return cache
	}))

	owner, err := c.testEnv.identity.GetUser(t.Context(), ownerUser, false)
	require.NoError(t, err)
	owner.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})
	_, err = c.testEnv.identity.UpdateUser(t.Context(), owner)
	require.NoError(t, err)

	var accessLists []*accesslist.AccessList
	for i, ownerName := range []string{ownerUser2, ownerUser2, ownerUser, ownerUser2, ownerUser} {
		accessList := newAccessList(t, fmt.Sprintf("%02d-prod", i), c.clock)
		accessList.Spec.Title = "prod database"
		accessList.Spec.Owners = []accesslist.Owner{
			{Name: ownerName, MembershipKind: accesslist.MembershipKindUser},
		}
		accessLists = append(accessLists, accessList)
	}
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, accessLists, nil)

	resp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 1,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "prod Garcia Garcia",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 1)
	require.NotEmpty(t, resp.GetNextPageToken())
	require.Equal(t, ownerUser, resp.GetAccessLists()[0].GetSpec().GetOwners()[0].GetName())

	require.Equal(t, 1, cache.listUsersCalls)
	require.Greater(t, len(cache.listAccessListsRequests), 1)
	for _, observed := range cache.listAccessListsRequests {
		require.Empty(t, observed.search)
	}
}

func TestService_ListAccessListsV2OwnerDisplaySearchPagination(t *testing.T) {
	c := initSvc(t)

	owner, err := c.testEnv.identity.GetUser(t.Context(), ownerUser, false)
	require.NoError(t, err)
	owner.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})
	_, err = c.testEnv.identity.UpdateUser(t.Context(), owner)
	require.NoError(t, err)

	var accessLists []*accesslist.AccessList
	for name, ownerName := range map[string]string{
		"00-jane":  ownerUser,
		"01-other": ownerUser2,
		"02-jane":  ownerUser,
		"03-other": ownerUser2,
	} {
		accessList := newAccessList(t, name, c.clock)
		accessList.Spec.Owners = []accesslist.Owner{
			{Name: ownerName, MembershipKind: accesslist.MembershipKindUser},
		}
		accessLists = append(accessLists, accessList)
	}
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, accessLists, nil)

	firstPage, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 1,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "Garcia",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, firstPage.GetAccessLists(), 1)
	require.Equal(t, "00-jane", firstPage.GetAccessLists()[0].GetHeader().GetMetadata().GetName())
	require.NotEmpty(t, firstPage.GetNextPageToken())

	secondPage, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize:  1,
		PageToken: firstPage.GetNextPageToken(),
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "Garcia",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, secondPage.GetAccessLists(), 1)
	require.Equal(t, "02-jane", secondPage.GetAccessLists()[0].GetHeader().GetMetadata().GetName())
	require.Empty(t, secondPage.GetNextPageToken())
}

func TestService_ListAccessListsV2OwnerDisplaySearchLookupFailure(t *testing.T) {
	cacheErr := errors.New("backend unavailable")
	c := initSvc(t, withCacheWrap(func(inner Cache) Cache {
		return &listUsersErrCache{Cache: inner, err: cacheErr}
	}))

	owner, err := c.testEnv.identity.GetUser(t.Context(), ownerUser, false)
	require.NoError(t, err)
	owner.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})
	_, err = c.testEnv.identity.UpdateUser(t.Context(), owner)
	require.NoError(t, err)

	accessList := newAccessList(t, "primary-prod", c.clock)
	accessList.Spec.Title = "prod database"
	accessList.Spec.Owners = []accesslist.Owner{
		{Name: ownerUser, MembershipKind: accesslist.MembershipKindUser},
	}
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{accessList}, nil)

	resp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 100,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "Garcia",
		}.Build(),
	}.Build())
	// Expect that the search resolver will fail gracefully and return an empty list of access lists.
	require.NoError(t, err)
	require.Empty(t, resp.GetAccessLists())

	resp, err = c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{
		PageSize: 100,
		Filter: accesslistv1.AccessListsFilter_builder{
			Search: "prod",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 1)
}

type listUsersErrCache struct {
	Cache
	err error
}

func (c *listUsersErrCache) ListUsers(context.Context, *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	return nil, c.err
}

type observedListAccessListsRequest struct {
	search string
	owners []string
	roles  []string
	origin string
}

type observingAccessListCache struct {
	Cache
	listUsersCalls          int
	listAccessListsRequests []observedListAccessListsRequest
}

func (c *observingAccessListCache) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	c.listUsersCalls++
	return c.Cache.ListUsers(ctx, req)
}

func (c *observingAccessListCache) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	c.listAccessListsRequests = append(c.listAccessListsRequests, observedListAccessListsRequest{
		search: req.GetFilter().GetSearch(),
		owners: append([]string(nil), req.GetFilter().GetOwners()...),
		roles:  append([]string(nil), req.GetFilter().GetRoles()...),
		origin: req.GetFilter().GetOrigin(),
	})
	return c.Cache.ListAccessListsV2(ctx, req)
}

func TestService_ListAccessLists_CurrentUserAssignments(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a1m := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m})

	// memberCtx meets the membership requirements (mrole1/2, mtrait1/2)
	// and is a member of a1 but not a2.
	memberCtx := genUserContext(t.Context(), member1, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	assertAssignments := func(t *testing.T, accessLists []*accesslistv1.AccessList, desc string) {
		t.Helper()
		for _, al := range accessLists {
			require.NotNil(t, al.GetStatus().GetCurrentUserAssignments(),
				"%s: CurrentUserAssignments should be populated (list %s)", desc, al.GetHeader().GetMetadata().GetName())
		}
	}

	for _, tc := range []struct {
		name    string
		listV1  func() []*accesslistv1.AccessList
		listV2  func() []*accesslistv1.AccessList
		checkFn func(t *testing.T, accessLists []*accesslistv1.AccessList)
	}{
		{
			name: "RBAC user gets assignments populated",
			listV1: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessLists(c.userCtx, accesslistv1.ListAccessListsRequest_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			listV2: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessListsV2(c.userCtx, accesslistv1.ListAccessListsV2Request_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			checkFn: func(t *testing.T, accessLists []*accesslistv1.AccessList) {
				assertAssignments(t, accessLists, "RBAC user")
				require.Len(t, accessLists, 2)
			},
		},
		{
			name: "non-RBAC owner gets explicit ownership",
			listV1: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessLists(c.ownerCtx, accesslistv1.ListAccessListsRequest_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			listV2: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessListsV2(c.ownerCtx, accesslistv1.ListAccessListsV2Request_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			checkFn: func(t *testing.T, accessLists []*accesslistv1.AccessList) {
				assertAssignments(t, accessLists, "owner")
				for _, al := range accessLists {
					require.Equal(t,
						accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT,
						al.GetStatus().GetCurrentUserAssignments().GetOwnershipType(),
						"ownerUser should have EXPLICIT ownership on list %s", al.GetHeader().GetMetadata().GetName())
				}
			},
		},
		{
			name: "non-RBAC member gets explicit membership",
			listV1: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessLists(memberCtx, accesslistv1.ListAccessListsRequest_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			listV2: func() []*accesslistv1.AccessList {
				resp, err := c.svc.ListAccessListsV2(memberCtx, accesslistv1.ListAccessListsV2Request_builder{PageSize: 100}.Build())
				require.NoError(t, err)
				return resp.GetAccessLists()
			},
			checkFn: func(t *testing.T, accessLists []*accesslistv1.AccessList) {
				assertAssignments(t, accessLists, "member")
				// member1 is a member of a1 only.
				require.Len(t, accessLists, 1)
				require.Equal(t,
					accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT,
					accessLists[0].GetStatus().GetCurrentUserAssignments().GetMembershipType(),
					"member1 should have EXPLICIT membership")
			},
		},
	} {
		t.Run(tc.name+"/v1", func(t *testing.T) {
			tc.checkFn(t, tc.listV1())
		})
		t.Run(tc.name+"/v2", func(t *testing.T) {
			tc.checkFn(t, tc.listV2())
		})
	}
}

func listAccessLists(ctx context.Context, t *testing.T, svc *Service, pageSize int) []*accesslist.AccessList {
	t.Helper()

	accessLists, err := stream.Collect(
		stream.FilterMap(
			clientutils.Resources(ctx, func(ctx context.Context, pageSize int, token string) ([]*accesslistv1.AccessList, string, error) {
				resp, err := svc.ListAccessLists(ctx, accesslistv1.ListAccessListsRequest_builder{
					PageSize:  int32(pageSize),
					NextToken: token,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				return resp.GetAccessLists(), resp.GetNextToken(), nil
			}), func(accessList *accesslistv1.AccessList) (*accesslist.AccessList, bool) {
				out, err := conv.FromProto(accessList)
				if err != nil {
					return nil, false
				}
				return out, true
			}),
	)
	require.NoError(t, err)
	return accessLists
}

func listAccessListsV2(ctx context.Context, t *testing.T, svc *Service, pageSize int) []*accesslist.AccessList {
	t.Helper()

	accessLists, err := stream.Collect(
		stream.FilterMap(
			clientutils.Resources(ctx, func(ctx context.Context, pageSize int, token string) ([]*accesslistv1.AccessList, string, error) {
				resp, err := svc.ListAccessListsV2(ctx, accesslistv1.ListAccessListsV2Request_builder{
					PageSize:  int32(pageSize),
					PageToken: token,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				return resp.GetAccessLists(), resp.GetNextPageToken(), nil
			}), func(accessList *accesslistv1.AccessList) (*accesslist.AccessList, bool) {
				out, err := conv.FromProto(accessList)
				if err != nil {
					return nil, false
				}
				return out, true
			}),
	)
	require.NoError(t, err)
	return accessLists
}

func TestService_UpsertAccessList(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessList(t, "5", c.clock)

	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a5.SetOrigin(types.OriginOkta)

	upsertResp, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a1)}.Build())
	require.NoError(t, err)
	// The upsert response carries the caller's assignments like the read paths do.
	require.NotNil(t, upsertResp.GetStatus().GetCurrentUserAssignments())
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a1.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// User tries to create a new access list that they own. Shouldn't work.
	_, err = c.svc.UpsertAccessList(c.ownerCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a2)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a2)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a2.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// Owner cannot make any modifications
	a2.Spec.Audit.NextAuditDate = c.clock.Now().AddDate(100, 0, 0)
	_, err = c.svc.UpsertAccessList(c.ownerCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a2)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Admin can make modifications
	a2.Spec.Audit.NextAuditDate = c.clock.Now().AddDate(100, 0, 0)
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a2)}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a2.GetName(), event.AccessListUpdate.Metadata.Id)
	})

	// UserWhere cannot upsert a3
	_, err = c.svc.UpsertAccessList(c.userWhereCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a3)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// UserWhere can upsert a4
	_, err = c.svc.UpsertAccessList(c.userWhereCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a4)}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a4.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// Even RBAC users can't create Okta sourced lists.
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a5)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Okta user can, though
	oktaIdentity := authtest.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	_, err = c.svc.UpsertAccessList(oktaUserCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a5)}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, a5.GetName(), event.AccessListCreate.Metadata.Id)
	})

	// RBAC users can't update Okta sourced lists either.
	a5.Spec.Grants.Roles = append(a5.Spec.Grants.Roles, "new-role")
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a5)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Okta user can do that too.
	_, err = c.svc.UpsertAccessList(oktaUserCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a5)}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(event *apievents.AccessListUpdate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListUpdate) {
		require.Equal(t, a5.GetName(), event.AccessListUpdate.Metadata.Id)
	})

	// Cannot explicitly set the memberOf or ownerOf fields.
	a6 := newAccessList(t, "6", c.clock)
	a6.Status.MemberOf = []string{"test"}

	resp, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a6)}.Build())
	require.NoError(t, err)
	require.Equal(t, []string{}, resp.GetStatus().GetMemberOf())

	a6.Status.MemberOf = nil
	a6.Status.OwnerOf = []string{"test"}

	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a6)}.Build())
	require.NoError(t, err)
	require.Equal(t, []string{}, resp.GetStatus().GetOwnerOf())
}

func TestService_GetAccessList(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	eligibleOwnersWithStatus := []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "owner user",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindUnspecified,
		},
		{
			Name:             ownerUser2,
			Description:      "owner user 2",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindUnspecified,
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
			MembershipKind:   accesslist.MembershipKindUser,
		},
	}

	// a4 will have a label attached.
	a4 := newAccessList(t, "4", c.clock)
	a4.SetStaticLabels(map[string]string{
		"test-label": "test",
	})
	a4.Spec.Owners = a3.Spec.Owners
	a4.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	a1.Status.MemberCount = ptrToUint32(2)
	a1.Status.MemberListCount = ptrToUint32(0)
	a2.Status.MemberCount = ptrToUint32(1)
	a2.Status.MemberListCount = ptrToUint32(0)
	a3.Status.MemberCount = ptrToUint32(2)
	a3.Status.MemberListCount = ptrToUint32(0)
	a4.Status.MemberCount = ptrToUint32(0)
	a4.Status.MemberListCount = ptrToUint32(0)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a3m1, a3m2})

	get, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a2.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	get, err = c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a4.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a4, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	// member2 can't see a2
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1.Status.MemberCount = nil
	a1.Status.MemberListCount = nil

	get, err = c.svc.GetAccessList(memberCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	_, err = c.svc.GetAccessList(memberCtx, accesslistv1.GetAccessListRequest_builder{Name: a2.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	a3.Status.MemberCount = nil
	a3.Status.MemberListCount = nil

	get, err = c.svc.GetAccessList(memberCtx, accesslistv1.GetAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	// Owner can see the member counts.
	a1.Status.MemberCount = ptrToUint32(2)
	a1.Status.MemberListCount = ptrToUint32(0)

	get, err = c.svc.GetAccessList(c.ownerCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	get, err = c.svc.GetAccessList(c.ownerCtx, accesslistv1.GetAccessListRequest_builder{Name: a2.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))

	// owner can't see a3
	_, err = c.svc.GetAccessList(c.ownerCtx, accesslistv1.GetAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// userDenyWhere can't see a2
	_, err = c.svc.GetAccessList(c.userDenyWhereCtx, accesslistv1.GetAccessListRequest_builder{Name: a2.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// userDenyWhere gets not found for non-existent list
	_, err = c.svc.GetAccessList(c.userDenyWhereCtx, accesslistv1.GetAccessListRequest_builder{Name: "non-existent-list"}.Build())
	require.True(t, trace.IsNotFound(err))

	// userWhere can only see a4
	_, err = c.svc.GetAccessList(c.userWhereCtx, accesslistv1.GetAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	get, err = c.svc.GetAccessList(c.userWhereCtx, accesslistv1.GetAccessListRequest_builder{Name: a4.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a4, mustFromProto(t, get, conv.WithOwnersIneligibleStatusField(get.GetSpec().GetOwners())), cmpOpts...))
}

func TestService_GetInheritedGrants(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{a1}, nil)

	// Admin can get inherited grants for an existing access list.
	resp, err := c.svc.GetInheritedGrants(c.userCtx, accesslistv1.GetInheritedGrantsRequest_builder{AccessListId: a1.GetName()}.Build())
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Non-member, non-owner, non-admin is denied.
	nonPrivCtx := genUserContext(t.Context(), member3, []string{"mrole1", "mrole2"}, nil)
	_, err = c.svc.GetInheritedGrants(nonPrivCtx, accesslistv1.GetInheritedGrantsRequest_builder{AccessListId: a1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Non-existent access list returns not found.
	_, err = c.svc.GetInheritedGrants(c.userCtx, accesslistv1.GetInheritedGrantsRequest_builder{AccessListId: "does-not-exist"}.Build())
	require.True(t, trace.IsNotFound(err))

	// This case covers a regression where the cache returns a non-nil error
	// which wasn't correctly propagated to the caller.
	cacheErr := errors.New("cache error")
	c2 := initSvc(t, withCacheWrap(func(inner Cache) Cache {
		return &cacheGetErrWrapper{Cache: inner, acl: a1, getErr: cacheErr}
	}))
	_, err = c2.svc.GetInheritedGrants(c2.userCtx, accesslistv1.GetInheritedGrantsRequest_builder{AccessListId: a1.GetName()}.Build())
	require.ErrorIs(t, err, cacheErr)
}

// cacheGetErrWrapper wraps a Cache and overrides GetAccessList to return a fixed (acl, err) pair,
// simulating a cache that returns a result together with a non-nil error.
type cacheGetErrWrapper struct {
	Cache
	acl    *accesslist.AccessList
	getErr error
}

func (c *cacheGetErrWrapper) GetAccessList(_ context.Context, _ string) (*accesslist.AccessList, error) {
	return c.acl, c.getErr
}

func (c *cacheGetErrWrapper) GetAccessListV2(_ context.Context, _ *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	return c.acl, c.getErr
}

func TestService_GetAccessListsToReview(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	c := initSvc(t, withClock(clock))

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	resp, err := c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.GetAccessLists())

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessList(t, "5", c.clock)
	a7 := newAccessListWithPartialSpec(t, "7", c.clock.Now().Add(time.Hour*24*365), accesslist.Spec{
		Owners: []accesslist.Owner{
			{Name: ownerUser, Description: "owner user", MembershipKind: accesslist.MembershipKindUser},
			{Name: ownerUser2, Description: "owner user 2", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyWhere, Description: "deny where user", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyAll, Description: "deny where user", MembershipKind: accesslist.MembershipKindUser},
		},
	})
	a6 := newAccessListWithPartialSpec(t, "6", time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), accesslist.Spec{
		Owners: []accesslist.Owner{
			{
				Name:           a7.GetName(),
				MembershipKind: accesslist.MembershipKindList,
			},
		},
	})
	nonReviewable1 := newAccessList(t, "8", c.clock, withType(accesslist.Static))
	nonReviewable2 := newAccessList(t, "9", c.clock, withType(accesslist.SCIM))
	// Since ownership is inherited by members of sub-lists, add ownerCtx as member of a7.
	a7m1 := newAccessListMember(t, a7.GetName(), ownerUser, accesslist.MembershipKindUser, c.clock)

	a1.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	a2.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a3.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a4.Spec.Audit.NextAuditDate = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	a5.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	// Provide a7 before a6 since a6 depends on a7 for ownership relationship.
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{a1, a2, a3, a4, a5, a7, a6, nonReviewable1, nonReviewable2}, []*accesslist.AccessListMember{a7m1})

	setDate(clock, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.GetAccessLists())

	setDate(clock, time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5, a6}, mustFromProtoAll(t, resp.GetAccessLists()...), cmpOpts...))

	setDate(clock, time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a5, a6}, mustFromProtoAll(t, resp.GetAccessLists()...), cmpOpts...))
	setDate(clock, time.Date(2024, 2, 16, 0, 0, 0, 0, time.UTC))

	resp, err = c.svc.GetAccessListsToReview(c.ownerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3, a4, a5, a6}, mustFromProtoAll(t, resp.GetAccessLists()...), cmpOpts...))
}

func TestService_GetAccessListsToReview_ScopePin(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	c := initSvc(t, withClock(clock))

	// Review listing should respect the caller's scope pin even when the caller
	// owns same-name reviewable lists in multiple scopes.
	teamA := newScopedAccessList(t, scopedAccessListName(scopeTeamA, "review-me"), c.clock)
	teamB := newScopedAccessList(t, scopedAccessListName(scopeTeamB, "review-me"), c.clock)
	teamA.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	teamB.Spec.Audit.NextAuditDate = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	createAccessLists(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{teamA, teamB})

	scopedOwnerCtx := genScopedUserContext(t.Context(), ownerUser, scopeTeamA)
	setDate(clock, time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC))

	resp, err := c.svc.GetAccessListsToReview(scopedOwnerCtx, &accesslistv1.GetAccessListsToReviewRequest{})
	require.NoError(t, err)
	require.Equal(t, []accesslists.NormalizedSQN{accesslists.ScopeQualifiedName(teamA)}, accessListSQNsFromProto(resp.GetAccessLists()))
}

func TestService_UpsertAndGetAccessList_OwnersIneligibleReason(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	// Create an access list, with varying eligbility for owners.
	a1 := newAccessList(t, "1", c.clock)
	a1.Spec.Owners = []accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "OK existing user",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)],
			MembershipKind:   accesslist.MembershipKindUser,
		},
		{
			Name:             member1,
			Description:      "NOK ownermemship_requires does not match",
			IneligibleStatus: accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)],
			MembershipKind:   accesslist.MembershipKindUser,
		},
	}

	// Test that owner's ineligible status got stripped before upsertion.
	createdAccessList, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a1)}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]accesslist.Owner{
		{
			Name:             ownerUser,
			Description:      "OK existing user",
			IneligibleStatus: "",
			MembershipKind:   accesslist.MembershipKindUser,
		},
		{
			Name:             member1,
			Description:      "NOK ownermemship_requires does not match",
			IneligibleStatus: "",
			MembershipKind:   accesslist.MembershipKindUser,
		},
	}, mustFromProto(t, createdAccessList).GetOwners(), cmpOpts...))

	// Check retrieved access list owners has determined the ineligible status field.
	getAccessList, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1.Spec.Owners, mustFromProto(t, getAccessList, conv.WithOwnersIneligibleStatusField(getAccessList.GetSpec().GetOwners())).GetOwners(), cmpOpts...))
}

func TestService_GetAccessListOwnerDisplays(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	const (
		displayOwnerName = "display-owner"
		emptyOwnerName   = "empty-owner"
		missingOwnerName = "missing-owner"
		listOwnerName    = "list-owner"
	)
	createDisplayUser(t, c.testEnv.identity, displayOwnerName, "Display Owner", "display-owner@example.com")
	createDisplayUser(t, c.testEnv.identity, emptyOwnerName, "", "")
	_, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(newAccessList(t, listOwnerName, c.clock))}.Build())
	require.NoError(t, err)

	a1 := newAccessList(t, "1", c.clock)
	a1.Spec.Owners = []accesslist.Owner{
		{Name: displayOwnerName, MembershipKind: accesslist.MembershipKindUser},
		{Name: emptyOwnerName, MembershipKind: accesslist.MembershipKindUser},
		{Name: missingOwnerName, MembershipKind: accesslist.MembershipKindUser},
		{Name: listOwnerName, MembershipKind: accesslist.MembershipKindList},
	}
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a1)}.Build())
	require.NoError(t, err)

	getAccessList, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)

	require.Equal(t, map[string]*accesslistv1.UserDisplay{
		displayOwnerName: accesslistv1.UserDisplay_builder{Primary: "Display Owner", Secondary: "display-owner@example.com"}.Build(),
		emptyOwnerName:   {},
	}, getAccessList.GetStatus().GetOwnerDisplays())

	storedAccessList, err := c.testEnv.accessLists.GetAccessList(c.userCtx, a1.GetName())
	require.NoError(t, err)
	require.Empty(t, storedAccessList.Status.OwnerDisplays)
}

func TestService_UpsertAndGetAccessList_MembersIneligibleReason(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	// Create an access list.
	a1 := newAccessList(t, "1", c.clock)
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a1)}.Build())
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
			MembershipKind:   accesslist.MembershipKindUser,
		},
	)
	require.NoError(t, err)

	membersToCreate := []*accesslist.AccessListMember{
		// NOK member is expired
		member_expired,
		// OK member
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), member2, c.clock, accesslist.MembershipKindUser, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE)]),
		// NOK membership_requires does not match
		newAccessListMemberWithIneligibleReason(t, a1.GetName(), ownerUser, c.clock, accesslist.MembershipKindUser, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS)]),
	}

	// Test that member's ineligible status got stripped before upsertion.
	for _, member := range membersToCreate {
		upsertedMember, err := c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(member)}.Build())
		require.NoError(t, err)
		require.Empty(t, upsertedMember.GetSpec().GetIneligibleStatus())
	}

	// Check retrieved members list has determined the ineligible status field.
	getMembers, err := c.svc.ListAccessListMembers(c.userCtx, accesslistv1.ListAccessListMembersRequest_builder{PageSize: 0, AccessList: a1.GetName()}.Build())
	require.NoError(t, err)

	var members []*accesslist.AccessListMember
	for _, member := range getMembers.GetMembers() {
		members = append(members, mustFromMemberProto(t, member, conv.WithMemberIneligibleStatusField(member)))
	}
	require.Empty(t, cmp.Diff(membersToCreate, members, cmpOpts...))
}

func TestService_ListAccessListMembersUserDisplays(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	const (
		displayMemberName = "display-member"
		emptyMemberName   = "empty-member"
		missingMemberName = "missing-member"
		listMemberName    = "list-member"
		adderName         = "display-adder"
		missingAdderName  = "missing-adder"
	)
	createDisplayUser(t, c.testEnv.identity, displayMemberName, "Display Member", "display-member@example.com")
	createDisplayUser(t, c.testEnv.identity, emptyMemberName, "", "")
	createDisplayUser(t, c.testEnv.identity, adderName, "Display Adder", "display-adder@example.com")
	_, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(newAccessList(t, listMemberName, c.clock))}.Build())
	require.NoError(t, err)

	a1 := newAccessList(t, "1", c.clock)
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(a1)}.Build())
	require.NoError(t, err)

	membersToCreate := []*accesslist.AccessListMember{
		newAccessListMember(t, a1.GetName(), displayMemberName, accesslist.MembershipKindUser, c.clock),
		newAccessListMember(t, a1.GetName(), emptyMemberName, accesslist.MembershipKindUser, c.clock),
		newAccessListMember(t, a1.GetName(), missingMemberName, accesslist.MembershipKindUser, c.clock),
		newAccessListMember(t, a1.GetName(), listMemberName, accesslist.MembershipKindList, c.clock),
	}
	membersToCreate[0].Spec.AddedBy = adderName
	membersToCreate[1].Spec.AddedBy = missingAdderName
	membersToCreate[2].Spec.AddedBy = missingAdderName
	membersToCreate[3].Spec.AddedBy = adderName

	for _, member := range membersToCreate {
		_, err := c.testEnv.accessLists.UpsertAccessListMember(c.userCtx, member)
		require.NoError(t, err)
	}

	getMembers, err := c.svc.ListAccessListMembers(c.userCtx, accesslistv1.ListAccessListMembersRequest_builder{PageSize: 0, AccessList: a1.GetName()}.Build())
	require.NoError(t, err)

	membersByName := make(map[string]*accesslistv1.Member, len(getMembers.GetMembers()))
	for _, member := range getMembers.GetMembers() {
		membersByName[member.GetSpec().GetName()] = member
	}

	require.Equal(t, accesslistv1.UserDisplay_builder{Primary: "Display Member", Secondary: "display-member@example.com"}.Build(), membersByName[displayMemberName].GetStatus().GetDisplay())
	require.Equal(t, accesslistv1.UserDisplay_builder{Primary: "Display Adder", Secondary: "display-adder@example.com"}.Build(), membersByName[displayMemberName].GetStatus().GetAddedByDisplay())
	require.Equal(t, &accesslistv1.UserDisplay{}, membersByName[emptyMemberName].GetStatus().GetDisplay())
	require.Nil(t, membersByName[emptyMemberName].GetStatus().GetAddedByDisplay())
	require.Nil(t, membersByName[missingMemberName].GetStatus().GetDisplay())
	require.Nil(t, membersByName[missingMemberName].GetStatus().GetAddedByDisplay())
	require.Nil(t, membersByName[listMemberName].GetStatus().GetDisplay())
	require.Equal(t, accesslistv1.UserDisplay_builder{Primary: "Display Adder", Secondary: "display-adder@example.com"}.Build(), membersByName[listMemberName].GetStatus().GetAddedByDisplay())

	storedMember, err := c.testEnv.accessLists.GetAccessListMember(c.userCtx, a1.GetName(), displayMemberName)
	require.NoError(t, err)
	require.Nil(t, storedMember.Status)
}

func TestService_UpsertAccessListWithMembersUserDisplays(t *testing.T) {
	t.Parallel()
	c := initSvc(t, withDisabledReconcilers())

	const (
		displayOwnerName  = "display-owner"
		displayMemberName = "display-member"
	)
	createDisplayUser(t, c.testEnv.identity, displayOwnerName, "Display Owner", "display-owner@example.com")
	createDisplayUser(t, c.testEnv.identity, displayMemberName, "Display Member", "display-member@example.com")
	// Freshly added members get AddedBy set to the calling user (testUser).
	createDisplayUser(t, c.testEnv.identity, testUser, "Test User", "test-user@example.com")

	a1 := newAccessList(t, "1", c.clock)
	a1.Spec.Owners = []accesslist.Owner{
		{Name: displayOwnerName, MembershipKind: accesslist.MembershipKindUser},
	}
	member := newAccessListMember(t, a1.GetName(), displayMemberName, accesslist.MembershipKindUser, c.clock)

	resp, err := c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(a1),
		Members:    conv.ToMembersProto([]*accesslist.AccessListMember{member}),
	}.Build())
	require.NoError(t, err)

	// Owner displays ride the response access list status (AC-11b).
	require.Equal(t, map[string]*accesslistv1.UserDisplay{
		displayOwnerName: accesslistv1.UserDisplay_builder{Primary: "Display Owner", Secondary: "display-owner@example.com"}.Build(),
	}, resp.GetAccessList().GetStatus().GetOwnerDisplays())

	// Member and added_by displays ride each response member status.
	require.Len(t, resp.GetMembers(), 1)
	memberStatus := resp.GetMembers()[0].GetStatus()
	require.Equal(t, accesslistv1.UserDisplay_builder{Primary: "Display Member", Secondary: "display-member@example.com"}.Build(), memberStatus.GetDisplay())
	require.Equal(t, accesslistv1.UserDisplay_builder{Primary: "Test User", Secondary: "test-user@example.com"}.Build(), memberStatus.GetAddedByDisplay())

	// Displays are response-only and never persisted.
	storedAccessList, err := c.testEnv.accessLists.GetAccessList(c.userCtx, a1.GetName())
	require.NoError(t, err)
	require.Empty(t, storedAccessList.Status.OwnerDisplays)
	storedMember, err := c.testEnv.accessLists.GetAccessListMember(c.userCtx, a1.GetName(), displayMemberName)
	require.NoError(t, err)
	require.Nil(t, storedMember.Status)
}

func TestService_DeleteAccessList(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	getResp, err := c.svc.GetAccessLists(c.userCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.GetAccessLists())

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a3.SetOrigin(types.OriginOkta)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2}, nil)

	a1.Status.MemberCount = ptrToUint32(0)
	a1.Status.MemberListCount = ptrToUint32(0)
	a2.Status.MemberCount = ptrToUint32(0)
	a2.Status.MemberListCount = ptrToUint32(0)

	get, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	_, err = c.svc.DeleteAccessList(c.userWhereCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a1.GetName(), event.AccessListDelete.Metadata.Id)
	})

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a1.GetName()}.Build())
	require.True(t, trace.IsNotFound(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	_, err = c.svc.DeleteAccessList(c.userWhereCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a2.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a2.GetName(), event.AccessListDelete.Metadata.Id)
	})

	// Delete non existent access list if auth err is non nill
	_, err = c.svc.DeleteAccessList(c.userDenyWhereCtx, accesslistv1.DeleteAccessListRequest_builder{Name: "non-existent access list"}.Build())
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	// Delete Okta-originated Access List with no plugin backing it.
	createAccessLists(t, c.oktaSvcCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a3})

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a3.GetName(), event.AccessListDelete.Metadata.Id)
	})

	// Delete Okta-originated Access List as regular user with plugin sync enabled.
	createAccessLists(t, c.oktaSvcCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a3})
	err = c.svc.plugins.CreatePlugin(t.Context(), newTestOktaPlugin(t, true))
	require.NoError(t, err)

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))
	expectEvent(t, events.AccessListDeleteFailureCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.False(t, event.Success)
	})

	// Delete Okta-originated Access List as Okta service role with plugin sync enabled.
	_, err = c.svc.DeleteAccessList(c.oktaSvcCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a3.GetName(), event.AccessListDelete.Metadata.Id)
	})

	// Delete Okta-originated Access List as regular user with plugin sync disabled.
	createAccessLists(t, c.oktaSvcCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a3})

	oktaPlugin, err := c.svc.plugins.GetPlugin(t.Context(), "okta", true)
	require.NoError(t, err)

	oktaPluginV1, ok := oktaPlugin.(*types.PluginV1)
	require.True(t, ok)

	oktaPluginV1.Spec.GetOkta().SyncSettings.SyncAccessLists = false

	_, err = c.svc.plugins.UpdatePlugin(t.Context(), oktaPluginV1)
	require.NoError(t, err)

	_, err = c.svc.DeleteAccessList(c.userCtx, accesslistv1.DeleteAccessListRequest_builder{Name: a3.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListDeleteSuccessCode, c.emitter, func(event *apievents.AccessListDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListDelete) {
		require.Equal(t, a3.GetName(), event.AccessListDelete.Metadata.Id)
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

func (a *fakeAuth) UpdateRole(ctx context.Context, r types.Role) (types.Role, error) {
	return &types.RoleV6{}, nil
}

func (a *fakeAuth) CreateRole(ctx context.Context, r types.Role) (types.Role, error) {
	return &types.RoleV6{}, nil
}

func (a *fakeAuth) UpsertRole(ctx context.Context, r types.Role) (types.Role, error) {
	return &types.RoleV6{}, nil
}

func (a *fakeAuth) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	return &types.ListResourcesResponse{}, nil
}

func (a *fakeAuth) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return &accesslist.AccessList{}, nil
}

func (a *fakeAuth) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	return []*accesslist.AccessList{}, nil
}

func (a *fakeAuth) GetAccessListMember(ctx context.Context, accessListName string, memberName string) (*accesslist.AccessListMember, error) {
	return &accesslist.AccessListMember{}, nil
}

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
	access      services.Access
}

type testSvcComponents struct {
	userCtx          context.Context
	userWhereCtx     context.Context
	userDenyWhereCtx context.Context
	userDenyAllCtx   context.Context
	ownerCtx         context.Context
	svc              *Service
	clock            clockwork.Clock
	emitter          *eventstest.ChannelEmitter
	usageEvents      *usageEventsClient
	usageReporter    *usageReporter
	testEnv          *testEnvironment
	oktaSvcCtx       context.Context
}

type testSvcOptions struct {
	disabledReconcilers bool
	clock               clockwork.Clock
	cacheWrapFn         func(Cache) Cache
}

type svcOpts func(*testSvcOptions)

func withDisabledReconcilers() svcOpts {
	return func(o *testSvcOptions) {
		o.disabledReconcilers = true
	}
}

func withClock(clock clockwork.Clock) svcOpts {
	return func(o *testSvcOptions) {
		o.clock = clock
	}
}

func withCacheWrap(fn func(Cache) Cache) svcOpts {
	return func(o *testSvcOptions) {
		o.cacheWrapFn = fn
	}
}

func initSvc(t *testing.T, opts ...svcOpts) testSvcComponents {
	options := testSvcOptions{
		clock: clockwork.NewFakeClock(),
	}
	for _, opt := range opts {
		opt(&options)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	clock := options.clock
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc, err := local.NewIdentityService(backend)
	require.NoError(t, err)
	storage, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: backend,
		Modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
				Cloud: true,
			},
		},
		ScopesFeatures:              scopes.Features{Enabled: true},
		RunWhileLockedRetryInterval: -1 * time.Millisecond,
	})
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
	scopedAccessService := local.NewScopedAccessService(backend)
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

	authorizer, err := authz.NewScopedAuthorizer(authz.AuthorizerOpts{
		ClusterName:      "test-cluster",
		AccessPoint:      accessPoint,
		LockWatcher:      lockWatcher,
		ScopedRoleReader: scopedAccessService,
		ScopesFeatures:   scopes.Features{Enabled: true},
	})
	require.NoError(t, err)

	clt := client{
		Access:        accessService,
		Identity:      userSvc,
		AccessLists:   storage,
		EventsService: eventService,
	}

	role, err := authtest.CreateRole(ctx, clt, "access-lists", types.RoleSpecV6{
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

	roleWhere, err := authtest.CreateRole(ctx, clt, "access-lists-where", types.RoleSpecV6{
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

	roleDenyWhere, err := authtest.CreateRole(ctx, clt, "access-lists-deny-where", types.RoleSpecV6{
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

	roleDenyAll, err := authtest.CreateRole(ctx, clt, "access-lists-deny-all", types.RoleSpecV6{
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

	_, err = authtest.CreateRole(ctx, clt, "mrole1", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, clt, "mrole2", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, clt, "orole1", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, clt, "orole2", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, clt, "noprole", types.RoleSpecV6{})
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
	var svcCache Cache = &clt
	if options.cacheWrapFn != nil {
		svcCache = options.cacheWrapFn(svcCache)
	}
	svc, err := NewService(
		t.Context(),
		ServiceConfig{
			Authorizer:         authorizer,
			AccessLists:        storage,
			LockGetter:         locks,
			AccessListReviews:  storage,
			Plugins:            local.NewPluginsService(backend),
			Emitter:            emitter,
			UsageEvents:        usageEvents,
			UsageReporter:      usageReporter,
			Clock:              clock,
			Cache:              svcCache,
			AuthServer:         &fakeAuth{},
			Backend:            backend,
			disableReconcilers: options.disabledReconcilers,
			Modules:            modulestest.EnterpriseModules(),
		})
	require.NoError(t, err)

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
		oktaSvcCtx: authz.ContextWithUser(ctx, authz.BuiltinRole{
			Role:     types.RoleOkta,
			Username: "okta",
			Identity: tlsca.Identity{
				Groups: []string{string(types.RoleOkta)},
			},
		}),
		svc:           svc,
		clock:         clock,
		emitter:       emitter,
		usageEvents:   usageEvents,
		usageReporter: usageReporter,
		testEnv: &testEnvironment{
			identity:    userSvc,
			accessLists: storage,
			access:      accessService,
		},
	}
}

func setDate(clock *clockwork.FakeClock, date time.Time) {
	now := clock.Now()
	clock.Advance(date.Sub(now))
}

func TestService_CountAccessListMembers(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a1m3 := newAccessListMember(t, a1.GetName(), member3, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a3m3 := newAccessListMember(t, a3.GetName(), a4.GetName(), accesslist.MembershipKindList, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a1m3, a2m1, a3m1, a3m2, a3m3})

	resp, err := c.svc.CountAccessListMembers(c.userCtx, accesslistv1.CountAccessListMembersRequest_builder{AccessListName: a1.GetName()}.Build())
	require.NoError(t, err)
	require.Equal(t, uint32(3), resp.GetCount())
	require.Equal(t, uint32(0), resp.GetListCount())

	resp, err = c.svc.CountAccessListMembers(c.ownerCtx, accesslistv1.CountAccessListMembersRequest_builder{AccessListName: a2.GetName()}.Build())
	require.NoError(t, err)
	require.Equal(t, uint32(1), resp.GetCount())
	require.Equal(t, uint32(0), resp.GetListCount())

	resp, err = c.svc.CountAccessListMembers(c.userCtx, accesslistv1.CountAccessListMembersRequest_builder{AccessListName: a3.GetName()}.Build())
	require.NoError(t, err)
	require.Equal(t, uint32(2), resp.GetCount())
	require.Equal(t, uint32(1), resp.GetListCount())

	resp, err = c.svc.CountAccessListMembers(c.userCtx, accesslistv1.CountAccessListMembersRequest_builder{AccessListName: a4.GetName()}.Build())
	require.NoError(t, err)
	require.Equal(t, uint32(0), resp.GetCount())
	require.Equal(t, uint32(0), resp.GetListCount())

	memberCtx := genUserContext(context.Background(), member1, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	_, err = c.svc.CountAccessListMembers(memberCtx, accesslistv1.CountAccessListMembersRequest_builder{AccessListName: a1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))
}

func TestService_ListAccessListMembers(t *testing.T) {
	t.Parallel()
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

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2, a4m1, a4m2})

	// Admin should be able to list everything
	members := getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m1, a1m2}, members, cmpOpts...))

	// owner should be able to see members for a2
	members = getAccessListMembers(c.ownerCtx, t, c.svc, a2.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a2m1, a2m2}, members, cmpOpts...))

	// userDenyWhere should not be able to see members for a2
	_, err := c.svc.ListAccessListMembers(c.userDenyWhereCtx, accesslistv1.ListAccessListMembersRequest_builder{
		PageSize:   0,
		PageToken:  "",
		AccessList: a2.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// owner should not be able to see members for a3
	_, err = c.svc.ListAccessListMembers(c.ownerCtx, accesslistv1.ListAccessListMembersRequest_builder{
		PageSize:   0,
		PageToken:  "",
		AccessList: a3.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should not be able to see members for a2
	_, err = c.svc.ListAccessListMembers(c.userWhereCtx, accesslistv1.ListAccessListMembersRequest_builder{
		PageSize:   0,
		PageToken:  "",
		AccessList: a2.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should be able to see members for a4
	members = getAccessListMembers(c.userWhereCtx, t, c.svc, a4.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a4m1, a4m2}, members, cmpOpts...))

	// Not authorized users get an access denied error
	_, err = getAllAccessListMembers(t.Context(), c.svc)
	require.True(t, trace.IsAccessDenied(err))

	// Authorized users get all members visible members
	allMembers, err := getAllAccessListMembers(c.userCtx, c.svc)
	require.NoError(t, err)
	expectedAllMembers := []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2, a4m1, a4m2}
	require.Empty(t, cmp.Diff(expectedAllMembers, allMembers, cmpOpts...))
}

func TestService_GetAccessListMember(t *testing.T) {
	t.Parallel()
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

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// Admin should be able to get members
	member, err := c.svc.GetAccessListMember(c.userCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m1, mustFromMemberProto(t, member), cmpOpts...))

	// owner should be able to see members for a1
	member, err = c.svc.GetAccessListMember(c.ownerCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m2.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1m2, mustFromMemberProto(t, member), cmpOpts...))

	// userDenyWhere should not be able to see members for a1
	_, err = c.svc.GetAccessListMember(c.userDenyWhereCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m2.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// userWhere should be able to see members for a2
	member, err = c.svc.GetAccessListMember(c.userWhereCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a2.GetName(), MemberName: a2m2.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2m2, mustFromMemberProto(t, member), cmpOpts...))
}

func TestService_GetStaticAccessListMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	staticAccessList := newAccessList(t, "test-acl-1", c.clock, withType(accesslist.Static))
	defaultAccessList := newAccessList(t, "test-acl-3", c.clock)
	scimAccessList := newAccessList(t, "test-acl-4", c.clock, withType(accesslist.SCIM))

	accessLists := []*accesslist.AccessList{
		staticAccessList, defaultAccessList, scimAccessList,
	}

	createAccessLists(t, c.userCtx, c.svc, c.emitter, nil, accessLists)

	members := []*accesslist.AccessListMember{}

	for _, accessList := range accessLists {
		members = append(members, newAccessListMember(t, accessList.GetName(), member1, accesslist.MembershipKindUser, c.clock))
		members = append(members, newAccessListMember(t, accessList.GetName(), member2, accesslist.MembershipKindUser, c.clock))
	}

	for _, member := range members {
		// Add members directly in the backend to bypass service validation
		_, err := c.testEnv.accessLists.UpsertAccessListMember(c.userCtx, member)
		require.NoError(t, err)
	}

	t.Run("getting member of non-static access_list fails", func(t *testing.T) {
		nonStaticAccessLists := []*accesslist.AccessList{
			defaultAccessList, scimAccessList,
		}
		for _, accessList := range nonStaticAccessLists {
			t.Run(accessList.GetName(), func(t *testing.T) {
				for _, m := range []string{member1, member2} {
					_, err := c.svc.GetStaticAccessListMember(c.userCtx, accesslistv1.GetStaticAccessListMemberRequest_builder{
						AccessList: accessList.GetMetadata().Name,
						MemberName: m,
					}.Build())
					require.Error(t, err, "member = %q", m)
					require.True(t, isNonStaticAccessList(err), "member = %q", m)
					require.True(t, trace.IsBadParameter(err), "member = %q", m)
				}
			})
		}
	})

	t.Run("getting member of static access_list succeeds", func(t *testing.T) {
		for _, m := range []string{member1, member2} {
			_, err := c.svc.GetStaticAccessListMember(c.userCtx, accesslistv1.GetStaticAccessListMemberRequest_builder{
				AccessList: staticAccessList.GetMetadata().Name,
				MemberName: m,
			}.Build())
			require.NoError(t, err)
		}
	})
}

type client struct {
	services.Access
	services.Identity
	services.AccessLists
	*local.EventsService
}

func TestService_UpsertAccessListMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	var err error

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	scimList := newAccessList(t, "scim", c.clock, withType(accesslist.SCIM))

	// create two lists without membership/ownership reqs to test nested membership checks
	a5 := newAccessListWithPartialSpec(t, "5", c.clock.Now().Add(time.Hour*24*365), accesslist.Spec{
		Owners: []accesslist.Owner{
			{Name: testUser, Description: "test user", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyWhere, Description: "test user deny where", MembershipKind: accesslist.MembershipKindUser},
		},
	})
	a6 := newAccessListWithPartialSpec(t, "6", c.clock.Now().Add(time.Hour*24*365), accesslist.Spec{
		Owners: []accesslist.Owner{
			{Name: testUser, Description: "test user", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyWhere, Description: "test user deny where", MembershipKind: accesslist.MembershipKindUser},
		},
	})

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	for _, list := range []*accesslist.AccessList{a1, a2, a3, a4, a5, a6, scimList} {
		_, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(list)}.Build())
		require.NoError(t, err)
		expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
			require.True(t, event.Success)
		})
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
			require.Equal(t, list.GetName(), event.AccessListCreate.Metadata.Id)
		})
	}

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)

	require.Equal(t, testUser, a1m1.Spec.AddedBy)

	_, err = c.svc.UpsertAccessListMember(c.userWhereCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(a1m1)}.Build())
	require.True(t, trace.IsAccessDenied(err))

	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)

	_, err = c.svc.UpsertAccessListMember(c.userWhereCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(a2m1)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), a2m1.Spec.MembershipKind)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a2.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, a2m1.Spec.MembershipKind, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
	})

	got, err := c.svc.UpsertAccessListMember(c.ownerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(a1m1)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), a1m1.Spec.MembershipKind)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, a1m1.Spec.MembershipKind, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
	})

	// Added by should be overridden from "test-user" to "owner-user"
	want := a1m1
	want.Spec.AddedBy = ownerUser

	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	got, err = c.svc.GetAccessListMember(c.userCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	// Update from admin should still retain the old added by, reason, and joined
	oldJoined := a1m1.Spec.Joined
	oldReason := a1m1.Spec.Reason
	want.Spec.Reason = "some new reason"
	want.Spec.Joined = c.clock.Now().Add(time.Hour * 24)
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(a1m1)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), a1m1.Spec.MembershipKind)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberUpdate.Metadata.Id)
		require.Equal(t, a1m1.Spec.MembershipKind, event.AccessListMemberUpdate.MemberMetadata.MembershipKind.String())
	})

	want.Spec.Joined = oldJoined
	want.Spec.Reason = oldReason
	got, err = c.svc.GetAccessListMember(c.userCtx, accesslistv1.GetAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m1.GetName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))

	// Owner can't add themselves as a member
	_, err = c.svc.UpsertAccessListMember(c.ownerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(newAccessListMember(t, a1.GetName(), ownerUser, accesslist.MembershipKindUser, c.clock))}.Build())
	require.ErrorIs(t, err, trace.AccessDenied("Adding yourself to an Access List is not allowed"))
	expectEvent(t, events.AccessListMemberCreateFailureCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.False(t, event.Success)
	})

	// User with KindUser access can add themselves as a member.
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(newAccessListMember(t, a1.GetName(), testUser, accesslist.MembershipKindUser, c.clock))}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), accesslist.MembershipKindUser)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a1.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, accesslist.MembershipKindUser, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
	})

	// Nested lists cannot be cyclical
	// adding a3 as a member list of a4 should succeed
	_, err = c.svc.UpsertAccessListMember(c.ownerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(newAccessListMember(t, a4.GetName(), a3.GetName(), accesslist.MembershipKindList, c.clock))}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), accesslist.MembershipKindList)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a4.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, accesslist.MembershipKindList, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
	})
	// now that a3 is a member of a4, adding a4 as a member of a3 should fail
	_, err = c.svc.UpsertAccessListMember(c.ownerCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(newAccessListMember(t, a3.GetName(), a4.GetName(), accesslist.MembershipKindList, c.clock))}.Build())
	expectedErrMsg := fmt.Sprintf("Access List '%s' can't be added as a Member of '%s' because '%s' is already included as a Member or Owner in '%s'", a4.Spec.Title, a3.Spec.Title, a3.Spec.Title, a4.Spec.Title)
	require.ErrorContains(t, err, expectedErrMsg)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorIs(t, err, accesslists.ErrCyclicMembership)
	expectEvent(t, events.AccessListMemberCreateFailureCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.False(t, event.Success)
	})

	// User cannot add a nested list they are a member of to another list
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(newAccessListMember(t, a5.GetName(), testUserDenyWhere, accesslist.MembershipKindUser, c.clock))}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.True(t, event.Success)
		require.Equal(t, event.Members[0].MembershipKind.String(), accesslist.MembershipKindUser)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
		require.Equal(t, a5.GetName(), event.AccessListMemberCreate.Metadata.Id)
		require.Equal(t, accesslist.MembershipKindUser, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
	})
	_, err = c.svc.UpsertAccessListMember(c.userDenyWhereCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(newAccessListMemberWithIneligibleReason(
			t, a6.GetName(), a5.GetName(), c.clock, accesslist.MembershipKindList, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String())),
	}.Build())
	// since user is a member of a1, and lacks Update/Create RBAC, they should not be able to add a1 to a4
	require.ErrorIs(t, err, trace.AccessDenied("Adding an Access List you are a member of to another Access List is not allowed"))
	expectEvent(t, events.AccessListMemberCreateFailureCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
		require.False(t, event.Success)
	})

	// SCIM-sourced Access Lists should prohibit member addition.
	scimMember := newAccessListMember(t, scimList.GetName(), externalMember1, accesslist.MembershipKindUser, c.clock)
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(scimMember),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "SCIM-sourced Access List members modification not allowed")

	// Entra ID-sourced Access Lists should prohibit member addition.
	entraList := newAccessList(t, "entra", c.clock)
	entraList.Metadata.SetStaticLabels(map[string]string{types.OriginLabel: types.OriginEntraID})
	c.svc.accessLists.UpsertAccessList(c.userCtx, entraList)

	entraMember := newAccessListMember(t, entraList.GetName(), "entra-member", accesslist.MembershipKindUser, c.clock)
	_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(entraMember),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "Entra ID-sourced Access List members modification not allowed")
}

func TestService_UpsertStaticAccessListMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	staticAccessList := newAccessList(t, "test-acl-1", c.clock, withType(accesslist.Static))
	defaultAccessList := newAccessList(t, "test-acl-3", c.clock)
	scimAccessList := newAccessList(t, "test-acl-4", c.clock, withType(accesslist.SCIM))

	accessLists := []*accesslist.AccessList{
		staticAccessList, defaultAccessList, scimAccessList,
	}

	newMember := func(t *testing.T, accessList *accesslist.AccessList, memberName string) *accesslistv1.Member {
		t.Helper()
		m := newAccessListMember(t, accessList.GetName(), memberName, accesslist.MembershipKindUser, c.clock)
		return conv.ToMemberProto(m)
	}

	createAccessLists(t, c.userCtx, c.svc, c.emitter, nil, accessLists)

	t.Run("upserting member to non-static access_list fails", func(t *testing.T) {
		nonStaticAccessLists := []*accesslist.AccessList{
			defaultAccessList, scimAccessList,
		}
		for _, accessList := range nonStaticAccessLists {
			t.Run(accessList.GetName(), func(t *testing.T) {
				for _, m := range []string{member1, member2} {
					_, err := c.svc.UpsertStaticAccessListMember(c.userCtx, accesslistv1.UpsertStaticAccessListMemberRequest_builder{
						Member: newMember(t, accessList, m),
					}.Build())
					require.Error(t, err, "member = %q", m)
					require.True(t, isNonStaticAccessList(err), "member = %q", m)
					require.True(t, trace.IsBadParameter(err), "member = %q", m)
				}
			})
		}
	})

	t.Run("upserting member of static access_list succeeds", func(t *testing.T) {
		for _, m := range []string{member1, member2} {
			resp, err := c.svc.UpsertStaticAccessListMember(c.userCtx, accesslistv1.UpsertStaticAccessListMemberRequest_builder{
				Member: newMember(t, staticAccessList, m),
			}.Build())
			require.NoError(t, err, "member = %q", m)

			resp.GetMember().GetHeader().GetMetadata().SetLabels(map[string]string{"updated": "label"})
			_, err = c.svc.UpsertStaticAccessListMember(c.userCtx, accesslistv1.UpsertStaticAccessListMemberRequest_builder{
				Member: resp.GetMember(),
			}.Build())
			require.NoError(t, err, "member = %q", m)
		}
	})
}

func TestService_UpsertAccessListMemberMaxDepth(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Nested lists cannot be > `MaxAllowedDepth` levels deep
	rootList := newAccessList(t, "root", c.clock)
	_, err := c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(rootList)}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
		require.Equal(t, rootList.GetName(), event.AccessListCreate.Metadata.Id)
	})

	for i := 0; i <= accesslist.MaxAllowedDepth; i++ {
		var parentList *accesslist.AccessList
		if i == 0 {
			parentList = rootList
		} else {
			if list, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{Name: fmt.Sprintf("nested-%d", i-1)}.Build()); err == nil {
				parentList, err = conv.FromProto(list)
				require.NoError(t, err)
			} else {
				require.Fail(t, "failed to get parent list")
			}
		}

		nestedList := newAccessList(t, fmt.Sprintf("nested-%d", i), c.clock)
		_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(nestedList)}.Build())
		require.NoError(t, err)
		expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {
			require.True(t, event.Success)
		})
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
			require.Equal(t, nestedList.GetName(), event.AccessListCreate.Metadata.Id)
		})

		_, err = c.svc.UpsertAccessListMember(c.userCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: conv.ToMemberProto(newAccessListMember(t, parentList.GetName(), nestedList.GetName(), accesslist.MembershipKindList, c.clock)),
		}.Build())

		if i == accesslist.MaxAllowedDepth {
			expectedErrMsg := fmt.Sprintf("Access List '%s' can't be added as a Member of '%s' because it would exceed the maximum nesting depth of %d", nestedList.Spec.Title, parentList.Spec.Title, accesslist.MaxAllowedDepth)
			require.ErrorContains(t, err, expectedErrMsg)
			require.True(t, trace.IsBadParameter(err))
			require.ErrorIs(t, err, accesslists.ErrMaxNestedMembershipDepth)
			expectEvent(t, events.AccessListMemberCreateFailureCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
				require.False(t, event.Success)
			})
		} else {
			require.NoError(t, err)
			expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {
				require.True(t, event.Success)
				require.Equal(t, event.Members[0].MembershipKind.String(), accesslist.MembershipKindList)
			})
			expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
				require.Equal(t, parentList.GetName(), event.AccessListMemberCreate.Metadata.Id)
				require.Equal(t, accesslist.MembershipKindList, event.AccessListMemberCreate.MemberMetadata.MembershipKind.String())
			})
		}
	}
}

func TestService_DeleteAccessListMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	scimList := newAccessList(t, "scim", c.clock, withType(accesslist.SCIM))

	// a1 will have a label attached.
	a1.SetStaticLabels(map[string]string{
		"denied": "true",
	})

	// a2 will have a label attached.
	a2.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	scimMember := newAccessListMember(t, scimList.GetName(), externalMember1, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
		[]*accesslist.AccessList{a1, a2, scimList}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// userWhere can't delete from a1
	_, err := c.svc.DeleteAccessListMember(c.userWhereCtx, accesslistv1.DeleteAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Admin should be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.userCtx, accesslistv1.DeleteAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m1.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})

	members := getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m2}, members, cmpOpts...))

	// userDenyWhere should not be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.userDenyWhereCtx, accesslistv1.DeleteAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m2.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// owner should be able to delete members
	_, err = c.svc.DeleteAccessListMember(c.ownerCtx, accesslistv1.DeleteAccessListMemberRequest_builder{AccessList: a1.GetName(), MemberName: a1m2.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a1.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})
	members = getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, members)

	// userWhere can delete from a2
	_, err = c.svc.DeleteAccessListMember(c.userWhereCtx, accesslistv1.DeleteAccessListMemberRequest_builder{AccessList: a2.GetName(), MemberName: a2m1.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteSuccessCode, c.emitter, func(event *apievents.AccessListMemberDelete) {
		require.True(t, event.Success)
	})
	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
		require.Equal(t, a2.GetName(), event.AccessListMemberDelete.Metadata.Id)
	})

	// Add a member directly in the backend to bypass service validation
	// This seeds the list with a member so we can test deletion enforcement on a SCIM list.
	_, err = c.testEnv.accessLists.UpsertAccessListMember(c.userCtx, scimMember)
	require.NoError(t, err)

	// SCIM-sourced Access Lists should prohibit member removal.
	_, err = c.svc.DeleteAccessListMember(c.userCtx, accesslistv1.DeleteAccessListMemberRequest_builder{
		AccessList: scimList.GetName(),
		MemberName: scimMember.GetName(),
	}.Build())

	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "SCIM-sourced Access List members modification not allowed")

	// Entra ID-sourced Access Lists should prohibit member removal.
	entraList := newAccessList(t, "entra-delete", c.clock)
	entraList.Metadata.SetStaticLabels(map[string]string{types.OriginLabel: types.OriginEntraID})
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{entraList}, nil)

	entraMember := newAccessListMember(t, entraList.GetName(), "entra-member", accesslist.MembershipKindUser, c.clock)
	_, err = c.testEnv.accessLists.UpsertAccessListMember(c.userCtx, entraMember)
	require.NoError(t, err)

	_, err = c.svc.DeleteAccessListMember(c.userCtx, accesslistv1.DeleteAccessListMemberRequest_builder{
		AccessList: entraList.GetName(),
		MemberName: entraMember.GetName(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "Entra ID-sourced Access List members modification not allowed")
}

func TestService_DeleteStaticAccessListMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	staticAccessList := newAccessList(t, "test-acl-1", c.clock, withType(accesslist.Static))
	defaultAccessList := newAccessList(t, "test-acl-3", c.clock)
	scimAccessList := newAccessList(t, "test-acl-4", c.clock, withType(accesslist.SCIM))

	accessLists := []*accesslist.AccessList{
		staticAccessList, defaultAccessList, scimAccessList,
	}

	createAccessLists(t, c.userCtx, c.svc, c.emitter, nil, accessLists)

	members := []*accesslist.AccessListMember{}

	for _, accessList := range accessLists {
		members = append(members, newAccessListMember(t, accessList.GetName(), member1, accesslist.MembershipKindUser, c.clock))
		members = append(members, newAccessListMember(t, accessList.GetName(), member2, accesslist.MembershipKindUser, c.clock))
	}

	for _, member := range members {
		// Add members directly in the backend to bypass service validation
		_, err := c.testEnv.accessLists.UpsertAccessListMember(c.userCtx, member)
		require.NoError(t, err)
	}

	t.Run("deleting member of non-static access_list fails", func(t *testing.T) {
		nonStaticAccessLists := []*accesslist.AccessList{
			defaultAccessList, scimAccessList,
		}
		for _, accessList := range nonStaticAccessLists {
			t.Run(accessList.GetName(), func(t *testing.T) {
				for _, m := range []string{member1, member2} {
					_, err := c.svc.DeleteStaticAccessListMember(c.userCtx, accesslistv1.DeleteStaticAccessListMemberRequest_builder{
						AccessList: accessList.GetMetadata().Name,
						MemberName: m,
					}.Build())
					require.Error(t, err, "member = %q", m)
					require.True(t, isNonStaticAccessList(err), "member = %q", m)
					require.True(t, trace.IsBadParameter(err), "member = %q", m)
				}
			})
		}
	})

	t.Run("deleting member of static access_list succeeds", func(t *testing.T) {
		for _, m := range []string{member1, member2} {
			_, err := c.svc.DeleteStaticAccessListMember(c.userCtx, accesslistv1.DeleteStaticAccessListMemberRequest_builder{
				AccessList: staticAccessList.GetMetadata().Name,
				MemberName: m,
			}.Build())
			require.NoError(t, err)
		}
	})
}

func TestService_DeleteAllAccessListMembersForAccessList(t *testing.T) {
	t.Parallel()
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

	nestedList := newAccessList(t, "nested", c.clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a2m2 := newAccessListMember(t, a2.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a3mNested := newAccessListMember(t, a3.GetName(), nestedList.GetName(), accesslist.MembershipKindList, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
		[]*accesslist.AccessList{a1, a2, a3, nestedList}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2, a3mNested})

	// userWhere can't delete from a1
	_, err := c.svc.DeleteAllAccessListMembersForAccessList(c.userWhereCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{AccessList: a1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Admin should be able to delete members
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{AccessList: a1.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members := getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, members)

	// userDenyWhere can't delete from a1
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userDenyWhereCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{AccessList: a1.GetName()}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// owner should be able to delete members
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.ownerCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{AccessList: a2.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})

	members = getAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 1)
	require.Empty(t, members)

	// UserWhere should be able to delete members from a3
	_, err = c.svc.DeleteAllAccessListMembersForAccessList(c.userWhereCtx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{AccessList: a3.GetName()}.Build())
	require.NoError(t, err)
	expectEvent(t, events.AccessListMemberDeleteAllForAccessListSuccessCode, c.emitter, func(event *apievents.AccessListMemberDeleteAllForAccessList) {
		require.True(t, event.Success)
	})
}

func TestService_UpsertAccessListWithMembers_IneligibleStatus(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Create a list.
	testlist := newAccessList(t, "testlist", c.clock)

	// Replace owners with a ineligible new owner.
	testlist.SetOwners([]accesslist.Owner{{Name: testUser}})

	// Replace members with a ineligible new member.
	member := newAccessListMember(t, testlist.GetName(), ownerUser, accesslist.MembershipKindUser, c.clock)

	resp, err := c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(testlist),
		Members: conv.ToMembersProto([]*accesslist.AccessListMember{
			member,
		}),
	}.Build())
	require.NoError(t, err)

	require.Len(t, resp.GetAccessList().GetSpec().GetOwners(), 1)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS, resp.GetAccessList().GetSpec().GetOwners()[0].GetIneligibleStatus())

	require.Len(t, resp.GetMembers(), 1)
	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS, resp.GetMembers()[0].GetSpec().GetIneligibleStatus())
}

func TestPickAccessListForWrite(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name              string
		mutate            func(originalAccessList, requestedAccessList *accesslist.AccessList)
		originalNil       bool
		wantModified      bool
		wantOriginalWrite bool
	}{
		{
			name:         "original is nil - keep requested and ACL has changed",
			originalNil:  true,
			wantModified: true,
		},
		{
			name: "ephemeral only differences keep requested ACL",
			mutate: func(originalAccessList, requestedAccessList *accesslist.AccessList) {
				requestedAccessList.SetRevision("new-revision")
				requestedAccessList.Status.OwnerOf = []string{"owner-of"}
				requestedAccessList.Status.ScopedOwnerOf = []string{"/scope/owned"}
				requestedAccessList.Spec.Owners[0].IneligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED.String()
			},
			wantModified: false,
		},
		{
			name: "canonical only differences use stored ACL",
			mutate: func(originalAccessList, requestedAccessList *accesslist.AccessList) {
				requestedAccessList.Spec.Owners = []accesslist.Owner{
					originalAccessList.Spec.Owners[1],
					originalAccessList.Spec.Owners[1],
					originalAccessList.Spec.Owners[0],
					originalAccessList.Spec.Owners[2],
					originalAccessList.Spec.Owners[3],
				}
				requestedAccessList.Spec.Grants.Roles = []string{"grole2", "grole1", "grole1"}
				requestedAccessList.Spec.MembershipRequires.Roles = []string{"mrole2", "mrole1", "mrole1"}
				requestedAccessList.Spec.OwnershipRequires.Roles = []string{"orole2", "orole1", "orole1"}
			},
			wantModified:      false,
			wantOriginalWrite: true,
		},
		{
			name: "semantic differences keep requested ACL and mark modified",
			mutate: func(originalAccessList, requestedAccessList *accesslist.AccessList) {
				requestedAccessList.Spec.Title = "updated title"
			},
			wantModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalAccessList := newAccessList(t, "test", clock)
			requestedAccessList := originalAccessList.Clone()
			if tt.originalNil {
				originalAccessList = nil
				requestedAccessList = newAccessList(t, "test", clock)
			}
			if tt.mutate != nil {
				tt.mutate(originalAccessList, requestedAccessList)
			}

			writeAccessList, modified := pickAccessListForWrite(originalAccessList, requestedAccessList)

			require.Equal(t, tt.wantModified, modified)
			if tt.wantOriginalWrite {
				require.Same(t, originalAccessList, writeAccessList)
			} else {
				require.Same(t, requestedAccessList, writeAccessList)
			}
		})
	}
}

func TestService_UpsertAccessListWithMembers(t *testing.T) {
	t.Parallel()
	// disable reconciler to avoid extra update events
	c := initSvc(t, withDisabledReconcilers())

	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	aOkta := newAccessList(t, "okta", c.clock)

	aOkta.SetOrigin(types.OriginOkta)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member3, accesslist.MembershipKindUser, c.clock)
	a2m2 := newAccessListMemberWithIneligibleReason(t, a2.GetName(), "user4", c.clock, accesslist.MembershipKindUser, accesslistv1.IneligibleStatus_name[int32(accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST)])

	oktaIdentity := authtest.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1})

	upsertAccessListWithMembers := func(t *testing.T, ctx context.Context, accessList *accesslist.AccessList,
		members []*accesslist.AccessListMember, wantErrFn require.ErrorAssertionFunc,
	) {
		oldAccessListResp, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{
			Name: accessList.GetName(),
		}.Build())
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
		if !accesslist.EqualAccessLists(oldAccessList, accessList, accesslist.WithIgnoreEphemeralFields()) {
			checkAccessListModificationEvent = true
		}

		accessListCreated := false
		if oldAccessList == nil {
			accessListCreated = true
			membersCreated = len(members)
		} else {
			oldMembers, err := c.svc.getAccessListMemberMap(ctx, accesslists.ScopeQualifiedName(accessList))
			require.NoError(t, err)

			for _, member := range members {
				memberName, err := accesslists.MemberScopeQualifiedName(member)
				require.NoError(t, err)
				if _, ok := oldMembers[memberName]; ok {
					membersUpdated++
					delete(oldMembers, memberName)
				} else {
					membersCreated++
				}
			}
			membersDeleted = len(oldMembers)
		}

		resp, err := c.svc.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(accessList),
			Members:    conv.ToMembersProto(members),
		}.Build())
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

		// The upsert response must carry the caller's assignments, matching what
		// a get returns, so clients can derive their permissions from it without
		// refetching the access list.
		getResp, getErr := c.svc.GetAccessList(ctx, accesslistv1.GetAccessListRequest_builder{Name: accessList.GetName()}.Build())
		require.NoError(t, getErr)
		require.NotNil(t, resp.GetAccessList().GetStatus().GetCurrentUserAssignments())
		require.Equal(t, getResp.GetStatus().GetCurrentUserAssignments(), resp.GetAccessList().GetStatus().GetCurrentUserAssignments())

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
			for range membersCreated {
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
			for range membersUpdated {
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
			for range membersDeleted {
				expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberDelete) {
					require.Equal(t, accessList.GetName(), event.AccessListMemberDelete.Metadata.Id)
				})
			}
		}
	}

	// Sanity check
	membersA1 := getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 2)
	require.Len(t, membersA1, 2)

	membersA2 := getAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
	require.Len(t, membersA2, 1)

	t.Run("non-RBAC owner can modify members when roles are in different order", func(t *testing.T) {
		// access list with roles in sorted order
		alWithSortedRoles := newAccessListWithPartialSpec(t, "sorted-roles", c.clock.Now().Add(time.Hour*24*365), accesslist.Spec{
			Owners: []accesslist.Owner{
				{Name: ownerUser, Description: "owner1", MembershipKind: accesslist.MembershipKindUser},
				{Name: ownerUser2, Description: "owner2", MembershipKind: accesslist.MembershipKindUser},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grant-a", "grant-b", "grant-c"},
			},
			OwnerGrants: accesslist.Grants{
				Roles: []string{"grant-a", "grant-b", "grant-c"},
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"alpha", "beta", "gamma"},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
			},
		})
		createResp, err := c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(alWithSortedRoles),
			Members:    nil,
		}.Build())
		require.NoError(t, err)

		expectEvent(t, events.AccessListCreateSuccessCode, c.emitter, func(event *apievents.AccessListCreate) {})
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {})

		alWithReorderedRoles, err := conv.FromProto(createResp.GetAccessList())
		require.NoError(t, err)

		// reorder and add dups to owners to make sure it gets canonicalised before the equality check.
		alWithReorderedRoles.Spec.Owners = []accesslist.Owner{
			alWithSortedRoles.Spec.Owners[1],
			alWithSortedRoles.Spec.Owners[1],
			alWithSortedRoles.Spec.Owners[0],
		}
		// reorder roles to simulate web ui behavior
		alWithReorderedRoles.Spec.Grants.Roles = []string{"grant-c", "grant-a", "grant-b"}
		alWithReorderedRoles.Spec.OwnerGrants.Roles = []string{"grant-c", "grant-a", "grant-b"}
		alWithReorderedRoles.Spec.MembershipRequires.Roles = []string{"gamma", "alpha", "beta"}
		alWithReorderedRoles.Spec.OwnershipRequires.Roles = []string{"orole2", "orole1"}

		ownerAuthCtx, err := c.svc.authorizer.AuthorizeScoped(c.ownerCtx)
		require.NoError(t, err)

		// make sure the owner does not have RBAC for this test to check the
		// non-RBAC owner can still add members to the ACL.
		err = c.svc.hasAccessListRBAC(
			c.ownerCtx,
			ownerAuthCtx,
			alWithReorderedRoles,
			scopedaccess.Update,
		)
		require.True(t, trace.IsAccessDenied(err), "owner does not need RBAC to be able to add members to ACL")

		// owner should be able to add members even though role order differs
		newMember := newAccessListMember(t, alWithSortedRoles.GetName(), member1, accesslist.MembershipKindUser, c.clock)
		upsertResp, err := c.svc.UpsertAccessListWithMembers(c.ownerCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(alWithReorderedRoles),
			Members:    conv.ToMembersProto([]*accesslist.AccessListMember{newMember}),
		}.Build())
		require.NoError(t, err, "owner should be able to add members when roles are in different order")
		require.NotNil(t, upsertResp)

		// non-RBAC owners are not authorized to change the ACL (other than
		// add/remove members), so check that the ACL remains unchanged.

		// ensure the unordered owners were not persisted.
		require.Equal(t, createResp.GetAccessList().GetSpec().GetOwners(), upsertResp.GetAccessList().GetSpec().GetOwners())

		// ensure the unordered roles were not persisted
		require.Equal(t, createResp.GetAccessList().GetSpec().GetMembershipRequires(), upsertResp.GetAccessList().GetSpec().GetMembershipRequires(), "membershipRequires should not be updated")
		require.Equal(t, createResp.GetAccessList().GetSpec().GetGrants(), upsertResp.GetAccessList().GetSpec().GetGrants(), "memberGrants should not be updated")
		require.Equal(t, createResp.GetAccessList().GetSpec().GetOwnershipRequires(), upsertResp.GetAccessList().GetSpec().GetOwnershipRequires(), "ownershipRequires should not be updated")
		require.Equal(t, createResp.GetAccessList().GetSpec().GetOwnerGrants(), upsertResp.GetAccessList().GetSpec().GetOwnerGrants(), "ownerGrants should not be updated")

		expectEvent(t, events.AccessListMemberCreateSuccessCode, c.emitter, func(event *apievents.AccessListMemberCreate) {})
		expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {})

		// verify member was added
		members := getAccessListMembers(c.userCtx, t, c.svc, alWithSortedRoles.GetName(), 2)
		require.Len(t, members, 1)
		require.Equal(t, member1, members[0].GetName())
	})

	t.Run("create a new access list with members", func(t *testing.T) {
		a4 := newAccessList(t, "4", c.clock)
		upsertAccessListWithMembers(t, c.userCtx, a4, []*accesslist.AccessListMember{
			newAccessListMember(t, a4.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a4.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		membersA4 := getAccessListMembers(c.userCtx, t, c.svc, a4.GetName(), 3)
		require.Len(t, membersA4, 2)
	})

	t.Run("remove one member", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.userCtx, a1, []*accesslist.AccessListMember{
			a1m1,
		}, require.NoError)

		// One member should have been deleted
		membersA1 = getAccessListMembers(c.userCtx, t, c.svc, a1.GetName(), 2)
		require.Len(t, membersA1, 1)
	})

	t.Run("add one member", func(t *testing.T) {
		// Add one member to a2
		upsertAccessListWithMembers(t, c.userCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
		}, require.NoError)

		// One member should have been added
		membersA2 = getAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Len(t, membersA2, 2)
	})

	t.Run("remove all members", func(t *testing.T) {
		// If not members are provided all members should be deleted
		upsertAccessListWithMembers(t, c.userCtx, a2, nil, require.NoError)

		// All members should have been deleted
		membersA2 = getAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Empty(t, membersA2)
	})

	t.Run("owner can't modify owners", func(t *testing.T) {
		// Make a copy of a3.
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Owners = append(a3.Spec.Owners, accesslist.Owner{
			Name: "dummy",
		})
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify grant roles", func(t *testing.T) {
		// Make a copy of a3.
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Grants.Roles = append(a3.Spec.Grants.Roles, "dummy")
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify grant traits", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Grants.Traits["dummy"] = []string{"value1", "value2"}
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify membership requires", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"some-new-role"},
		}
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't modify audit", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		a3.Spec.Audit.NextAuditDate = a3.Spec.Audit.NextAuditDate.Add(24 * time.Hour * 365)
		upsertAccessListWithMembers(t, c.ownerCtx, a3, nil, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("userDenyWhere can't modify members", func(t *testing.T) {
		_, err := c.svc.UpsertAccessListWithMembers(c.userWhereCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(a3),
			Members: []*accesslistv1.Member{
				conv.ToMemberProto(newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)),
				conv.ToMemberProto(newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)),
			},
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("owner can modify members", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.ownerCtx, a3, []*accesslist.AccessListMember{
			newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		// One member should have been deleted
		membersA3 := getAccessListMembers(c.userCtx, t, c.svc, a3.GetName(), 3)
		require.Len(t, membersA3, 2)
	})

	t.Run("member can't modify members", func(t *testing.T) {
		a3, err := conv.FromProto(conv.ToProto(a3))
		require.NoError(t, err)
		upsertAccessListWithMembers(t, memberCtx, a3, []*accesslist.AccessListMember{
			newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, func(t require.TestingT, err error, i ...any) {
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("owner can't add itself as a member", func(t *testing.T) {
		ownerMember := newAccessListMember(t, a2.GetName(), ownerUser, accesslist.MembershipKindUser, c.clock)

		// Owner tries to add itself as a member.
		upsertAccessListWithMembers(t, c.ownerCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
		}, func(t require.TestingT, err error, i ...any) {
			require.ErrorIs(t, err, trace.AccessDenied("Adding yourself to an Access List is not allowed"))
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
			newAccessListMember(t, a2.GetName(), "new-user1", accesslist.MembershipKindUser, c.clock),
		}, require.NoError)
		ownerMember.Spec.Joined = oldJoin

		resp, err := c.svc.GetAccessListMember(c.userCtx, accesslistv1.GetAccessListMemberRequest_builder{
			AccessList: ownerMember.Spec.AccessList,
			MemberName: ownerMember.Metadata.Name,
		}.Build())
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
			newAccessListMember(t, a2.GetName(), "new-user1", accesslist.MembershipKindUser, c.clock),
		}, func(t require.TestingT, err error, i ...any) {
			require.ErrorIs(t, err, trace.AccessDenied("Adding yourself to an Access List is not allowed"))
		})

		// Test user attempts to add their own user, which is okay because they have KindUser RBAC access.
		upsertAccessListWithMembers(t, c.userCtx, a2, []*accesslist.AccessListMember{
			a2m1,
			a2m2,
			ownerMember,
			newAccessListMember(t, a2.GetName(), "new-user1", accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a2.GetName(), testUser, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		membersA2 = getAccessListMembers(c.userCtx, t, c.svc, a2.GetName(), 2)
		require.Len(t, membersA2, 5)
	})

	t.Run("user where can't create access list without appropriate where clause being satisfied", func(t *testing.T) {
		a5 := conv.ToProto(newAccessList(t, "6", c.clock))

		_, err := c.svc.UpsertAccessListWithMembers(c.userWhereCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: a5,
		}.Build())
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
			newAccessListMember(t, a5.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a5.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		membersA5 := getAccessListMembers(c.userCtx, t, c.svc, a5.GetName(), 3)
		require.Len(t, membersA5, 2)
	})

	t.Run("userDenyWhere can't create or update an access list with where clause satisfied", func(t *testing.T) {
		a6 := newAccessList(t, "6", c.clock)

		// a6 will have a label attached.
		a6.SetStaticLabels(map[string]string{
			"denied": "true",
		})

		_, err := c.svc.UpsertAccessListWithMembers(c.userDenyWhereCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(a6),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))

		expectEvent(t, events.AccessListCreateFailureCode, c.emitter, func(event *apievents.AccessListCreate) {
			require.False(t, event.Success)
		})

		upsertAccessListWithMembers(t, c.userCtx, a6, []*accesslist.AccessListMember{
			newAccessListMember(t, a6.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, a6.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		a6.Spec.Description = "some new description"
		_, err = c.svc.UpsertAccessListWithMembers(c.userDenyWhereCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
			AccessList: conv.ToProto(a6),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))

		expectEvent(t, events.AccessListUpdateFailureCode, c.emitter, func(event *apievents.AccessListUpdate) {
			require.False(t, event.Success)
		})
	})

	t.Run("okta user can create Okta sourced access lists", func(t *testing.T) {
		upsertAccessListWithMembers(t, oktaUserCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)
	})

	t.Run("owner can modify members for Okta sourced access lists", func(t *testing.T) {
		upsertAccessListWithMembers(t, c.ownerCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, aOkta.GetName(), member3, accesslist.MembershipKindUser, c.clock),
		}, require.NoError)

		// One member should have been added
		membersA3 := getAccessListMembers(c.userCtx, t, c.svc, aOkta.GetName(), 3)
		require.Len(t, membersA3, 3)
	})

	t.Run("user can modify membership requirements for Okta sourced access lists.", func(t *testing.T) {
		aOkta.Spec.MembershipRequires.Roles = append(aOkta.Spec.MembershipRequires.Roles, "some-new-role")
		upsertAccessListWithMembers(t, c.userCtx, aOkta, []*accesslist.AccessListMember{
			newAccessListMember(t, aOkta.GetName(), member1, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, aOkta.GetName(), member2, accesslist.MembershipKindUser, c.clock),
			newAccessListMember(t, aOkta.GetName(), member3, accesslist.MembershipKindUser, c.clock),
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

// TestService_UpsertAccessListWithMembers_DuplicateMember verifies that
// duplicate members in the request don't produce spurious create audit events
func TestService_UpsertAccessListWithMembers_DuplicateMember(t *testing.T) {
	t.Parallel()
	c := initSvc(t, withDisabledReconcilers())

	al := newAccessList(t, "dup-test", c.clock)
	m := newAccessListMember(t, al.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
		[]*accesslist.AccessList{al}, []*accesslist.AccessListMember{m})

	// Send the same member twice
	memberProto := conv.ToMemberProto(m)
	_, err := c.svc.UpsertAccessListWithMembers(c.userCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(al),
		Members:    []*accesslistv1.Member{memberProto, memberProto},
	}.Build())
	require.NoError(t, err)

	// The member already existed, so we expect a single update event, no creates
	expectEvent(t, events.AccessListMemberUpdateSuccessCode, c.emitter, func(event *apievents.AccessListMemberUpdate) {
		require.True(t, event.Success)
	})

	// No further audit events should be pending (specifically, no spurious create)
	select {
	case evt := <-c.emitter.C():
		t.Fatalf("unexpected extra event: %T code=%s", evt, evt.GetCode())
	default:
	}
}

func TestService_AuthOrIsOwner(t *testing.T) {
	t.Parallel()
	c := initSvc(t)
	memberCtx := genUserContext(t.Context(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})
	nonExistentUser := genUserContext(t.Context(), "doesnt-exist", []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil, []*accesslist.AccessList{a1, a2}, nil)

	tests := []struct {
		name           string
		ctx            context.Context
		accessListName accesslists.NormalizedSQN
		wantErr        require.ErrorAssertionFunc
	}{
		{
			name:           "admin context",
			ctx:            c.userCtx,
			accessListName: accesslists.ScopeQualifiedName(a1),
			wantErr:        require.NoError,
		},
		{
			name:           "owner context",
			ctx:            c.ownerCtx,
			accessListName: accesslists.ScopeQualifiedName(a1),
			wantErr:        require.NoError,
		},
		{
			name:           "member context",
			ctx:            memberCtx,
			accessListName: accesslists.ScopeQualifiedName(a1),
			wantErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:           "non-existent user context",
			ctx:            nonExistentUser,
			accessListName: accesslists.ScopeQualifiedName(a1),
			wantErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// This test must not be parallel to avoid testing issues with lock interaction and
			// the fake clock being used for the underlying tests.
			_, err := c.svc.authOrIsOwner(test.ctx, test.accessListName, scopedaccess.Read)
			test.wantErr(t, err)
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
		t.Run(test.name, func(t *testing.T) {
			var members []*apievents.AccessListMember
			if test.numberOfEvents > 0 {
				members = make([]*apievents.AccessListMember, test.numberOfEvents)
				for i := range test.numberOfEvents {
					members[i] = &apievents.AccessListMember{
						MemberName: fmt.Sprintf("%d", i),
					}
				}
			}

			batches := batchAccessListMemberMetadata(accesslists.NormalizedSQN{Name: "test-access-list"}, "test-access-list", members)
			require.Len(t, batches, test.expectedBatches)

			for i := range test.expectedBatches {
				startIndex := i * eventMemberBatches
				endIndex := min(startIndex+eventMemberBatches, test.numberOfEvents)

				batchIndex := 0
				for j := startIndex; j < endIndex; j++ {
					require.Empty(t, cmp.Diff(members[j], batches[i].Members[batchIndex]))
					batchIndex++
				}
			}
		})
	}
}

func TestService_CreateAccessListWithPreset_Permissions(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Create a role with full permissions for preset operations
	roleWithAllPerms, err := types.NewRole("preset-full-perms", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessList, types.KindUser, types.KindRole},
					Verbs:     services.RW(),
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = c.testEnv.access.UpsertRole(context.Background(), roleWithAllPerms)
	require.NoError(t, err)

	// Create a user with full permissions
	userWithAllPerms, err := types.NewUser("preset-full-perms-user")
	require.NoError(t, err)
	userWithAllPerms.SetRoles([]string{"preset-full-perms"})
	_, err = c.testEnv.identity.CreateUser(context.Background(), userWithAllPerms)
	require.NoError(t, err)

	// Create a context for user with full permissions
	ctxWithAllPerms := genUserContext(context.Background(), "preset-full-perms-user", []string{"preset-full-perms"}, nil)

	devRole, err := types.NewRole("dev-app", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"env": []string{"dev"}},
		},
	})
	require.NoError(t, err)

	req := accesslistv1.CreateAccessListWithPresetRequest_builder{
		AccessList: conv.ToProto(&accesslist.AccessList{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: "test-permissions",
				},
			},
			Spec: accesslist.Spec{
				Title:       "Test Permissions",
				Description: "Test access list for permissions",
				Owners: []accesslist.Owner{
					{Name: ownerUser},
				},
			},
		}),
		PresetType: "short-term",
		Roles:      []*types.RoleV6{devRole.(*types.RoleV6)},
	}.Build()

	tests := []struct {
		name    string
		ctx     context.Context
		require require.ErrorAssertionFunc
	}{
		{
			name:    "success - user with proper permissions",
			ctx:     ctxWithAllPerms,
			require: require.NoError,
		},
		{
			name: "error - user without role permissions",
			ctx:  c.userCtx, // has access list permissions but not role permissions
			require: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unique access list name for each test by creating a new request
			acl, err := conv.FromProto(req.GetAccessList())
			require.NoError(t, err)

			uniqueReq := accesslistv1.CreateAccessListWithPresetRequest_builder{
				AccessList: conv.ToProto(acl),
				PresetType: req.GetPresetType(),
				Roles:      req.GetRoles(),
			}.Build()

			_, err = c.svc.CreateAccessListWithPreset(tt.ctx, uniqueReq)
			tt.require(t, err)
		})
	}
}

func TestService_CreateAccessListReview(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	c := initSvc(t, withClock(clock))

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

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2})

	require.Empty(t, listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1))

	review1ForA1 := newAccessListReview(t, a1.GetName())

	// RBAC user with where clause can't create a review.
	_, err := c.svc.CreateAccessListReview(c.userWhereCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA1),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// RBAC user can create a review.
	_, err = c.svc.CreateAccessListReview(c.userCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA1),
	}.Build())
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

	_, err = c.svc.CreateAccessListReview(c.userDenyWhereCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Owner can only create reviews with member changes.
	review = newAccessListReview(t, a1.GetName())
	review.Spec.Changes.RemovedMembers = []string{a1m1.GetName()}

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review),
	}.Build())
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

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review),
	}.Build())
	require.ErrorIs(t, err, trace.AccessDenied("user cannot modify the access list as part of the review"))

	expectEvent(t, events.AccessListReviewFailureCode, c.emitter, func(event *apievents.AccessListReview) {
		require.False(t, event.Success)
	})

	review = newAccessListReview(t, a1.GetName())
	review.Spec.Changes.MembershipRequirementsChanged = &accesslist.Requires{
		Roles: []string{"some-new-role"},
	}

	_, err = c.svc.CreateAccessListReview(c.ownerCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review),
	}.Build())
	require.ErrorIs(t, err, trace.AccessDenied("user cannot modify the access list as part of the review"))

	expectEvent(t, events.AccessListReviewFailureCode, c.emitter, func(event *apievents.AccessListReview) {
		require.False(t, event.Success)
	})

	review1ForA2 := newAccessListReview(t, a2.GetName())

	// RBAC user with where clause can create a review if access list meets where clause.
	_, err = c.svc.CreateAccessListReview(c.userWhereCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA2),
	}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a2.GetName())
	})

	// Create the Okta sourced access list.
	oktaIdentity := authtest.TestBuiltin(types.RoleOkta)
	oktaUserCtx := authz.ContextWithUser(context.Background(), oktaIdentity.I)
	createAccessListsAndMembers(t, oktaUserCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a3}, []*accesslist.AccessListMember{a3m1, a3m2})

	review1ForA3 := newAccessListReview(t, a3.GetName())

	// User is allowed to change the various fields that can be changed in an Okta sourced access list.
	review1ForA3.Spec.Changes.RemovedMembers = []string{a3m1.GetName()}
	_, err = c.svc.CreateAccessListReview(c.userCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA3),
	}.Build())
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

	_, err = c.svc.CreateAccessListReview(c.userCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA3),
	}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListReviewSuccessCode, c.emitter, func(event *apievents.AccessListReview) {
		require.True(t, event.Success)
	})

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
		require.Equal(t, event.AccessListReviewCreate.Metadata.Id, a3.GetName())
	})

	// User is NOT allowed to remove members during a review of an Entra ID sourced access list.
	a4 := newAccessList(t, "4", c.clock)
	a4m1 := newAccessListMember(t, a4.GetName(), member1, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a4}, []*accesslist.AccessListMember{a4m1})

	// Change it to an Entra ID sourced access list.
	a4.SetOrigin(types.OriginEntraID)
	_, err = c.svc.UpsertAccessList(c.userCtx, accesslistv1.UpsertAccessListRequest_builder{
		AccessList: conv.ToProto(a4),
	}.Build())
	require.NoError(t, err)

	expectEvent(t, events.AccessListUpdateSuccessCode, c.emitter, func(*apievents.AccessListUpdate) {})

	review1ForA4 := newAccessListReview(t, a4.GetName())
	review1ForA4.Spec.Changes.RemovedMembers = []string{a4m1.GetName()}

	_, err = c.svc.CreateAccessListReview(c.userCtx, accesslistv1.CreateAccessListReviewRequest_builder{
		Review: conv.ToReviewProto(review1ForA4),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "membership changes are not allowed for access lists created via Entra ID integration")

	expectEvent(t, events.AccessListReviewFailureCode, c.emitter, func(event *apievents.AccessListReview) {
		require.False(t, event.Success)
	})
}

func TestService_ListAccessListReviews(t *testing.T) {
	t.Parallel()
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

	_, err := c.svc.ListAccessListReviews(c.userWhereCtx, accesslistv1.ListAccessListReviewsRequest_builder{
		AccessList: a1.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))
	reviews = listAllAccessListReviews(c.userWhereCtx, t, c.svc, a3.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review1ForA3, review2ForA3}, reviews, cmpOpts...))
}

func TestService_ListAccessListReviewsReviewerDisplays(t *testing.T) {
	t.Parallel()
	c := initSvc(t)
	auth := &authWithIdentity{
		fakeAuth: &fakeAuth{},
		identity: c.testEnv.identity,
	}
	c.svc.authServer = auth

	reviewer, err := c.testEnv.identity.GetUser(t.Context(), testUser, false)
	require.NoError(t, err)
	reviewer.SetTraits(map[string][]string{
		"displayName": {"Test Reviewer"},
		"email":       {"reviewer@example.com"},
	})
	_, err = c.testEnv.identity.UpdateUser(t.Context(), reviewer)
	require.NoError(t, err)

	a1 := newAccessList(t, "review-displays", c.clock)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1}, nil)

	reviewWithReviewers := newAccessListReview(t, a1.GetName())
	reviewWithReviewers.Spec.Reviewers = []string{testUser, ownerUser, "deleted-user"}
	_, _, err = c.testEnv.accessLists.CreateAccessListReview(t.Context(), reviewWithReviewers)
	require.NoError(t, err)

	resp, err := c.svc.ListAccessListReviews(c.userCtx, accesslistv1.ListAccessListReviewsRequest_builder{
		AccessList: a1.GetName(),
		PageSize:   10,
	}.Build())
	require.NoError(t, err)

	displays := resp.GetReviews()[0].GetStatus().GetReviewerDisplays()
	require.Equal(t, accesslistv1.UserDisplay_builder{
		Primary:   "Test Reviewer",
		Secondary: "reviewer@example.com",
	}.Build(), displays[testUser])
	require.Equal(t, &accesslistv1.UserDisplay{}, displays[ownerUser])
	require.NotContains(t, displays, "deleted-user")
}

func TestService_ListAccessListReviewsReviewerDisplayError(t *testing.T) {
	t.Parallel()
	c := initSvc(t)
	c.svc.authServer = &erroringAuth{
		fakeAuth: &fakeAuth{},
		err:      errors.New("display resolver failed"),
	}

	a1 := newAccessList(t, "review-display-error", c.clock)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1}, nil)

	review := newAccessListReview(t, a1.GetName())
	review.Spec.Reviewers = []string{testUser}
	_, _, err := c.testEnv.accessLists.CreateAccessListReview(t.Context(), review)
	require.NoError(t, err)

	resp, err := c.svc.ListAccessListReviews(c.userCtx, accesslistv1.ListAccessListReviewsRequest_builder{
		AccessList: a1.GetName(),
		PageSize:   10,
	}.Build())
	require.NoError(t, err)
	require.Len(t, resp.GetReviews(), 1)
	require.Nil(t, resp.GetReviews()[0].GetStatus())
}

func TestService_DeleteAccessListReviews(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	c := initSvc(t, withClock(clock))

	// Set the clock to a fixed date. This will ensure that the expected days past calculation later on
	// will be predictable.
	clock.Advance(c.clock.Since(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)

	// a3 will have a label attached.
	a3.SetStaticLabels(map[string]string{
		"test-label": "test",
	})

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents, []*accesslist.AccessList{a1, a2, a3}, nil)

	// Advance the clock so that the new reviews are past the review date.
	clock.Advance(time.Hour * 24 * 366) // 24 hours past the access list review date.

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

	_, err := c.svc.DeleteAccessListReview(c.userWhereCtx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.DeleteAccessListReview(c.ownerCtx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	_, err = c.svc.DeleteAccessListReview(c.userCtx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListName: review1ForA1.Spec.AccessList,
		ReviewName:     review1ForA1.GetName(),
	}.Build())
	require.NoError(t, err)

	expectUsageEvent(t, c.usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewDelete) {
		require.Equal(t, a1.GetName(), event.AccessListReviewDelete.Metadata.Id)
		require.Equal(t, review1ForA1.GetName(), event.AccessListReviewDelete.AccessListReviewId)
	})

	reviews := listAllAccessListReviews(c.userCtx, t, c.svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.Review{review2ForA1, review3ForA1}, reviews, cmpOpts...))

	_, err = c.svc.DeleteAccessListReview(c.userWhereCtx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListName: review1ForA3.Spec.AccessList,
		ReviewName:     review1ForA3.GetName(),
	}.Build())
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
	t.Parallel()
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
	t.Parallel()
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
			userCtx: genUserContext(context.Background(), ownerUser, []string{"noprole"}, nil),
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
			userCtx: genUserContext(context.Background(), ownerUser, []string{"noprole"}, nil),
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
			userCtx: genUserContext(context.Background(), ownerUser, []string{"noprole"}, nil),
			newMember: member(t, header.Metadata{Name: ownerUser}, accesslist.AccessListMemberSpec{
				Name:             ownerUser,
				AccessList:       "access-list",
				Joined:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Reason:           "reason",
				AddedBy:          "owner",
				IneligibleStatus: "ineligible",
			}),
			wantErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:    "owner modifies itself",
			userCtx: genUserContext(context.Background(), ownerUser, []string{"noprole"}, nil),
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
			wantErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:    "owner doesn't modify itself",
			userCtx: genUserContext(context.Background(), ownerUser, []string{"noprole"}, nil),
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
			authCtx, err := c.svc.authorizer.AuthorizeScoped(test.userCtx)
			require.NoError(t, err)
			username, err := getUsername(authCtx)
			require.NoError(t, err)

			test.wantErr(t, c.svc.canUpdateMembership(ctx, authCtx, username, test.oldMember, test.newMember))
		})
	}
}

func TestPopulateMembersFields(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)

	a1.SetOrigin(types.OriginOkta)

	_, err := c.svc.UpsertAccessListWithMembers(c.oktaSvcCtx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(a1),
	}.Build())
	require.NoError(t, err)
	aclMember := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	_, err = c.svc.UpsertAccessListMember(c.oktaSvcCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(aclMember),
	}.Build())
	require.NoError(t, err)
	_, err = c.svc.UpsertAccessListMember(c.oktaSvcCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(aclMember),
	}.Build())
	require.NoError(t, err)

	resp, err := c.svc.GetAccessListMember(c.oktaSvcCtx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessList: a1.GetName(),
		MemberName: member1,
	}.Build())
	require.NoError(t, err)

	got := resp.GetHeader().GetMetadata().GetLabels()
	var want map[string]string
	require.Equal(t, want, got)

	t.Run("update labels", func(t *testing.T) {
		if resp.GetHeader().GetMetadata().GetLabels() == nil {
			resp.GetHeader().GetMetadata().SetLabels(map[string]string{
				types.OriginLabel: types.OriginOkta,
			})
		}
		_, err = c.svc.UpsertAccessListMember(c.oktaSvcCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: resp,
		}.Build())
		require.NoError(t, err)
		resp, err = c.svc.GetAccessListMember(c.ownerCtx, accesslistv1.GetAccessListMemberRequest_builder{
			AccessList: a1.GetName(),
			MemberName: member1,
		}.Build())
		require.NoError(t, err)
		got := resp.GetHeader().GetMetadata().GetLabels()
		want := map[string]string{
			types.OriginLabel: types.OriginOkta,
		}
		require.Equal(t, want, got)

		resp.GetHeader().GetMetadata().SetLabels(nil)
		_, err = c.svc.UpsertAccessListMember(c.oktaSvcCtx, accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: resp,
		}.Build())
		require.NoError(t, err)
		resp, err = c.svc.GetAccessListMember(c.ownerCtx, accesslistv1.GetAccessListMemberRequest_builder{
			AccessList: a1.GetName(),
			MemberName: member1,
		}.Build())
		require.NoError(t, err)
		got = resp.GetHeader().GetMetadata().GetLabels()
		want = map[string]string{
			types.OriginLabel: types.OriginOkta,
		}
		require.Equal(t, want, got)
	})
}

func Test_nonStaticAccessListError(t *testing.T) {
	t.Parallel()
	err := newNonStaticAccessListErrorFromMemberMetaReq(
		accesslistv1.GetAccessListMemberRequest_builder{
			AccessList: "vegetables",
			MemberName: "carrot",
		}.Build(),
		accesslist.Default,
	)
	msg := err.Error()
	expected := `Access list member's ("carrot") access list ("vegetables") is not static (i.e., access_list with spec.type set to "static"). Access list "vegetables" type is "" (default). Teleport IaC tools support adding members only to access lists of type "static".`
	require.Equal(t, expected, msg)
	require.True(t, isNonStaticAccessList(err))
}

func Test_userTryingToAddThemselves(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	// Test setup: creating common fixtures.
	const (
		unprivilegedUserName = "my-user"
		unprivilegedRoleName = "normal-role"
		unrelatedUserName    = "some-other-user"
		adminUserName        = "admin-user"
		adminRoleName        = "admin-role"
		rootListName         = "root-list"
		middleListName       = "middle-list"
		leafListName         = "leaf-list"
		impossibleListName   = "impossible-list"
	)
	adminRole, err := types.NewRole(adminRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = c.testEnv.access.CreateRole(t.Context(), adminRole)
	require.NoError(t, err)

	// unprivilegedRole is a role that gives nothing, because a user must have a role.
	unprivilegedRole, err := types.NewRole(unprivilegedRoleName, types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = c.testEnv.access.CreateRole(t.Context(), unprivilegedRole)
	require.NoError(t, err)

	// unprivilegedUser is a user without any rights.
	unprivilegedUser, err := types.NewUser(unprivilegedUserName)
	require.NoError(t, err)
	unprivilegedUser.SetRoles([]string{unprivilegedRole.GetName()})
	_, err = c.testEnv.identity.CreateUser(t.Context(), unprivilegedUser)
	require.NoError(t, err)

	// unrelatedUser is a user without any rights, different from unprivilegedUser.
	// This is used to test that a user can add another user to an access list.
	unrelatedUser, err := types.NewUser(unrelatedUserName)
	require.NoError(t, err)
	unrelatedUser.SetRoles([]string{unprivilegedRole.GetName()})
	_, err = c.testEnv.identity.CreateUser(t.Context(), unrelatedUser)
	require.NoError(t, err)

	// adminUser can edit other users and, they are allowed to add themselves to an access list.
	adminUser, err := types.NewUser(adminUserName)
	require.NoError(t, err)
	adminUser.SetRoles([]string{adminRole.GetName()})
	_, err = c.testEnv.identity.CreateUser(t.Context(), adminUser)
	require.NoError(t, err)

	// rootAccessList is the list we are trying to add users and other lists to in the tests.
	rootAccessList := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: rootListName}},
		Spec: accesslist.Spec{
			Title:  rootListName,
			Grants: accesslist.Grants{Roles: []string{adminRoleName}},
			Owners: []accesslist.Owner{{Name: ownerUser}},
		},
	}
	_, err = c.svc.accessLists.UpsertAccessList(t.Context(), rootAccessList)
	require.NoError(t, err)

	middleAccessList := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: middleListName}},
		Spec: accesslist.Spec{
			Title:  middleListName,
			Grants: accesslist.Grants{Roles: []string{adminRoleName}},
			Owners: []accesslist.Owner{{Name: ownerUser}},
		},
	}
	_, err = c.svc.accessLists.UpsertAccessList(t.Context(), middleAccessList)
	require.NoError(t, err)

	leafAccessList := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: leafListName}},
		Spec: accesslist.Spec{
			Title:  leafListName,
			Grants: accesslist.Grants{Roles: []string{adminRoleName}},
			Owners: []accesslist.Owner{{Name: ownerUser}},
		},
	}
	_, err = c.svc.accessLists.UpsertAccessList(t.Context(), leafAccessList)
	require.NoError(t, err)

	// impossibleAccessList is an accesslist whose requirements cannot be met.
	impossibleAccessList := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: impossibleListName}},
		Spec: accesslist.Spec{
			Title:              impossibleListName,
			Grants:             accesslist.Grants{Roles: []string{adminRoleName}},
			MembershipRequires: accesslist.Requires{Traits: map[string][]string{"impossible": {"impossible"}}},
			Owners:             []accesslist.Owner{{Name: ownerUser}},
		},
	}
	_, err = c.svc.accessLists.UpsertAccessList(t.Context(), impossibleAccessList)
	require.NoError(t, err)

	middleInRootMembership := &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: middleListName}},
		Spec: accesslist.AccessListMemberSpec{
			AccessList:     rootListName,
			Name:           middleListName,
			MembershipKind: accesslist.MembershipKindList,
		},
	}

	tests := []struct {
		name                string
		user                types.User
		existingMemberships []*accesslist.AccessListMember
		newMemberships      []*accesslist.AccessListMember
		expectErr           require.ErrorAssertionFunc
	}{
		{
			name: "user adds another user which is not themselves",
			user: unprivilegedUser,
			newMemberships: []*accesslist.AccessListMember{{
				ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unrelatedUserName}},
				Spec: accesslist.AccessListMemberSpec{
					AccessList:     rootListName,
					Name:           unrelatedUserName,
					MembershipKind: accesslist.MembershipKindUser,
				},
			}},
			expectErr: require.NoError,
		},
		{
			name: "user cannot add themselves directly",
			user: unprivilegedUser,
			newMemberships: []*accesslist.AccessListMember{{
				ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unprivilegedUserName}},
				Spec: accesslist.AccessListMemberSpec{
					AccessList:     rootListName,
					Name:           unprivilegedUserName,
					MembershipKind: accesslist.MembershipKindUser,
				},
			}},
			expectErr: require.Error,
		},
		{
			name: "user can add themselves directly if they can edit other users",
			user: adminUser,
			newMemberships: []*accesslist.AccessListMember{{
				ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: adminUserName}},
				Spec: accesslist.AccessListMemberSpec{
					AccessList:     rootListName,
					Name:           adminUserName,
					MembershipKind: accesslist.MembershipKindUser,
				},
			}},
			expectErr: require.NoError,
		},
		{
			name: "user cannot add a list it is a direct member of",
			user: unprivilegedUser,
			existingMemberships: []*accesslist.AccessListMember{
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unprivilegedUserName}},
					Spec:           accesslist.AccessListMemberSpec{AccessList: middleListName, Name: unprivilegedUserName, MembershipKind: accesslist.MembershipKindUser},
				},
			},
			newMemberships: []*accesslist.AccessListMember{middleInRootMembership},
			expectErr:      require.Error,
		},
		{
			name: "user adds a list it is a nested member of",
			user: unprivilegedUser,
			existingMemberships: []*accesslist.AccessListMember{
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: leafListName}},
					Spec:           accesslist.AccessListMemberSpec{AccessList: middleListName, Name: leafListName, MembershipKind: accesslist.MembershipKindList},
				},
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unprivilegedUserName}},
					Spec:           accesslist.AccessListMemberSpec{AccessList: leafListName, Name: unprivilegedUserName, MembershipKind: accesslist.MembershipKindUser},
				},
			},
			newMemberships: []*accesslist.AccessListMember{middleInRootMembership},
			expectErr:      require.Error,
		},
		{
			name: "user adds a list it is an expired member of",
			user: unprivilegedUser,
			existingMemberships: []*accesslist.AccessListMember{
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unprivilegedUserName}},
					Spec: accesslist.AccessListMemberSpec{
						AccessList:     middleListName,
						Name:           unprivilegedUserName,
						MembershipKind: accesslist.MembershipKindUser,
						Expires:        c.clock.Now().Add(-time.Hour),
					},
				},
			},
			newMemberships: []*accesslist.AccessListMember{middleInRootMembership},
			expectErr:      require.Error,
		},
		{
			name: "user adds a nested list it is a member of but doesn't meet requirements",
			user: unprivilegedUser,
			existingMemberships: []*accesslist.AccessListMember{
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: impossibleListName}},
					Spec:           accesslist.AccessListMemberSpec{AccessList: middleListName, Name: impossibleListName, MembershipKind: accesslist.MembershipKindList},
				},
				{
					ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: unprivilegedUserName}},
					Spec:           accesslist.AccessListMemberSpec{AccessList: impossibleListName, Name: unprivilegedUserName, MembershipKind: accesslist.MembershipKindUser},
				},
			},
			newMemberships: []*accesslist.AccessListMember{middleInRootMembership},
			expectErr:      require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: load test-specific fixtures.
			for _, m := range tt.existingMemberships {
				// Setting fields the service wants to see, those fields are not relevant for the test.
				m.Spec.Joined = c.clock.Now().Add(-2 * time.Hour)
				m.Spec.AddedBy = testUser

				_, err = c.testEnv.accessLists.UpsertAccessListMember(t.Context(), m)
				require.NoError(t, err)
			}

			// Test execution.
			userCtx := genUserContext(t.Context(), tt.user.GetName(), tt.user.GetRoles(), tt.user.GetTraits())
			authCtx, err := c.svc.authorizer.AuthorizeScoped(userCtx)
			require.NoError(t, err)
			tt.expectErr(t, c.svc.userTryingToAddThemselves(t.Context(), authCtx, tt.user.GetName(), tt.newMemberships...))

			// Test cleanup: remove test-specific fixtures.
			for _, m := range tt.existingMemberships {
				require.NoError(t, c.testEnv.accessLists.DeleteAccessListMember(t.Context(), m.Spec.AccessList, m.Spec.Name))
			}
		})
	}
}

type fakeAuthWithUsers struct {
	*fakeAuth
	svc *Service
}

func (a *fakeAuthWithUsers) UpsertRole(ctx context.Context, r types.Role) (types.Role, error) {
	return &types.RoleV6{}, nil
}

func (a *fakeAuthWithUsers) GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error) {
	return a.svc.cache.(services.Identity).GetUser(ctx, userName, withSecrets)
}

type authWithIdentity struct {
	*fakeAuth
	identity services.Identity
}

func (a *authWithIdentity) GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error) {
	return a.identity.GetUser(ctx, userName, withSecrets)
}

type erroringAuth struct {
	*fakeAuth
	err error
}

func (a *erroringAuth) GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error) {
	return nil, a.err
}

func TestService_ListUserAccessLists(t *testing.T) {
	t.Parallel()
	c := initSvc(t)

	c.svc.authServer = &fakeAuthWithUsers{
		fakeAuth: &fakeAuth{},
		svc:      c.svc,
	}

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)
	a5 := newAccessListWithPartialSpec(t, "5", c.clock.Now().Add(time.Hour*24*365), accesslist.Spec{
		Owners: []accesslist.Owner{
			{Name: ownerUser2, Description: "owner user 2", MembershipKind: accesslist.MembershipKindUser},
			{Name: a3.GetName(), Description: "acl 3", MembershipKind: accesslist.MembershipKindList},
		},
	})

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1ma4 := newAccessListMember(t, a1.GetName(), a4.GetName(), accesslist.MembershipKindList, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a4m2 := newAccessListMember(t, a4.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4, a5}, []*accesslist.AccessListMember{
			a1m1, a1ma4, a2m1, a3m2, a4m2,
		})

	// member 1
	req := accesslistv1.ListUserAccessListsRequest_builder{
		Username: member1,
	}.Build()

	resp, err := c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 2)

	a3.Status.OwnerOf = []string{"5"}
	a4.Status.MemberOf = []string{"1"}

	a1.Status.MemberCount = ptrToUint32(1)
	a1.Status.MemberListCount = ptrToUint32(1)
	a2.Status.MemberCount = ptrToUint32(1)
	a2.Status.MemberListCount = ptrToUint32(0)

	gotACLs := mustFromProtoAll(t, resp.GetAccessLists()...)
	wantACLs := []*accesslist.AccessList{a1, a2}
	require.Empty(t, cmp.Diff(wantACLs, gotACLs, cmpOpts...))

	explicitMembership := accesslistv1.UserAssignments_builder{
		MembershipType: accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT,
		OwnershipType:  accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
	}.Build()

	gotAssignments := make([]*accesslistv1.UserAssignments, len(resp.GetAccessLists()))
	for i, al := range resp.GetAccessLists() {
		gotAssignments[i] = al.GetStatus().GetUserAssignments()
	}
	wantAssignments := []*accesslistv1.UserAssignments{explicitMembership, explicitMembership}
	require.Equal(t, wantAssignments, gotAssignments)

	// member 2
	req.SetUsername(member2)
	resp, err = c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 4)

	a3.Status.MemberCount = ptrToUint32(1)
	a3.Status.MemberListCount = ptrToUint32(0)
	a4.Status.MemberCount = ptrToUint32(1)
	a4.Status.MemberListCount = ptrToUint32(0)
	a5.Status.MemberCount = ptrToUint32(0)
	a5.Status.MemberListCount = ptrToUint32(0)

	gotACLs = mustFromProtoAll(t, resp.GetAccessLists()...)

	wantACLs = []*accesslist.AccessList{a1, a3, a4, a5}
	require.Empty(t, cmp.Diff(wantACLs, gotACLs, cmpOpts...))

	inheritedMembership := accesslistv1.UserAssignments_builder{
		MembershipType: accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED,
		OwnershipType:  accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
	}.Build()

	inheritedOwnership := accesslistv1.UserAssignments_builder{
		MembershipType: accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
		OwnershipType:  accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED,
	}.Build()

	gotAssignments = make([]*accesslistv1.UserAssignments, len(resp.GetAccessLists()))
	for i, al := range resp.GetAccessLists() {
		gotAssignments[i] = al.GetStatus().GetUserAssignments()
	}
	wantAssignments = []*accesslistv1.UserAssignments{inheritedMembership, explicitMembership, explicitMembership, inheritedOwnership}
	require.Equal(t, wantAssignments, gotAssignments)

	// member 2 pagination, default pageSize
	req.SetUsername(member2)
	req.SetPageSize(-1)

	resp, err = c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 4)

	gotACLs = mustFromProtoAll(t, resp.GetAccessLists()...)
	wantACLs = []*accesslist.AccessList{a1, a3, a4, a5}
	require.Empty(t, cmp.Diff(wantACLs, gotACLs, cmpOpts...))

	// member 2 pagination
	req.SetUsername(member2)
	req.SetPageToken("3")
	req.SetPageSize(2)

	resp, err = c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 2)

	gotACLs = mustFromProtoAll(t, resp.GetAccessLists()...)
	wantACLs = []*accesslist.AccessList{a3, a4}
	require.Empty(t, cmp.Diff(wantACLs, gotACLs, cmpOpts...))

	// member 3
	req = accesslistv1.ListUserAccessListsRequest_builder{
		Username: member3,
	}.Build()
	resp, err = c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Empty(t, resp.GetAccessLists())

	// owner
	req.SetUsername(ownerUser)
	resp, err = c.svc.ListUserAccessLists(c.userCtx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetAccessLists(), 4)

	gotACLs = mustFromProtoAll(t, resp.GetAccessLists()...)
	wantACLs = []*accesslist.AccessList{a1, a2, a3, a4}
	require.Empty(t, cmp.Diff(wantACLs, gotACLs, cmpOpts...))

	explicitOwnership := accesslistv1.UserAssignments_builder{
		MembershipType: accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
		OwnershipType:  accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT,
	}.Build()

	gotAssignments = make([]*accesslistv1.UserAssignments, len(resp.GetAccessLists()))
	for i, al := range resp.GetAccessLists() {
		gotAssignments[i] = al.GetStatus().GetUserAssignments()
	}
	wantAssignments = []*accesslistv1.UserAssignments{explicitOwnership, explicitOwnership, explicitOwnership, explicitOwnership}
	require.Equal(t, wantAssignments, gotAssignments)
}

func getAllAccessListMembers(ctx context.Context, service *Service) ([]*accesslist.AccessListMember, error) {
	allMembers, err := stream.Collect(
		stream.FilterMap(
			clientutils.Resources(ctx, func(ctx context.Context, pageSize int, token string) ([]*accesslistv1.Member, string, error) {
				resp, err := service.ListAllAccessListMembers(ctx, accesslistv1.ListAllAccessListMembersRequest_builder{
					PageSize:  int32(pageSize),
					PageToken: token,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				return resp.GetMembers(), resp.GetNextPageToken(), nil
			}), func(m *accesslistv1.Member) (*accesslist.AccessListMember, bool) {
				out, err := conv.FromMemberProto(m)
				if err != nil {
					return nil, false
				}

				return out, true
			}),
	)
	return allMembers, trace.Wrap(err)
}

func getAccessListMembers(ctx context.Context, t *testing.T, service *Service, accessListName string, pageSize int) []*accesslist.AccessListMember {
	t.Helper()

	members, err := stream.Collect(stream.FilterMap(
		clientutils.ResourcesWithPageSize(ctx,
			func(ctx context.Context, pageSize int, token string) ([]*accesslistv1.Member, string, error) {
				resp, err := service.ListAccessListMembers(ctx, accesslistv1.ListAccessListMembersRequest_builder{
					PageSize:   int32(pageSize),
					PageToken:  token,
					AccessList: accessListName,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				return resp.GetMembers(), resp.GetNextPageToken(), nil
			},
			pageSize),
		func(m *accesslistv1.Member) (*accesslist.AccessListMember, bool) {
			out, err := conv.FromMemberProto(m)
			if err != nil {
				return nil, false
			}

			return out, true
		},
	))
	require.NoError(t, err)
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
		accessList, err := svc.GetAccessList(ctx, accesslistv1.GetAccessListRequest_builder{
			Name: review.Spec.AccessList,
		}.Build())
		require.NoError(t, err)

		resp, err := svc.CreateAccessListReview(ctx, accesslistv1.CreateAccessListReviewRequest_builder{
			Review: conv.ToReviewProto(review),
		}.Build())
		require.NoError(t, err)

		// Calculate the expected number of days past.
		expectedDaysPast := int32(svc.clock.Now().Sub(accessList.GetSpec().GetAudit().GetNextAuditDate().AsTime()).Hours() / 24)

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
			require.Equal(t, review.Spec.Changes.ScopedRemovedMembers, event.ScopedRemovedMembers)
		})

		expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListReviewCreate) {
			require.Equal(t, review.Spec.AccessList, event.AccessListReviewCreate.Metadata.Id)
			require.Equal(t, expectedDaysPast, event.AccessListReviewCreate.DaysPastNextAuditDate)
			require.Equal(t, review.Spec.Changes.MembershipRequirementsChanged != nil, event.AccessListReviewCreate.MembershipRequirementsChanged)
			require.Equal(t, review.Spec.Changes.ReviewFrequencyChanged.String() != "", event.AccessListReviewCreate.ReviewFrequencyChanged)
			require.Equal(t, review.Spec.Changes.ReviewDayOfMonthChanged.String() != "", event.AccessListReviewCreate.ReviewDayOfMonthChanged)
			require.Equal(t, int32(len(review.Spec.Changes.RemovedMembers)+len(review.Spec.Changes.ScopedRemovedMembers)), event.AccessListReviewCreate.NumberOfRemovedMembers)
		})

		// Update info for the review.
		review.Spec.Reviewers = []string{username}
		review.Spec.ReviewDate = svc.clock.Now()
		review.SetName(resp.GetReviewName())
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
		resp, err := service.ListAccessListReviews(ctx, accesslistv1.ListAccessListReviewsRequest_builder{
			PageSize:   int32(pageSize),
			NextToken:  nextToken,
			AccessList: accessListName,
		}.Build())
		require.NoError(t, err)

		for _, review := range resp.GetReviews() {
			reviews = append(reviews, mustFromReviewProto(t, review))
		}

		nextToken = resp.GetNextToken()
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

func genScopedUserContext(ctx context.Context, username string, scope string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: scope,
			}.Build(),
		},
	})
}

func scopedAccessListName(scope, name string) accesslists.NormalizedSQN {
	return accesslists.NormalizeSQN(apiscopes.QualifiedName{Scope: scope, Name: name})
}

func accessListSQNsFromProto(accessLists []*accesslistv1.AccessList) []accesslists.NormalizedSQN {
	out := make([]accesslists.NormalizedSQN, 0, len(accessLists))
	for _, accessList := range accessLists {
		out = append(out, accesslists.NormalizeSQN(apiscopes.QualifiedName{
			Scope: accessList.GetScope(),
			Name:  accessList.GetHeader().GetMetadata().GetName(),
		}))
	}
	return out
}

type accessListOptions struct {
	typ accesslist.Type
}

type accessListOpt func(*accessListOptions)

func withType(typ accesslist.Type) accessListOpt {
	return func(o *accessListOptions) {
		o.typ = typ
	}
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock, opts ...accessListOpt) *accesslist.AccessList {
	return newScopedAccessList(t, accesslists.NormalizedSQN{Name: name}, clock, opts...)
}

func newScopedAccessList(t *testing.T, name accesslists.NormalizedSQN, clock clockwork.Clock, opts ...accessListOpt) *accesslist.AccessList {
	options := accessListOptions{}
	for _, o := range opts {
		o(&options)
	}

	// Default to an access list with the next audit date 1 year in the future,
	// and ownership/membership requirements if it's unscoped.
	spec := accesslist.Spec{
		Type: options.typ,
		Owners: []accesslist.Owner{
			{Name: ownerUser, Description: "owner user", MembershipKind: accesslist.MembershipKindUser},
			{Name: ownerUser2, Description: "owner user 2", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyWhere, Description: "deny where user", MembershipKind: accesslist.MembershipKindUser},
			{Name: testUserDenyAll, Description: "deny where user", MembershipKind: accesslist.MembershipKindUser},
		},
	}
	if name.Scope == "" {
		spec.MembershipRequires = accesslist.Requires{
			Roles: []string{"mrole1", "mrole2"},
			Traits: map[string][]string{
				"mtrait1": {"mvalue1", "mvalue2"},
				"mtrait2": {"mvalue3", "mvalue4"},
			},
		}
		spec.OwnershipRequires = accesslist.Requires{
			Roles: []string{"orole1", "orole2"},
			Traits: map[string][]string{
				"otrait1": {"ovalue1", "ovalue2"},
				"otrait2": {"ovalue3", "ovalue4"},
			},
		}
		spec.Grants = accesslist.Grants{
			Roles: []string{"grole1", "grole2"},
			Traits: map[string][]string{
				"gtrait1": {"gvalue1", "gvalue2"},
				"gtrait2": {"gvalue3", "gvalue4"},
			},
		}
	}
	return newScopedAccessListWithPartialSpec(t, name, clock.Now().Add(time.Hour*24*365), spec)
}

func newAccessListWithPartialSpec(t *testing.T, name string, nextAuditDate time.Time, spec accesslist.Spec) *accesslist.AccessList {
	return newScopedAccessListWithPartialSpec(t, accesslists.NormalizedSQN{Name: name}, nextAuditDate, spec)
}

func newScopedAccessListWithPartialSpec(t *testing.T, name accesslists.NormalizedSQN, nextAuditDate time.Time, spec accesslist.Spec) *accesslist.AccessList {
	t.Helper()

	audit := accesslist.Audit{}
	if spec.Type.IsReviewable() {
		audit = accesslist.Audit{
			NextAuditDate: nextAuditDate,
			Notifications: accesslist.Notifications{
				Start: 336 * time.Hour, // Two weeks.
			},
		}
	}

	accessList, err := accesslist.NewAccessListWithScope(
		header.Metadata{
			Name: name.Name,
		},
		accesslist.Spec{
			Title:              name.Name,
			Type:               spec.Type,
			Description:        "test access list",
			Owners:             spec.Owners,
			Audit:              audit,
			MembershipRequires: spec.MembershipRequires,
			OwnershipRequires:  spec.OwnershipRequires,
			Grants:             spec.Grants,
			OwnerGrants:        spec.OwnerGrants,
		},
		name.Scope,
	)
	require.NoError(t, err)
	accessList.Status = accesslist.Status{
		OwnerOf:                []string{},
		MemberOf:               []string{},
		CurrentUserAssignments: &accesslist.CurrentUserAssignments{},
	}

	return accessList
}

type newMemberOption func(*accesslist.AccessListMember)

func withOriginLabel(origin string) newMemberOption {
	return func(am *accesslist.AccessListMember) {
		am.SetOrigin(origin)
	}
}

func withExpire(t time.Time) newMemberOption {
	return func(am *accesslist.AccessListMember) {
		am.Spec.Expires = t
	}
}

func newTestOktaPlugin(t *testing.T, syncAccessLists bool) *types.PluginV1 {
	t.Helper()

	return &types.PluginV1{
		Metadata: types.Metadata{Name: "okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "http://localhost",
					SyncSettings: &types.PluginOktaSyncSettings{
						SyncUsers:       true,
						SyncAccessLists: syncAccessLists,
						SsoConnectorId:  "test-connector",
						DefaultOwners:   []string{"admin"},
					},
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{"plugin": "okta"},
				},
			},
		},
	}
}

func newAccessListMember(t *testing.T, accessListName, memberName string, memberKind string, clock clockwork.Clock, opts ...newMemberOption) *accesslist.AccessListMember {
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
			AddedBy:        testUser,
			MembershipKind: memberKind,
		},
	)
	require.NoError(t, err)

	for _, opt := range opts {
		opt(member)
	}

	return member
}

func newScopedAccessListMember(t *testing.T, parentListName, memberName accesslists.NormalizedSQN, memberKind string, clock clockwork.Clock, opts ...newMemberOption) *accesslist.AccessListMember {
	t.Helper()

	accessListName := parentListName.Name
	if parentListName.Scope != "" {
		accessListName = parentListName.String()
	}
	name := memberName.Name
	if memberKind == accesslist.MembershipKindScopedList {
		name = memberName.String()
	}

	member, err := accesslist.NewAccessListMemberWithScope(
		header.Metadata{Name: name},
		accesslist.AccessListMemberSpec{
			AccessList:     accessListName,
			Name:           name,
			Joined:         clock.Now().UTC(),
			Expires:        clock.Now().UTC().Add(24 * time.Hour),
			Reason:         "because",
			AddedBy:        testUser,
			MembershipKind: memberKind,
		},
		parentListName.Scope,
	)
	require.NoError(t, err)

	for _, opt := range opts {
		opt(member)
	}

	return member
}

func newAccessListMemberWithIneligibleReason(t *testing.T, accessListName, memberName string, clock clockwork.Clock, kind string, ineligibleReason string) *accesslist.AccessListMember {
	t.Helper()

	member := newAccessListMember(t, accessListName, memberName, kind, clock)
	member.Spec.IneligibleStatus = ineligibleReason

	return member
}

func createDisplayUser(t *testing.T, identity services.Identity, username, primary, secondary string) {
	t.Helper()

	user, err := identity.GetUser(t.Context(), username, false)
	if trace.IsNotFound(err) {
		user, err = types.NewUser(username)
		require.NoError(t, err)
	} else {
		require.NoError(t, err)
	}
	traits := map[string][]string{}
	if primary != "" {
		traits[testDisplayNameTrait] = []string{primary}
	}
	if secondary != "" {
		traits[testEmailTrait] = []string{secondary}
	}
	user.SetTraits(traits)
	_, err = identity.UpsertUser(t.Context(), user)
	require.NoError(t, err)
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

func getAccessListV2(t *testing.T, c testSvcComponents, name accesslists.NormalizedSQN) *accesslistv1.AccessList {
	t.Helper()

	resp, err := c.svc.GetAccessList(c.userCtx, accesslistv1.GetAccessListRequest_builder{
		Scope: name.Scope,
		Name:  name.Name,
	}.Build())
	require.NoError(t, err)
	return resp
}

func listAccessListMembersV2(t *testing.T, c testSvcComponents, accessListName accesslists.NormalizedSQN) []*accesslistv1.Member {
	t.Helper()

	members, err := stream.Collect(clientutils.Resources(c.userCtx, func(ctx context.Context, pageSize int, pageToken string) ([]*accesslistv1.Member, string, error) {
		resp, err := c.svc.ListAccessListMembers(ctx, accesslistv1.ListAccessListMembersRequest_builder{
			AccessListScope: accessListName.Scope,
			AccessList:      accessListName.Name,
			PageSize:        10,
			PageToken:       pageToken,
		}.Build())
		return resp.GetMembers(), resp.GetNextPageToken(), err
	}))
	require.NoError(t, err)
	return members
}

func memberNames(members []*accesslistv1.Member) []string {
	out := make([]string, 0, len(members))
	for _, member := range members {
		out = append(out, member.GetSpec().GetName())
	}
	sort.Strings(out)
	return out
}

func createAccessLists(t *testing.T, ctx context.Context, service *Service, emitter *eventstest.ChannelEmitter,
	usageEvents *usageEventsClient, accessLists []*accesslist.AccessList,
) {
	t.Helper()
	createAccessListsAndMembers(t, ctx, service, emitter, usageEvents, accessLists, nil)
}

func createAccessListsAndMembers(t *testing.T, ctx context.Context, service *Service, emitter *eventstest.ChannelEmitter,
	usageEvents *usageEventsClient, accessLists []*accesslist.AccessList, members []*accesslist.AccessListMember,
) {
	t.Helper()

	for _, al := range accessLists {
		_, err := service.UpsertAccessList(ctx, accesslistv1.UpsertAccessListRequest_builder{AccessList: conv.ToProto(al)}.Build())
		require.NoError(t, err, trace.DebugReport(err))
		expectEvent(t, events.AccessListCreateSuccessCode, emitter, func(event *apievents.AccessListCreate) {
			require.True(t, event.Success)
			require.Equal(t, al.GetName(), event.Name)
			require.Equal(t, al.GetScope(), event.Scope)
		})
		if usageEvents != nil {
			expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListCreate) {
				require.Equal(t, al.GetName(), event.AccessListCreate.Metadata.Id)
				require.Equal(t, al.GetScope(), event.AccessListCreate.Metadata.Scope)
			})
		}
	}

	for _, member := range members {
		_, err := service.UpsertAccessListMember(ctx, accesslistv1.UpsertAccessListMemberRequest_builder{Member: conv.ToMemberProto(member)}.Build())
		require.NoError(t, err)
		memberSQN, err := accesslists.MemberScopeQualifiedName(member)
		require.NoError(t, err)
		expectEvent(t, events.AccessListMemberCreateSuccessCode, emitter, func(event *apievents.AccessListMemberCreate) {
			require.True(t, event.Success)
			require.Equal(t, member.GetScope(), event.AccessListMemberMetadata.AccessListScope)
			require.Equal(t, memberSQN.Scope, event.AccessListMemberMetadata.Members[0].MemberScope)
			if usageEvents != nil {
				expectUsageEvent(t, usageEvents, func(event *usageeventsv1.UsageEventOneOf_AccessListMemberCreate) {
					require.Equal(t, member.Spec.AccessList, event.AccessListMemberCreate.Metadata.Id)
					require.Equal(t, member.GetScope(), event.AccessListMemberCreate.Metadata.Scope)
				})
			}
		})
	}
}

func newAccessListReview(t *testing.T, accessListName string) *accesslist.Review {
	return newScopedAccessListReview(t, accesslists.NormalizedSQN{Name: accessListName})
}

func newScopedAccessListReview(t *testing.T, accessListName accesslists.NormalizedSQN) *accesslist.Review {
	t.Helper()

	review, err := accesslist.NewReviewWithScope(
		header.Metadata{
			Name: "dummy", // This will be overwritten by the service.
		},
		accesslist.ReviewSpec{
			AccessList: accessListName.String(),
			Reviewers:  []string{"dummy"}, // This will be overwritten as well.
			ReviewDate: time.Now(),        // This will be overwritten by the service.
		},
		accessListName.Scope,
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
