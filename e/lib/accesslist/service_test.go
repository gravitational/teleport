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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	testUser  = "test-user"
	ownerUser = "owner-user"
	member1   = "member1"
	member2   = "member2"
	member3   = "member3"
)

// cmpOpts are general cmpOpts for all comparisons.
var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "ID"),
	cmpopts.SortSlices(func(a, b *accesslist.AccessList) bool {
		return a.GetName() < b.GetName()
	}),
}

func TestService_GetAccessLists(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	// a2 will only have member1 as a member.
	a2.Spec.Members = []accesslist.Member{
		{
			Name:    member1,
			Joined:  clock.Now().UTC(),
			Expires: clock.Now().UTC().Add(24 * time.Hour),
			Reason:  "because",
			AddedBy: testUser,
		},
	}

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2, a3}, nil)

	// members should always be stripped from getall/list endpoints
	a1.Spec.Members = []accesslist.Member{}
	a2.Spec.Members = []accesslist.Member{}
	a3.Spec.Members = []accesslist.Member{}

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// owner should only see a1 and a2
	getResp, err = svc.GetAccessLists(ownerCtx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	// member2 should only see a1 and a3, should have no membership information.
	a1.Spec.Members = []accesslist.Member{}
	a3.Spec.Members = []accesslist.Member{}

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

	ctx, ownerCtx, svc, clock := initSvc(t)

	accessLists := listAccessLists(ctx, t, svc, 1)
	require.Empty(t, accessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)
	a4 := newAccessList(t, "4", clock)
	a5 := newAccessList(t, "5", clock)

	// a2 will only have member1 as a member.
	a2.Spec.Members = []accesslist.Member{
		{
			Name:    member1,
			Joined:  clock.Now().UTC(),
			Expires: clock.Now().UTC().Add(24 * time.Hour),
			Reason:  "because",
			AddedBy: testUser,
		},
	}

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2, a3, a4, a5}, nil)

	// members should always be stripped from getall/list endpoints
	a1.Spec.Members = []accesslist.Member{}
	a2.Spec.Members = []accesslist.Member{}
	a3.Spec.Members = []accesslist.Member{}
	a4.Spec.Members = []accesslist.Member{}
	a5.Spec.Members = []accesslist.Member{}

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
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	// User tries to create a new access list that they own. Shouldn't work.
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	a2.Spec.Members = append(a2.Spec.Members,
		accesslist.Member{
			Name:    "member3",
			Joined:  clock.Now().UTC(),
			Expires: clock.Now().UTC().Add(24 * time.Hour),
			Reason:  "because",
			AddedBy: "test-user2",
		},
	)

	// User tries to add a user to an access list that they own. This should work.
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	// Make sure added by is overwritten during access list update.
	a2.Spec.Members[len(a2.Spec.Members)-1].AddedBy = ownerUser

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	// Owner should be able to modify the audit.
	a2.Spec.Audit.Frequency = 2080 * time.Hour
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	// Owner should be able to modify the membership requires.
	a2.Spec.MembershipRequires.Roles = append(a2.Spec.MembershipRequires.Roles, "new")
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	// Owner should not be able to modify anything else. We'll test by changing ownership roles.
	a2.Spec.OwnershipRequires.Roles = append(a2.Spec.OwnershipRequires.Roles, "new")
	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsAccessDenied(err))

	// User tries to add a duplicate user to an access list that they own. This should not work.
	a2.Spec.OwnershipRequires.Roles = a2.Spec.OwnershipRequires.Roles[:len(a2.Spec.OwnershipRequires.Roles)-1]
	a2.Spec.Members = append(a2.Spec.Members,
		accesslist.Member{
			Name:    "member3",
			Joined:  clock.Now().UTC(),
			Expires: clock.Now().UTC().Add(24 * time.Hour),
			Reason:  "because",
			AddedBy: "test-user2",
		},
	)

	_, err = svc.UpsertAccessList(ownerCtx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.True(t, trace.IsBadParameter(err))
}

