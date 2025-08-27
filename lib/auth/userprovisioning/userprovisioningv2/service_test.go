// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package userprovisioningv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

type authorizerFactory func(t *testing.T, client localClient) authz.Authorizer

func staticHostUserName(i int) string {
	return fmt.Sprintf("user-%d", i)
}

func makeStaticHostUser(i int) *userprovisioningpb.StaticHostUser {
	name := staticHostUserName(i)
	return userprovisioning.NewStaticHostUser(name, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups: []string{"foo", "bar"},
			},
		},
	})
}

func authorizeWithVerbs(verbs []string, mfaVerified bool) authorizerFactory {
	return func(t *testing.T, client localClient) authz.Authorizer {
		return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
			authzContext := authorizerForDummyUser(t, ctx, client, verbs)
			if mfaVerified {
				authzContext.AdminActionAuthState = authz.AdminActionAuthMFAVerified
			} else {
				authzContext.AdminActionAuthState = authz.AdminActionAuthUnauthorized
			}
			return authzContext, nil
		})
	}
}

func assertTraceErr(f func(error) bool) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...any) {
		require.Error(t, err)
		require.True(t, f(err), "unexpected error: %v", err)
	}
}

func TestStaticHostUserAuditEvents(t *testing.T) {
	env := initSvc(t, authorizeWithVerbs([]string{types.VerbDelete, types.VerbCreate, types.VerbUpdate}, true))

	ctx := context.Background()
	user, err := env.resourceService.CreateStaticHostUser(
		ctx,
		&userprovisioningpb.CreateStaticHostUserRequest{
			User: userprovisioning.NewStaticHostUser(
				"test",
				&userprovisioningpb.StaticHostUserSpec{
					Matchers: []*userprovisioningpb.Matcher{
						{
							Gid:                  1,
							Uid:                  2,
							Groups:               []string{"bar", "baz"},
							NodeLabelsExpression: `labels.dev == "test"`,
						},
					},
				},
			)},
	)
	require.NoError(t, err)

	select {
	case evt := <-env.emitter.C():
		expectedEvent := &apievents.StaticHostUserCreate{
			Metadata: apievents.Metadata{
				Type: events.StaticHostUserCreateEvent,
				Code: events.StaticHostUserCreateCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name: "test",
			},
			UserMetadata: apievents.UserMetadata{
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		}

		require.Empty(t, cmp.Diff(expectedEvent, evt, cmpopts.IgnoreFields(apievents.UserMetadata{}, "User")))

	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for static host user create event")
	}

	user, err = env.resourceService.UpdateStaticHostUser(
		ctx,
		&userprovisioningpb.UpdateStaticHostUserRequest{User: user},
	)
	require.NoError(t, err)

	select {
	case evt := <-env.emitter.C():
		expectedEvent := &apievents.StaticHostUserUpdate{
			Metadata: apievents.Metadata{
				Type: events.StaticHostUserUpdateEvent,
				Code: events.StaticHostUserUpdateCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name: "test",
			},
			UserMetadata: apievents.UserMetadata{
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		}

		require.Empty(t, cmp.Diff(expectedEvent, evt, cmpopts.IgnoreFields(apievents.UserMetadata{}, "User")))
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for static host user update event")
	}
	user, err = env.resourceService.UpsertStaticHostUser(
		ctx,
		&userprovisioningpb.UpsertStaticHostUserRequest{User: user},
	)
	require.NoError(t, err)

	select {
	case evt := <-env.emitter.C():
		expectedEvent := &apievents.StaticHostUserCreate{
			Metadata: apievents.Metadata{
				Type: events.StaticHostUserCreateEvent,
				Code: events.StaticHostUserCreateCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name: "test",
			},
			UserMetadata: apievents.UserMetadata{
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		}

		require.Empty(t, cmp.Diff(expectedEvent, evt, cmpopts.IgnoreFields(apievents.UserMetadata{}, "User")))
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for static host user upsert event")
	}
	_, err = env.resourceService.DeleteStaticHostUser(
		ctx,
		&userprovisioningpb.DeleteStaticHostUserRequest{Name: user.Metadata.Name},
	)
	require.NoError(t, err)

	select {
	case evt := <-env.emitter.C():
		expectedEvent := &apievents.StaticHostUserDelete{
			Metadata: apievents.Metadata{
				Type: events.StaticHostUserDeleteEvent,
				Code: events.StaticHostUserDeleteCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name: "test",
			},
			UserMetadata: apievents.UserMetadata{
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		}

		require.Empty(t, cmp.Diff(expectedEvent, evt, cmpopts.IgnoreFields(apievents.UserMetadata{}, "User")))
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for static host user delete event")
	}
}

func TestStaticHostUserCRUD(t *testing.T) {
	t.Parallel()

	accessTests := []struct {
		name       string
		request    func(ctx context.Context, svc *Service, localSvc *local.StaticHostUserService) error
		allowVerbs []string
	}{
		{
			name: "list",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.ListStaticHostUsers(ctx, &userprovisioningpb.ListStaticHostUsersRequest{})
				return err
			},
			allowVerbs: []string{types.VerbList, types.VerbRead},
		},
		{
			name: "get",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
					Name: staticHostUserName(0),
				})
				return err
			},
			allowVerbs: []string{types.VerbRead},
		},
		{
			name: "create",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.CreateStaticHostUser(ctx, &userprovisioningpb.CreateStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			allowVerbs: []string{types.VerbCreate},
		},
		{
			name: "update",
			request: func(ctx context.Context, svc *Service, localSvc *local.StaticHostUserService) error {
				// Get the initial user from the local service to bypass RBAC.
				hostUser, err := localSvc.GetStaticHostUser(ctx, staticHostUserName(0))
				if err != nil {
					return trace.Wrap(err)
				}
				hostUser.Spec.Matchers[0].Groups = []string{"baz", "quux"}
				_, err = svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: hostUser,
				})
				return err
			},
			allowVerbs: []string{types.VerbRead, types.VerbUpdate},
		},
		{
			name: "upsert",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.UpsertStaticHostUser(ctx, &userprovisioningpb.UpsertStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			allowVerbs: []string{types.VerbCreate, types.VerbUpdate},
		},
		{
			name: "delete",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.DeleteStaticHostUser(ctx, &userprovisioningpb.DeleteStaticHostUserRequest{
					Name: staticHostUserName(0),
				})
				return err
			},
			allowVerbs: []string{types.VerbDelete},
		},
	}

	for _, tc := range accessTests {
		t.Run(tc.name, func(t *testing.T) {

			t.Run("allow", func(t *testing.T) {
				t.Parallel()
				// Create authorizer with required verbs.
				authorizer := authorizeWithVerbs(tc.allowVerbs, true)
				// CRUD action should succeed.
				testStaticHostUserAccess(t, authorizer, tc.request, require.NoError)
			})

			t.Run("deny rbac", func(t *testing.T) {
				t.Parallel()
				// Create authorizer without required verbs.
				authorizer := authorizeWithVerbs(nil, true)
				// CRUD action should fail.
				testStaticHostUserAccess(t, authorizer, tc.request, assertTraceErr(trace.IsAccessDenied))
			})

			t.Run("deny mfa", func(t *testing.T) {
				t.Parallel()
				// Create authorizer without verified MFA.
				authorizer := authorizeWithVerbs(tc.allowVerbs, false)
				// CRUD action should fail.
				testStaticHostUserAccess(t, authorizer, tc.request, assertTraceErr(trace.IsAccessDenied))
			})
		})
	}

	otherTests := []struct {
		name    string
		request func(ctx context.Context, svc *Service, localSvc *local.StaticHostUserService) error
		verbs   []string
		assert  require.ErrorAssertionFunc
	}{
		{
			name: "get nonexistent resource",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
					Name: "fake",
				})
				return err
			},
			verbs:  []string{types.VerbRead},
			assert: assertTraceErr(trace.IsNotFound),
		},
		{
			name: "create resource twice",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.CreateStaticHostUser(ctx, &userprovisioningpb.CreateStaticHostUserRequest{
					User: makeStaticHostUser(0),
				})
				return err
			},
			verbs:  []string{types.VerbCreate},
			assert: assertTraceErr(trace.IsAlreadyExists),
		},
		{
			name: "delete nonexisting resource",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.DeleteStaticHostUser(ctx, &userprovisioningpb.DeleteStaticHostUserRequest{
					Name: staticHostUserName(10),
				})
				return err
			},
			verbs:  []string{types.VerbDelete},
			assert: assertTraceErr(trace.IsNotFound),
		},
		{
			name: "update with wrong revision",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: makeStaticHostUser(0),
				})
				return err
			},
			verbs:  []string{types.VerbUpdate},
			assert: assertTraceErr(trace.IsCompareFailed),
		},
		{
			name: "update nonexistent resource",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			verbs:  []string{types.VerbUpdate},
			assert: assertTraceErr(trace.IsCompareFailed),
		},
		{
			name: "upsert with update permission only",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.UpsertStaticHostUser(ctx, &userprovisioningpb.UpsertStaticHostUserRequest{
					User: makeStaticHostUser(0),
				})
				return err
			},
			verbs:  []string{types.VerbUpdate},
			assert: assertTraceErr(trace.IsAccessDenied),
		},
		{
			name: "upsert with create permission only",
			request: func(ctx context.Context, svc *Service, _ *local.StaticHostUserService) error {
				_, err := svc.UpsertStaticHostUser(ctx, &userprovisioningpb.UpsertStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			verbs:  []string{types.VerbCreate},
			assert: assertTraceErr(trace.IsAccessDenied),
		},
	}
	for _, tc := range otherTests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authorizer := authorizeWithVerbs(tc.verbs, true)
			testStaticHostUserAccess(t, authorizer, tc.request, tc.assert)
		})
	}
}

