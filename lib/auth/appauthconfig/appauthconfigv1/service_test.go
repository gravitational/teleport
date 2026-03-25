/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package appauthconfigv1

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/appauthconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type testClient struct {
	services.Access
	services.ClusterConfiguration
	services.AppAuthConfig
	services.Identity
	services.Trust

	ServiceUnderTest *Service
	eventRecorder    *eventstest.MockRecorderEmitter
}

func TestAppAuthConfigCRUD(t *testing.T) {
	t.Parallel()

	tests := []accessTest{
		// Create
		{
			name: "CreateAppAuthConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				_, err := clt.ServiceUnderTest.CreateAppAuthConfig(ctx, &appauthconfigv1.CreateAppAuthConfigRequest{
					Config: newAppAuthConfigJWT(),
				})
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbCreate)},
			requireEvent:        &apievents.AppAuthConfigCreate{},
		},
		// Read
		{
			name: "GetAppAuthConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg1 := newAppAuthConfigJWT()
				_, err := clt.CreateAppAuthConfig(ctx, cfg1)
				require.NoError(t, err)
				_, err = clt.ServiceUnderTest.GetAppAuthConfig(ctx, &appauthconfigv1.GetAppAuthConfigRequest{
					Name: cfg1.Metadata.GetName(),
				})
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbRead)},
		},
		{
			name: "ListAppAuthConfigs",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				_, err := clt.CreateAppAuthConfig(ctx, newAppAuthConfigJWT())
				require.NoError(t, err)
				_, err = clt.CreateAppAuthConfig(ctx, newAppAuthConfigJWT())
				require.NoError(t, err)
				resp, err := clt.ServiceUnderTest.ListAppAuthConfigs(ctx, &appauthconfigv1.ListAppAuthConfigsRequest{})
				if err == nil {
					require.NotNil(t, resp)
					require.Len(t, resp.Configs, 2, "expected 2 inserted")
				}
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbRead, types.VerbList)},
		},
		// Update
		{
			name: "UpdateAppAuthConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg, err := clt.CreateAppAuthConfig(ctx, newAppAuthConfigJWT())
				require.NoError(t, err)
				firstRev := cfg.Metadata.Revision
				cfg, err = clt.ServiceUnderTest.UpdateAppAuthConfig(ctx, &appauthconfigv1.UpdateAppAuthConfigRequest{
					Config: cfg,
				})
				if err == nil {
					require.NotNil(t, cfg, "the updated resource should be returned")
					require.NotEqual(t, firstRev, cfg.Metadata.Revision, "the resource should have been updated")
				}
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbUpdate)},
			requireEvent:        &apievents.AppAuthConfigUpdate{},
		},
		{
			name: "UpsertAppAuthConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg := newAppAuthConfigJWT()
				_, err := clt.ServiceUnderTest.UpsertAppAuthConfig(ctx, &appauthconfigv1.UpsertAppAuthConfigRequest{
					Config: cfg,
				})
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbCreate, types.VerbUpdate)},
			requireEvent:        &apievents.AppAuthConfigCreate{},
		},
		// Delete
		{
			name: "DeleteAppAuthConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg, err := clt.CreateAppAuthConfig(ctx, newAppAuthConfigJWT())
				require.NoError(t, err)
				_, err = clt.ServiceUnderTest.DeleteAppAuthConfig(ctx, &appauthconfigv1.DeleteAppAuthConfigRequest{
					Name: cfg.Metadata.GetName(),
				})
				return err
			},
			requiredAccessRules: []types.Rule{appAuthConfigRule(types.VerbDelete)},
			requireEvent:        &apievents.AppAuthConfigDelete{},
		},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func appAuthConfigRule(verbs ...string) types.Rule {
	return types.Rule{
		Resources: []string{types.KindAppAuthConfig},
		Verbs:     verbs,
	}
}

