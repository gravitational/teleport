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

func TestGetAccessLists(t *testing.T) {
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

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

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

func TestListAccessLists(t *testing.T) {
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

	_, err := svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a4)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a5)})
	require.NoError(t, err)

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

func TestUpsertAccessList(t *testing.T) {
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

func TestGetAccessList(t *testing.T) {
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

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

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

func TestDeleteAccessList(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	get, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)

	_, err = svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteAllAccessLists(t *testing.T) {
	t.Parallel()

	ctx, _, svc, clock := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1", clock)
	a2 := newAccessList(t, "2", clock)
	a3 := newAccessList(t, "3", clock)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

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

func mustFromProto(t *testing.T, accessList *accesslistv1.AccessList) *accesslist.AccessList {
	t.Helper()

	out, err := conv.FromProto(accessList)
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