func testStaticHostUserAccess(
	t *testing.T,
	authorizer func(t *testing.T, client localClient) authz.Authorizer,
	request func(ctx context.Context, svc *Service, localSvc *local.StaticHostUserService) error,
	assert require.ErrorAssertionFunc,
) {
	env := initSvc(t, authorizer)
	err := request(context.Background(), env.resourceService, env.localService)
	assert(t, err)
}

func authorizerForDummyUser(t *testing.T, ctx context.Context, localClient localClient, roleVerbs []string) *authz.Context {
	const clusterName = "localhost"

	// Create role
	roleName := "role-" + uuid.NewString()
	var allowRules []types.Rule
	if len(roleVerbs) != 0 {
		allowRules = []types.Rule{
			{
				Resources: []string{types.KindStaticHostUser},
				Verbs:     roleVerbs,
			},
		}
	}
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: allowRules},
	})
	require.NoError(t, err)

	role, err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	user, err = localClient.CreateUser(ctx, user)
	require.NoError(t, err)

	localUser := authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	}
	authCtx, err := authz.ContextForLocalUser(ctx, localUser, localClient, clusterName, true)
	require.NoError(t, err)

	return authCtx
}

type localClient interface {
	authz.AuthorizerAccessPoint

	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
}

type testEnv struct {
	resourceService *Service

	localService *local.StaticHostUserService

	emitter *eventstest.ChannelEmitter
}