type accessTest struct {
	name     string
	actionFn func(t *testing.T, ctx context.Context, clt testClient) error
	// requiredAccessRules are rules that are expected to be allowed and not
	// denied to perform actionFn without an access error.
	requiredAccessRules []types.Rule
	// requireEvent must be emitted if the action succeeds.
	requireEvent apievents.AuditEvent
}

func (c *accessTest) setup(t *testing.T, specs ...types.RoleSpecV6) (context.Context, testClient) {
	t.Helper()
	clt := newService(t, t.Context())
	ctx := authorizerForDummyUser(t, clt, specs...)
	return ctx, clt
}

func (c *accessTest) run(t *testing.T) {
	t.Run(c.name, func(t *testing.T) {
		t.Run("allowed", func(t *testing.T) {
			spec := types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: c.requiredAccessRules},
			}
			ctx, clt := c.setup(t, spec)
			err := c.actionFn(t, ctx, clt)
			require.NoError(t, err)
			if c.requireEvent != nil {
				got := clt.eventRecorder.LastEvent()
				require.NotNil(t, got)
				require.IsType(t, c.requireEvent, got)
			}
		})
		t.Run("not allowed", func(t *testing.T) {
			spec := types.RoleSpecV6{}
			ctx, clt := c.setup(t, spec)
			err := c.actionFn(t, ctx, clt)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
		t.Run("denied", func(t *testing.T) {
			spec := types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: c.requiredAccessRules},
				Deny:  types.RoleConditions{Rules: c.requiredAccessRules},
			}
			ctx, clt := c.setup(t, spec)
			err := c.actionFn(t, ctx, clt)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	})
}

func newAppAuthConfigJWT() *appauthconfigv1.AppAuthConfig {
	return appauthconfig.NewAppAuthConfigJWT(
		uuid.NewString(),
		[]*labelv1.Label{{Name: "*", Values: []string{"*"}}},
		&appauthconfigv1.AppAuthConfigJWTSpec{
			Issuer:   "https://issuer-url",
			Audience: "teleport",
			KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
				JwksUrl: "https://issuer-url/.well-known/jwks.json",
			},
		},
	)
}

func newService(t *testing.T, ctx context.Context) testClient {
	t.Helper()
	clusterName := "test-cluster"
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	identitySvc, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)
	localAppAuthSvc, err := local.NewAppAuthConfigService(backend)
	require.NoError(t, err)
	accessSvc := local.NewAccessService(backend)
	eventSvc := local.NewEventsService(backend)
	trustSvc := local.NewCAService(backend)
	clt := testClient{
		Access:               accessSvc,
		ClusterConfiguration: clusterConfigSvc,
		AppAuthConfig:        localAppAuthSvc,
		Identity:             identitySvc,
		Trust:                trustSvc,
		eventRecorder:        &eventstest.MockRecorderEmitter{},
	}

	_, err = clt.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clt.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clt.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clt.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventSvc,
			Component: "test",
		},
		LockGetter: accessSvc,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		AccessPoint: clt,
		ClusterName: clusterName,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	appAuthSvc, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Backend:    localAppAuthSvc,
		Cache:      localAppAuthSvc,
		Emitter:    clt.eventRecorder,
	})
	require.NoError(t, err)
	clt.ServiceUnderTest = appAuthSvc
	return clt
}

func authorizerForDummyUser(t *testing.T, clt testClient, roleSpecs ...types.RoleSpecV6) context.Context {
	ctx := t.Context()

	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)

	roleNames := make([]string, 0, len(roleSpecs))
	for _, spec := range roleSpecs {
		role, err := types.NewRole("role-"+uuid.NewString(), spec)
		require.NoError(t, err)
		role, err = clt.CreateRole(ctx, role)
		require.NoError(t, err)
		user.AddRole(role.GetName())
		roleNames = append(roleNames, role.GetName())
	}

	user, err = clt.CreateUser(ctx, user)
	require.NoError(t, err)

	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   roleNames,
		},
	})
}
