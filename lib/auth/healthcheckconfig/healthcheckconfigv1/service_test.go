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

package healthcheckconfigv1

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
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
	services.HealthCheckConfig
	services.Identity
	services.Trust

	ServiceUnderTest *Service
	eventRecorder    *eventstest.MockRecorderEmitter
}

func TestHealthCheckConfigCRUD(t *testing.T) {
	t.Parallel()

	tests := []accessTest{
		// Create
		{
			name: "CreateHealthCheckConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				_, err := clt.ServiceUnderTest.CreateHealthCheckConfig(ctx, &healthcheckconfigv1.CreateHealthCheckConfigRequest{
					Config: newResource(t),
				})
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbCreate)},
			requireEvent:        &apievents.HealthCheckConfigCreate{},
		},
		// Read
		{
			name: "GetHealthCheckConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg1 := newResource(t)
				_, err := clt.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)
				_, err = clt.ServiceUnderTest.GetHealthCheckConfig(ctx, &healthcheckconfigv1.GetHealthCheckConfigRequest{
					Name: cfg1.Metadata.GetName(),
				})
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbRead)},
		},
		{
			name: "ListHealthCheckConfigs",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				_, err := clt.CreateHealthCheckConfig(ctx, newResource(t))
				require.NoError(t, err)
				_, err = clt.CreateHealthCheckConfig(ctx, newResource(t))
				require.NoError(t, err)
				resp, err := clt.ServiceUnderTest.ListHealthCheckConfigs(ctx, &healthcheckconfigv1.ListHealthCheckConfigsRequest{})
				if err == nil {
					require.NotNil(t, resp)
					require.Len(t, resp.Configs, 2+teleport.VirtualDefaultHealthCheckConfigCount,
						"expected 2 inserted and virtual defaults")
				}
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbRead, types.VerbList)},
		},
		// Update
		{
			name: "UpdateHealthCheckConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg, err := clt.CreateHealthCheckConfig(ctx, newResource(t))
				require.NoError(t, err)
				cfg.Spec.HealthyThreshold = 3
				cfg, err = clt.ServiceUnderTest.UpdateHealthCheckConfig(ctx, &healthcheckconfigv1.UpdateHealthCheckConfigRequest{
					Config: cfg,
				})
				if err == nil {
					require.NotNil(t, cfg, "the updated resource should be returned")
					require.Equal(t, 3, int(cfg.Spec.HealthyThreshold), "the resource should have been updated")
				}
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbUpdate)},
			requireEvent:        &apievents.HealthCheckConfigUpdate{},
		},
		{
			name: "UpsertHealthCheckConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg := newResource(t)
				_, err := clt.ServiceUnderTest.UpsertHealthCheckConfig(ctx, &healthcheckconfigv1.UpsertHealthCheckConfigRequest{
					Config: cfg,
				})
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbCreate, types.VerbUpdate)},
			requireEvent:        &apievents.HealthCheckConfigCreate{},
		},
		// Delete
		{
			name: "DeleteHealthCheckConfig",
			actionFn: func(t *testing.T, ctx context.Context, clt testClient) error {
				cfg, err := clt.CreateHealthCheckConfig(ctx, newResource(t))
				require.NoError(t, err)
				_, err = clt.ServiceUnderTest.DeleteHealthCheckConfig(ctx, &healthcheckconfigv1.DeleteHealthCheckConfigRequest{
					Name: cfg.Metadata.GetName(),
				})
				return err
			},
			requiredAccessRules: []types.Rule{healthCheckConfigRule(types.VerbDelete)},
			requireEvent:        &apievents.HealthCheckConfigDelete{},
		},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func healthCheckConfigRule(verbs ...string) types.Rule {
	return types.Rule{
		Resources: []string{types.KindHealthCheckConfig},
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
	clt := newService(t, context.Background())
	ctx := authorizerForDummyUser(t, clt, specs...)
	return ctx, clt
}

func (c *accessTest) run(t *testing.T) {
	t.Run(fmt.Sprintf("%s is allowed", c.name), func(t *testing.T) {
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

	t.Run(fmt.Sprintf("%s is not allowed", c.name), func(t *testing.T) {
		spec := types.RoleSpecV6{}
		ctx, clt := c.setup(t, spec)
		err := c.actionFn(t, ctx, clt)
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run(fmt.Sprintf("%s is denied", c.name), func(t *testing.T) {
		spec := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: c.requiredAccessRules},
			Deny:  types.RoleConditions{Rules: c.requiredAccessRules},
		}
		ctx, clt := c.setup(t, spec)
		err := c.actionFn(t, ctx, clt)
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

func newResource(t *testing.T) *healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	r, err := healthcheckconfig.NewHealthCheckConfig(uuid.NewString(),
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabels: []*labelv1.Label{{
					Name:   "*",
					Values: []string{"*"},
				}},
			},
		},
	)
	require.NoError(t, err)
	return r
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
	localHealthSvc, err := local.NewHealthCheckConfigService(backend)
	require.NoError(t, err)
	accessSvc := local.NewAccessService(backend)
	eventSvc := local.NewEventsService(backend)
	trustSvc := local.NewCAService(backend)
	clt := testClient{
		Access:               accessSvc,
		ClusterConfiguration: clusterConfigSvc,
		HealthCheckConfig:    localHealthSvc,
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

	healthSvc, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Backend:    localHealthSvc,
		Cache:      localHealthSvc,
		Emitter:    clt.eventRecorder,
	})
	require.NoError(t, err)
	clt.ServiceUnderTest = healthSvc
	return clt
}

func authorizerForDummyUser(t *testing.T, clt testClient, roleSpecs ...types.RoleSpecV6) context.Context {
	ctx := context.Background()

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