func TestService_GetAccessList(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	// a2 will only have member1 as a member.
	a2.Spec.Members = []accesslist.Member{
		{
			Name:    member1,
			Joined:  clock.Now().UTC(),
			Expires: clock.Now().UTC().Add(24 * time.Hour),
			Reason:  "because",
			AddedBy: testUser,
		},
	}

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2, a3}, nil)

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, mustFromProto(t, get), cmpOpts...))

	get, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, mustFromProto(t, get), cmpOpts...))

	// member2 can't see a2
	memberCtx := genUserContext(context.Background(), member2, []string{"mrole1", "mrole2"}, map[string][]string{
		"mtrait1": {"mvalue1", "mvalue2"},
		"mtrait2": {"mvalue3", "mvalue4"},
	})

	get, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, get.Spec.Members)

	_, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.True(t, trace.IsAccessDenied(err))

	get, err = svc.GetAccessList(memberCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.NoError(t, err)
	require.Empty(t, get.Spec.Members)

	// owner can't see a3
	get, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.NotEmpty(t, get.Spec.Members)

	get, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a2.GetName()})
	require.NoError(t, err)
	require.NotEmpty(t, get.Spec.Members)

	_, err = svc.GetAccessList(ownerCtx, &accesslistv1.GetAccessListRequest{Name: a3.GetName()})
	require.True(t, trace.IsAccessDenied(err))
}

func TestService_DeleteAccessList(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1}, nil)

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)

	_, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.True(t, trace.IsNotFound(err))
}

func TestService_DeleteAllAccessLists(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2, a3}, nil)

	// members should always be stripped from getall/list endpoints
	a1.Spec.Members = []accesslist.Member{}
	a2.Spec.Members = []accesslist.Member{}
	a3.Spec.Members = []accesslist.Member{}

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	_, err = svc.DeleteAllAccessLists(ctx, &accesslistv1.DeleteAllAccessListsRequest{})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)
}

func initSvc(t *testing.T) (userContext context.Context, ownerContext context.Context, svc *Service, clock clockwork.Clock) {
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

	accessPoint := struct {
		services.ClusterConfiguration
		services.Trust
		services.RoleGetter
		services.UserGetter
	}{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
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

	role, err := types.NewRole("access-lists", types.RoleSpecV6{
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
	roleSvc.CreateRole(ctx, role)
	require.NoError(t, err)

	mrole, err := types.NewRole("mrole1", types.RoleSpecV6{})
	require.NoError(t, err)
	roleSvc.CreateRole(ctx, mrole)

	mrole, err = types.NewRole("mrole2", types.RoleSpecV6{})
	require.NoError(t, err)
	roleSvc.CreateRole(ctx, mrole)

	orole, err := types.NewRole("orole1", types.RoleSpecV6{})
	require.NoError(t, err)
	roleSvc.CreateRole(ctx, orole)

	orole, err = types.NewRole("orole2", types.RoleSpecV6{})
	require.NoError(t, err)
	roleSvc.CreateRole(ctx, orole)

	user, err := types.NewUser(testUser)
	require.NoError(t, err)
	user.AddRole(role.GetName())

	owner, err := types.NewUser(ownerUser)
	require.NoError(t, err)

	require.NoError(t, userSvc.CreateUser(user))
	require.NoError(t, userSvc.CreateUser(owner))

	storage, err := local.NewAccessListService(backend, clock)
	require.NoError(t, err)
	svc, err = NewService(ServiceConfig{
		Authorizer:  authorizer,
		AccessLists: storage,
		Clock:       clock,
	})
	require.NoError(t, err)

	member1, err := types.NewUser(member1)
	require.NoError(t, err)
	require.NoError(t, userSvc.CreateUser(member1))

	member2, err := types.NewUser(member2)
	require.NoError(t, err)
	require.NoError(t, userSvc.CreateUser(member2))

	member3, err := types.NewUser(member3)
	require.NoError(t, err)
	require.NoError(t, userSvc.CreateUser(member3))

	return genUserContext(ctx, user.GetName(), []string{role.GetName()}, nil),
		genUserContext(ctx, owner.GetName(), []string{"orole1", "orole2"}, map[string][]string{
			"otrait1": {"ovalue1", "ovalue2"},
			"otrait2": {"ovalue3", "ovalue4"},
		}), svc, clock
}

func TestService_ListAccessListMembers(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	// a3 will have different ownership requirements.
	a3.Spec.OwnershipRequires.Roles = []string{"non-existent-role1"}

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)
	a1m2 := newAccessListMember(t, a1.GetName(), "user2", clock)
	a2m1 := newAccessListMember(t, a2.GetName(), "user1", clock)
	a2m2 := newAccessListMember(t, a2.GetName(), "user2", clock)
	a3m1 := newAccessListMember(t, a3.GetName(), "user1", clock)
	a3m2 := newAccessListMember(t, a3.GetName(), "user2", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2, a3}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2, a3m1, a3m2})

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

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)
	a1m2 := newAccessListMember(t, a1.GetName(), "user2", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1}, []*accesslist.AccessListMember{a1m1, a1m2})

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
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	_, err := svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)

	require.Equal(t, a1m1.Spec.AddedBy, testUser)

	got, err := svc.UpsertAccessListMember(ownerCtx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(a1m1)})
	require.NoError(t, err)

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

	want.Spec.Joined = oldJoined
	got, err = svc.GetAccessListMember(ctx, &accesslistv1.GetAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(want, mustFromMemberProto(t, got), cmpOpts...))
}