func initSvc(t *testing.T, authorizerFn func(t *testing.T, client localClient) authz.Authorizer) testEnv {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	roleSvc := local.NewAccessService(backend)
	userSvc, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)
	clusterSrv, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	caSrv := local.NewCAService(backend)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertClusterAuditConfig(ctx, types.DefaultClusterAuditConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	localResourceService, err := local.NewStaticHostUserService(backend)
	require.NoError(t, err)
	for i := range 10 {
		_, err := localResourceService.CreateStaticHostUser(ctx, makeStaticHostUser(i))
		require.NoError(t, err)
	}

	client := struct {
		*local.AccessService
		*local.IdentityService
		*local.ClusterConfigurationService
		*local.CA
	}{
		AccessService:               roleSvc,
		IdentityService:             userSvc,
		ClusterConfigurationService: clusterSrv,
		CA:                          caSrv,
	}

	emitter := eventstest.NewChannelEmitter(10)

	resourceSvc, err := NewService(ServiceConfig{
		Authorizer: authorizerFn(t, client),
		Emitter:    emitter,
		Backend:    localResourceService,
		Cache:      localResourceService,
	})
	require.NoError(t, err)

	return testEnv{
		resourceService: resourceSvc,
		localService:    localResourceService,
		emitter:         emitter,
	}
}
