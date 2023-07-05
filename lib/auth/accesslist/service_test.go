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
	"github.com/stretchr/testify/require"

	conv "github.com/gravitational/teleport/api/convert/teleport/accesslist/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAccessLists(t *testing.T) {
	ctx, svc := initSvc(t)

	getResp, err := svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.AccessLists)

	a1 := newAccessList(t, "1")
	a2 := newAccessList(t, "2")
	a3 := newAccessList(t, "3")

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToV1(a1)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToV1(a2)})
	require.NoError(t, err)

	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToV1(a3)})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*types.AccessList{a1, a2, a3}, mustFromV1All(t, getResp.AccessLists...),
		cmpopts.IgnoreFields(header.Metadata{}, "ID")))

	a1.SetExpiry(time.Now().Add(30 * time.Minute))
	_, err = svc.UpsertAccessList(ctx, &accesslistv1.UpsertAccessListRequest{AccessList: conv.ToV1(a1)})
	require.NoError(t, err)

	a, err := svc.GetAccessList(ctx, &accesslistv1.GetAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, mustFromV1(t, a.AccessList),
		cmpopts.IgnoreFields(header.Metadata{}, "ID")))

	_, err = svc.DeleteAccessList(ctx, &accesslistv1.DeleteAccessListRequest{Name: a1.GetName()})
	require.NoError(t, err)

	getResp, err = svc.GetAccessLists(ctx, &accesslistv1.GetAccessListsRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*types.AccessList{a2, a3}, mustFromV1All(t, getResp.AccessLists...),
		cmpopts.IgnoreFields(header.Metadata{}, "ID")))

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

func newAccessList(t *testing.T, name string) *types.AccessList {
	t.Helper()

	accessList, err := types.NewAccessList(
		header.Metadata{
			Name: name,
		},
		types.AccessListSpec{
			Description: "test access list",
			Owners: []types.AccessListOwner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: types.AccessListAudit{
				Frequency: time.Hour,
			},
			MembershipRequires: types.AccessListRequires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: types.AccessListRequires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: types.AccessListGrants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
			Members: []types.AccessListMember{
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

func mustFromV1(t *testing.T, accessList *accesslistv1.AccessList) *types.AccessList {
	t.Helper()

	out, err := conv.FromV1(accessList)
	require.NoError(t, err)

	return out
}

func mustFromV1All(t *testing.T, accessLists ...*accesslistv1.AccessList) []*types.AccessList {
	t.Helper()

	var convertedAccessLists []*types.AccessList
	for _, accessList := range accessLists {
		out, err := conv.FromV1(accessList)
		require.NoError(t, err)
		convertedAccessLists = append(convertedAccessLists, out)
	}

	return convertedAccessLists
}
