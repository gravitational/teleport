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

// cmpOpts are general cmpOpts for all comparisons.
var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "ID"),
	cmpopts.SortSlices(func(a, b *accesslist.AccessList) bool {
		return a.GetName() < b.GetName()
	}),
}

func TestGetAccessLists(t *testing.T) {
	ctx, svc := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1")
	a2 := newAccessList(t, "2")
	a3 := newAccessList(t, "3")

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))
}

func TestGetAccessList(t *testing.T) {
	ctx, svc := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1")
	a2 := newAccessList(t, "2")
	a3 := newAccessList(t, "3")

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
}

func TestDeleteAccessList(t *testing.T) {
	ctx, svc := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1")

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
	ctx, svc := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1")
	a2 := newAccessList(t, "2")
	a3 := newAccessList(t, "3")

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToProto(a3)})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*accesslist.AccessList{a1, a2, a3}, mustFromProtoAll(t, getResp.AccessLists...), cmpOpts...))

	_, err = svc.DeleteAllAccessLists(ctx, &accesslistv1.DeleteAllAccessListsRequest{})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)
}

func initSvc(t *testing.T) (context.Context, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
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

	user, err := types.NewUser("test-user")
	user.AddRole(role.GetName())
	require.NoError(t, err)
	userSvc.CreateUser(user)
	require.NoError(t, err)

	svc, err := NewService(ServiceConfig{
		Backend:    backend,
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	ctx = authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})

	return ctx, svc
}

func newAccessList(t *testing.T, name string) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
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
					Name:    "member1",
					Joined:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because",
					AddedBy: "test-user1",
				},
				{
					Name:    "member2",
					Joined:  time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because again",
					AddedBy: "test-user2",
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
