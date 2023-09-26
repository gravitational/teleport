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

package userloginstate

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	userloginstatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	conv "github.com/gravitational/teleport/api/types/userloginstate/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	testUser     = "test-user"
	noAccessUser = "no-access-user"
)

var (
	// cmpOpts are general cmpOpts for all comparisons across the service tests.
	cmpOpts = []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		cmpopts.SortSlices(func(a, b *userloginstate.UserLoginState) bool {
			return a.GetName() < b.GetName()
		}),
	}

	stRoles = []string{"role1", "role2"}

	stTraits = trait.Traits{
		"key1": []string{"value1"},
		"key2": []string{"value2"},
	}
)

func TestGetUserLoginStates(t *testing.T) {
	t.Parallel()

	ctx, noAccessCtx, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1, uls2}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))

	_, err = svc.GetUserLoginStates(noAccessCtx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.True(t, trace.IsAccessDenied(err))
}

func TestUpsertUserLoginStates(t *testing.T) {
	t.Parallel()

	ctx, noAccessCtx, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(noAccessCtx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.True(t, trace.IsAccessDenied(err))

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))
}

func TestGetUserLoginState(t *testing.T) {
	t.Parallel()

	ctx, noAccessCtx, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	get, err := svc.GetUserLoginState(ctx, &userloginstatev1.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(uls1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.GetUserLoginState(noAccessCtx, &userloginstatev1.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))
}

func TestDeleteUserLoginState(t *testing.T) {
	t.Parallel()

	ctx, _, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	get, err := svc.GetUserLoginState(ctx, &userloginstatev1.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(uls1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.DeleteUserLoginState(ctx, &userloginstatev1.DeleteUserLoginStateRequest{Name: uls1.GetName()})
	require.NoError(t, err)

	_, err = svc.GetUserLoginState(ctx, &userloginstatev1.GetUserLoginStateRequest{Name: uls1.GetName()})
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteAllAccessLists(t *testing.T) {
	t.Parallel()

	ctx, _, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatev1.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1, uls2}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))

	_, err = svc.DeleteAllUserLoginStates(ctx, &userloginstatev1.DeleteAllUserLoginStatesRequest{})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatev1.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)
}

func initSvc(t *testing.T) (userContext context.Context, noAccessContext context.Context, svc *Service) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
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

	role, err := types.NewRole("user-login-state", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUserLoginState},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)
	roleSvc.CreateRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser(testUser)
	require.NoError(t, err)
	user.AddRole(role.GetName())

	require.NoError(t, userSvc.CreateUser(user))

	noAccessUser, err := types.NewUser(noAccessUser)
	require.NoError(t, err)
	require.NoError(t, userSvc.CreateUser(noAccessUser))

	storage, err := local.NewUserLoginStateService(backend)
	require.NoError(t, err)
	svc, err = NewService(ServiceConfig{
		Authorizer:      authorizer,
		UserLoginStates: storage,
		Clock:           clock,
	})
	require.NoError(t, err)

	return genUserContext(ctx, user.GetName(), []string{role.GetName()}),
		genUserContext(ctx, noAccessUser.GetName(), []string{}), svc
}

func genUserContext(ctx context.Context, username string, groups []string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
			Groups:   groups,
			Traits:   nil,
		},
	})
}

func mustFromProto(t *testing.T, uls *userloginstatev1.UserLoginState) *userloginstate.UserLoginState {
	t.Helper()

	out, err := conv.FromProto(uls)
	require.NoError(t, err)

	return out
}

func mustFromProtoAll(t *testing.T, ulsList ...*userloginstatev1.UserLoginState) []*userloginstate.UserLoginState {
	t.Helper()

	var convertedUlsList []*userloginstate.UserLoginState
	for _, uls := range ulsList {
		out, err := conv.FromProto(uls)
		require.NoError(t, err)
		convertedUlsList = append(convertedUlsList, out)
	}

	return convertedUlsList
}
