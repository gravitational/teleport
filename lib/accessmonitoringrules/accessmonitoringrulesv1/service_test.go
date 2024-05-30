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

package accessmonitoringrulesv1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	accessmonitoringrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAccessMonitoringRuleCRUD(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"

	requireTraceErrorFn := func(traceFn func(error) bool) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...interface{}) {
			require.True(t, traceFn(err), "received an un-expected error: %v", err)
		}
	}

	ctx, localClient, resourceSvc := initSvc(t, clusterName)

	sampleAccessMonitoringRuleFn := func(name string) *accessmonitoringrulev1.AccessMonitoringRule {
		return &accessmonitoringrulev1.AccessMonitoringRule{
			Kind:     types.KindAccessMonitoringRule,
			Version:  types.V1,
			Metadata: &v1.Metadata{Name: name},
			Spec: &accessmonitoringrulev1.AccessMonitoringRuleSpec{
				Subjects:  []string{"someSubject"},
				Condition: "someCondition",
			},
		}
	}

	tt := []struct {
		Name         string
		Role         types.RoleSpecV6
		Setup        func(t *testing.T, amrName string)
		Test         func(ctx context.Context, resourceSvc *Service, amrName string) error
		ErrAssertion require.ErrorAssertionFunc
	}{
		// Read
		{
			Name: "allowed read access to AccessMonitoringRules",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, amrName string) {
				_, err := localClient.CreateAccessMonitoringRule(ctx, sampleAccessMonitoringRuleFn(amrName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.GetAccessMonitoringRule(ctx, &accessmonitoringrulev1.GetAccessMonitoringRuleRequest{
					Name: amrName,
				})
				return err
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no access to read AccessMonitoringRules",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.GetAccessMonitoringRule(ctx, &accessmonitoringrulev1.GetAccessMonitoringRuleRequest{
					Name: amrName,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "denied access to read AccessMonitoringRules",
			Role: types.RoleSpecV6{
				Deny: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.GetAccessMonitoringRule(ctx, &accessmonitoringrulev1.GetAccessMonitoringRuleRequest{
					Name: amrName,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},

		// List
		{
			Name: "allowed list access to AccessMonitoringRules",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbList, types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, _ string) {
				for i := 0; i < 10; i++ {
					_, err := localClient.CreateAccessMonitoringRule(ctx, sampleAccessMonitoringRuleFn(uuid.NewString()))
					require.NoError(t, err)
				}
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.ListAccessMonitoringRules(ctx, &accessmonitoringrulev1.ListAccessMonitoringRulesRequest{
					PageSize:  0,
					PageToken: "",
				})
				return err
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no list access to AccessMonitoringRule",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.ListAccessMonitoringRules(ctx, &accessmonitoringrulev1.ListAccessMonitoringRulesRequest{
					PageSize:  0,
					PageToken: "",
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},

		// Create
		{
			Name: "no access to create AccessMonitoringRules",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.CreateAccessMonitoringRule(ctx, &accessmonitoringrulev1.CreateAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to create AccessMonitoringRules",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.CreateAccessMonitoringRule(ctx, &accessmonitoringrulev1.CreateAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Update
		{
			Name: "no access to update AccessMonitoringRule",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.UpdateAccessMonitoringRule(ctx, &accessmonitoringrulev1.UpdateAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to update AccessMonitoringRule",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbUpdate},
				}}},
			},
			Setup: func(t *testing.T, amrName string) {
				_, err := localClient.CreateAccessMonitoringRule(ctx, sampleAccessMonitoringRuleFn(amrName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.UpdateAccessMonitoringRule(ctx, &accessmonitoringrulev1.UpdateAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Upsert
		{
			Name: "no access to upsert AccessMonitoringRule",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbUpdate}, // missing VerbCreate
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.UpsertAccessMonitoringRule(ctx, &accessmonitoringrulev1.UpsertAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to upsert AccessMonitoringRule",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Setup: func(t *testing.T, amrName string) {},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				amr := sampleAccessMonitoringRuleFn(amrName)
				_, err := resourceSvc.UpsertAccessMonitoringRule(ctx, &accessmonitoringrulev1.UpsertAccessMonitoringRuleRequest{
					Rule: amr,
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Delete
		{
			Name: "no access to delete AccessMonitoringRule",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.DeleteAccessMonitoringRule(ctx, &accessmonitoringrulev1.DeleteAccessMonitoringRuleRequest{Name: "x"})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to delete AccessMonitoringRule",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAccessMonitoringRule},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, amrName string) {
				_, err := localClient.CreateAccessMonitoringRule(ctx, sampleAccessMonitoringRuleFn(amrName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, amrName string) error {
				_, err := resourceSvc.DeleteAccessMonitoringRule(ctx, &accessmonitoringrulev1.DeleteAccessMonitoringRuleRequest{Name: amrName})
				return err
			},
			ErrAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			localCtx := authorizerForDummyUser(t, ctx, tc.Role, localClient)

			amrName := uuid.NewString()
			if tc.Setup != nil {
				tc.Setup(t, amrName)
			}

			err := tc.Test(localCtx, resourceSvc, amrName)
			tc.ErrAssertion(t, err)
		})
	}
}

func authorizerForDummyUser(t *testing.T, ctx context.Context, roleSpec types.RoleSpecV6, localClient localClient) context.Context {
	// Create role
	roleName := "role-" + uuid.NewString()
	role, err := types.NewRole(roleName, roleSpec)
	require.NoError(t, err)

	role, err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	user, err = localClient.CreateUser(ctx, user)
	require.NoError(t, err)

	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})
}

type localClient interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	services.AccessMonitoringRules
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
	services.Presence
}

func initSvc(t *testing.T, clusterName string) (context.Context, localClient, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewIdentityService(backend)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
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
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	localResourceService, err := local.NewAccessMonitoringRulesService(backend)
	require.NoError(t, err)

	resourceSvc, err := NewService(&ServiceConfig{
		Backend:    localResourceService,
		Authorizer: authorizer,
		Cache:      localResourceService,
	})
	require.NoError(t, err)

	return ctx, struct {
		*local.AccessService
		*local.IdentityService
		*local.AccessMonitoringRulesService
	}{
		AccessService:                roleSvc,
		IdentityService:              userSvc,
		AccessMonitoringRulesService: localResourceService,
	}, resourceSvc
}
