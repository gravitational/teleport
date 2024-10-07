/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package userloginstatev1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	userloginstatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
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
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
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

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", nil, stRoles, stTraits, stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", nil, stRoles, stTraits, stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1, uls2}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))

	_, err = svc.GetUserLoginStates(noAccessCtx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.True(t, trace.IsAccessDenied(err))
}

func TestUpsertUserLoginStates(t *testing.T) {
	t.Parallel()

	ctx, noAccessCtx, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", nil, stRoles, stTraits, stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", nil, stRoles, stTraits, stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(noAccessCtx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.True(t, trace.IsAccessDenied(err))

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))
}

func TestGetUserLoginState(t *testing.T) {
	t.Parallel()

	ctx, noAccessCtx, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", nil, stRoles, stTraits, stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	get, err := svc.GetUserLoginState(ctx, &userloginstatepb.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(uls1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.GetUserLoginState(noAccessCtx, &userloginstatepb.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.True(t, trace.IsAccessDenied(err))
}

func TestDeleteUserLoginState(t *testing.T) {
	t.Parallel()

	ctx, _, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", nil, stRoles, stTraits, stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	get, err := svc.GetUserLoginState(ctx, &userloginstatepb.GetUserLoginStateRequest{
		Name: uls1.GetName(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(uls1, mustFromProto(t, get), cmpOpts...))

	_, err = svc.DeleteUserLoginState(ctx, &userloginstatepb.DeleteUserLoginStateRequest{Name: uls1.GetName()})
	require.NoError(t, err)

	_, err = svc.GetUserLoginState(ctx, &userloginstatepb.GetUserLoginStateRequest{Name: uls1.GetName()})
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteAllAccessLists(t *testing.T) {
	t.Parallel()

	ctx, _, svc := initSvc(t)

	getResp, err := svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)

	uls1 := newUserLoginState(t, "1", nil, stRoles, stTraits, stRoles, stTraits)
	uls2 := newUserLoginState(t, "2", nil, stRoles, stTraits, stRoles, stTraits)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls1)})
	require.NoError(t, err)

	_, err = svc.UpsertUserLoginState(ctx, &userloginstatepb.UpsertUserLoginStateRequest{UserLoginState: conv.ToProto(uls2)})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{uls1, uls2}, mustFromProtoAll(t, getResp.UserLoginStates...), cmpOpts...))

	_, err = svc.DeleteAllUserLoginStates(ctx, &userloginstatepb.DeleteAllUserLoginStatesRequest{})
	require.NoError(t, err)

	getResp, err = svc.GetUserLoginStates(ctx, &userloginstatepb.GetUserLoginStatesRequest{})
	require.NoError(t, err)
	require.Empty(t, getResp.UserLoginStates)
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
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
	userSvc, err := local.NewTestIdentityService(backend)
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

	user, err = userSvc.CreateUser(ctx, user)
	require.NoError(t, err)

	noAccessUser, err := types.NewUser(noAccessUser)
	require.NoError(t, err)
	noAccessUser, err = userSvc.CreateUser(ctx, noAccessUser)
	require.NoError(t, err)

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

func mustFromProto(t *testing.T, uls *userloginstatepb.UserLoginState) *userloginstate.UserLoginState {
	t.Helper()

	out, err := conv.FromProto(uls)
	require.NoError(t, err)

	return out
}

func mustFromProtoAll(t *testing.T, ulsList ...*userloginstatepb.UserLoginState) []*userloginstate.UserLoginState {
	t.Helper()

	var convertedUlsList []*userloginstate.UserLoginState
	for _, uls := range ulsList {
		out, err := conv.FromProto(uls)
		require.NoError(t, err)
		convertedUlsList = append(convertedUlsList, out)
	}

	return convertedUlsList
}

func newUserLoginState(t *testing.T, name string, labels map[string]string, originalRoles []string, originalTraits map[string][]string,
	roles []string, traits map[string][]string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(header.Metadata{
		Name:   name,
		Labels: labels,
	}, userloginstate.Spec{
		OriginalRoles:  originalRoles,
		OriginalTraits: originalTraits,
		Roles:          roles,
		Traits:         traits,
	})
	require.NoError(t, err)

	return uls
}