func TestService_DeleteAccessListMember(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)
	a1m2 := newAccessListMember(t, a1.GetName(), "user2", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1}, []*accesslist.AccessListMember{a1m1, a1m2})

	// Admin should be able to delete members
	_, err := svc.DeleteAccessListMember(ctx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m1.GetName()})
	require.NoError(t, err)

	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, cmp.Diff([]*accesslist.AccessListMember{a1m2}, members, cmpOpts...))

	// owner should be able to delete members
	_, err = svc.DeleteAccessListMember(ownerCtx, &accesslistv1.DeleteAccessListMemberRequest{AccessList: a1.GetName(), MemberName: a1m2.GetName()})
	require.NoError(t, err)
	members = listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, members)
}

func TestService_DeleteAllAccessListMembersForAccessList(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)
	a1m2 := newAccessListMember(t, a1.GetName(), "user2", clock)
	a2m1 := newAccessListMember(t, a2.GetName(), "user1", clock)
	a2m2 := newAccessListMember(t, a2.GetName(), "user2", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// Admin should be able to delete members
	_, err := svc.DeleteAllAccessListMembersForAccessList(ctx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a1.GetName()})
	require.NoError(t, err)

	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, members)

	// owner should be able to delete members
	_, err = svc.DeleteAllAccessListMembersForAccessList(ownerCtx, &accesslistv1.DeleteAllAccessListMembersForAccessListRequest{AccessList: a2.GetName()})
	require.NoError(t, err)

	members = listAllAccessListMembers(ctx, t, svc, a2.GetName(), 1)
	require.Empty(t, members)
}

func TestService_DeleteAllAccessListMembers(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)

	a1m1 := newAccessListMember(t, a1.GetName(), "user1", clock)
	a1m2 := newAccessListMember(t, a1.GetName(), "user2", clock)
	a2m1 := newAccessListMember(t, a2.GetName(), "user1", clock)
	a2m2 := newAccessListMember(t, a2.GetName(), "user2", clock)

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2}, []*accesslist.AccessListMember{a1m1, a1m2, a2m1, a2m2})

	// owners can't use this endpoint
	_, err := svc.DeleteAllAccessListMembers(ownerCtx, &accesslistv1.DeleteAllAccessListMembersRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// Admin should be able to delete all members
	_, err = svc.DeleteAllAccessListMembers(ctx, &accesslistv1.DeleteAllAccessListMembersRequest{})
	require.NoError(t, err)

	members := listAllAccessListMembers(ctx, t, svc, a1.GetName(), 1)
	require.Empty(t, members)

	members = listAllAccessListMembers(ctx, t, svc, a2.GetName(), 1)
	require.Empty(t, members)
}

func TestService_AuthOrIsOwner(t *testing.T) {
	t.Parallel()

	ctx, ownerCtx, svc, clock := initSvc(t)
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

	createAccessListsAndMembers(t, ctx, svc, []*accesslist.AccessList{a1, a2}, nil)

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
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			test.wantErr(t, svc.authOrIsOwner(test.ctx, test.accessListName, types.VerbRead))
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
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				Frequency: time.Hour,
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
			Members: []accesslist.Member{
				{
					Name:    member1,
					Joined:  clock.Now().UTC(),
					Expires: clock.Now().UTC().Add(24 * time.Hour),
					Reason:  "because",
					AddedBy: testUser,
				},
				{
					Name:    member2,
					Joined:  clock.Now().UTC(),
					Expires: clock.Now().UTC().Add(24 * time.Hour),
					Reason:  "because again",
					AddedBy: testUser,
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

func mustFromProto(t *testing.T, accessList *accesslistv1.AccessList) *accesslist.AccessList {
	t.Helper()

	out, err := conv.FromProto(accessList)
	require.NoError(t, err)

	return out
}

func mustFromMemberProto(t *testing.T, member *accesslistv1.Member) *accesslist.AccessListMember {
	t.Helper()

	out, err := conv.FromMemberProto(member)
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

func createAccessListsAndMembers(t *testing.T, ctx context.Context, service *Service, accessLists []*accesslist.AccessList, members []*accesslist.AccessListMember) {
	t.Helper()

	for _, al := range accessLists {
		_, err := service.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(al)})
		require.NoError(t, err)
	}

	for _, member := range members {
		_, err := service.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{Member: conv.ToMemberProto(member)})
		require.NoError(t, err)
	}
}
