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

package statichostuser

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	convertv1 "github.com/gravitational/teleport/api/types/userprovisioning/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

type authorizerFactory func(t *testing.T, client localClient) authz.Authorizer

func staticHostUserName(i int) string {
	return fmt.Sprintf("user-%d", i)
}

func makeStaticHostUser(i int) *userprovisioningpb.StaticHostUser {
	name := staticHostUserName(i)
	return convertv1.ToProto(userprovisioning.NewStaticHostUser(&headerv1.Metadata{
		Name: name,
	}, userprovisioning.Spec{
		Login:  name,
		Groups: []string{"foo", "bar"},
		NodeLabels: types.Labels{
			"foo": {"bar"},
		},
	}))
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
				hostUser.Spec.Login = "bob"
				_, err = svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: convertv1.ToProto(hostUser),
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
		tc := tc
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
		tc := tc
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
	ctx, resourceSvc, localSvc := initSvc(t, authorizer)
	err := request(ctx, resourceSvc, localSvc)
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

func initSvc(t *testing.T, authorizerFn func(t *testing.T, client localClient) authz.Authorizer) (context.Context, *Service, *local.StaticHostUserService) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)
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
	for i := 0; i < 10; i++ {
		hostUser := makeStaticHostUser(i)
		hostUserProto, err := convertv1.FromProto(hostUser)
		require.NoError(t, err)
		_, err = localResourceService.CreateStaticHostUser(ctx, hostUserProto)
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

	resourceSvc, err := NewService(ServiceConfig{
		Authorizer: authorizerFn(t, client),
		Backend:    localResourceService,
		Cache:      localResourceService,
	})
	require.NoError(t, err)

	return ctx, resourceSvc, localResourceService
}
